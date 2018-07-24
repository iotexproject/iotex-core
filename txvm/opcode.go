// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package txvm

import (
	"bytes"
	"fmt"

	"golang.org/x/crypto/blake2b"

	cp "github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/pkg/keypair"
)

// ConstructFunc takes an array of bytecodes to build an opnode. Note that:
// 1. The bytecodes must start with a valid opcode byte.
// 2. The bytecodes must be longer than required.
// 3. If it is insufficient to constructing an OpNode, e.g., data length not met, OpEndIf not found, error is thrown.
// 4. If an OpNode is successfully created, an int offset tells how many bytes it consumes.
type ConstructFunc func([]byte) (*OpNode, int, error)

// RunFunc declares a function type
type RunFunc func(*OpNode, *IVM) error

// BuildOpNodeFromBytes builds operation node from bytes. It is a specific implementation of ConstructFunc
func BuildOpNodeFromBytes(bytecodes []byte) (*OpNode, int, error) {
	if len(bytecodes) == 0 {
		return nil, 0, scriptError(ErrInvalidOpcode, "Empty bytecodes")
	}
	opcode := bytecodes[0]
	opname := opinfoArray[opcode].name
	if opname == "" {
		return nil, 0, scriptError(ErrInvalidOpcode, "Opcode not found")
	}

	constructor := opinfoArray[opcode].constructor
	return constructor(bytecodes)
}

// RunOpNode runs operation node. It is a specific implementation of RunFunc
func RunOpNode(node *OpNode, vm *IVM) error {
	runfunc := opinfoArray[node.opcode].runfunc
	opname := opinfoArray[node.opcode].name
	if opname == "" {
		return scriptError(ErrInvalidOpcode, "Opcode not found")
	}
	return runfunc(node, vm)
}

// Enumerate of opcodes
const (
	Op0 = iota
	OpData1
	OpData2
	OpData3
	OpData20 = 0x14
	OpData32 = 0x20
	OpData64 = 0x40
	OpData72 = 0x48
)

// Enumerate of opcodes
const (
	OpIf = iota + 0x61
	OpElse
	OpEndIf
	OpNope
	OpDup
)

// Verify
const (
	OpVerify = iota + 0xa0
	OpEqualVerify
	OpScriptHashVerifyPop
)

// Hash
const (
	OpHash160 = iota + 0xb0
	OpCheckSig
)

// External Call
const (
	OpCheckLockTime = iota + 0xc0
	OpOtherSPVVerify
)

// Enumerate unused opcodes
const (
	OpUnused = iota + 0xd0
)

type opinfo struct {
	name        string
	constructor ConstructFunc
	runfunc     RunFunc
}

// ************************************
// Mapping from opcode to its ConstructFunc
// ************************************
var opinfoArray [256]opinfo

func opConstructDefault(bytecodes []byte) (*OpNode, int, error) {
	opcode := bytecodes[0]
	node := OpNode{}
	node.opcode = opcode
	return &node, 1, nil
}

func opConstructData(bytecodes []byte) (*OpNode, int, error) {
	datalen := int(bytecodes[0])
	opcode := bytecodes[0]
	if len(bytecodes) <= datalen {
		return nil, 0, scriptError(ErrInvalidOpcode,
			fmt.Sprintf("bytecodes not long enough."+
				"Expect > %d, get %d", datalen, len(bytecodes)))
	}
	node := OpNode{}
	node.opcode = opcode
	node.data = bytecodes[1 : datalen+1]
	return &node, datalen + 1, nil
}

func opConstructBranch(bytecodes []byte) (*OpNode, int, error) {
	node := OpNode{opcode: bytecodes[0]}
	offset := 1
	branchIdx := 0 // IF part is indexed 0, ELSE part(if any) is indexed 1
	node.asts = append(node.asts, IAST{})

	for offset < len(bytecodes) && bytecodes[offset] != OpEndIf {
		if bytecodes[offset] == OpElse {
			branchIdx = 1
			node.asts = append(node.asts, IAST{})
			offset++
			continue
		}
		newNode, newOffset, err := BuildOpNodeFromBytes(bytecodes[offset:])
		if err != nil {
			return nil, 0, err
		}
		node.asts[branchIdx].nodes = append(node.asts[branchIdx].nodes, *newNode)
		offset += newOffset
	}

	if offset >= len(bytecodes) {
		return nil, 0, scriptError(ErrInvalidOpcode, "Unbalanced conditional branch. No OpEndIf found")
	}
	return &node, offset + 1, nil

}

func opConstructError(bytecodes []byte) (*OpNode, int, error) {
	return nil, 0, scriptError(ErrInvalidOpcode,
		fmt.Sprintf("This opcode %x is not supposed to construct an OpNode", bytecodes[0]))
}

// ************************************
// Mapping from opcode to its RunFunc
// ************************************
func opcodePushTrue(node *OpNode, vm *IVM) error {
	vm.dstack = append(vm.dstack, []byte{0x01})
	return nil
}

func opcodePushFalse(node *OpNode, vm *IVM) error {
	vm.dstack = append(vm.dstack, []byte{0x00})
	return nil
}

func opcodePushData(node *OpNode, vm *IVM) error {
	vm.dstack = append(vm.dstack, node.data)
	return nil
}

func opcodeDup(node *OpNode, vm *IVM) error {
	s := vm.dstack
	if len(s) == 0 {
		return scriptError(ErrInvalidStackOperation, "empty stack, cannot dup")
	}

	vm.dstack = append(s, s[len(s)-1]) // pop
	return nil
}

func opcodeHash160(node *OpNode, vm *IVM) error {
	s := vm.dstack
	if len(s) == 0 {
		return scriptError(ErrInvalidStackOperation, "empty stack, cannot hash160")
	}

	// The public key is assumed to be 32 bytes.
	m := s[len(s)-1]
	news := s[:len(s)-1] // pop
	hash := blake2b.Sum256(m)
	_ = append(news, hash[7:27])
	return nil
}

func opcodeEqual(node *OpNode, vm *IVM) error {
	if len(vm.dstack) < 2 {
		return scriptError(ErrInvalidStackOperation, "stack has too few entries, cannot Equal")
	}

	a := vm.dstack[len(vm.dstack)-1]
	b := vm.dstack[len(vm.dstack)-2]
	vm.dstack = vm.dstack[:len(vm.dstack)-2] // pop
	if bytes.Equal(a, b) {
		return opcodePushTrue(node, vm)
	}
	return opcodePushFalse(node, vm)
}

func opcodeEqualVerify(node *OpNode, vm *IVM) error {
	err := opcodeEqual(node, vm)
	if err != nil {
		return err
	}

	verified := vm.dstack[len(vm.dstack)-1]
	vm.dstack = vm.dstack[:len(vm.dstack)-1] // pop
	if !bytes.Equal(verified, []byte{0x01}) {
		return scriptError(ErrEqualVerify, "invalid signature")
	}
	return nil
}

func opcodeCheckSig(node *OpNode, vm *IVM) error {
	if len(vm.dstack) < 2 {
		return scriptError(ErrInvalidStackOperation, "stack has too few entries, cannot Equal")
	}

	pubkey := vm.dstack[len(vm.dstack)-1]
	sig := vm.dstack[len(vm.dstack)-2]
	vm.dstack = vm.dstack[:len(vm.dstack)-2] // pop

	hash := blake2b.Sum256(vm.txin)
	publicKey, err := keypair.BytesToPublicKey(pubkey)
	if err != nil {
		return err
	}
	if cp.Verify(publicKey, hash[:], sig) {
		return opcodePushTrue(node, vm)
	}
	return opcodePushFalse(node, vm)
}

func opcodeRunBranch(node *OpNode, vm *IVM) error {
	return scriptError(ErrUnsupportedOpcode, "Unimplemented")
}

func opcodeRunNothing(node *OpNode, vm *IVM) error {
	return nil
}

func opcodeRunError(node *OpNode, vm *IVM) error {
	return scriptError(ErrInvalidOpcode,
		fmt.Sprintf("This opcode %x is not supposed to run", node.opcode))
}

func init() {
	opinfoArray[Op0] = opinfo{"Op0", opConstructDefault, opcodePushFalse}
	opinfoArray[OpData1] = opinfo{"OpData1", opConstructData, opcodePushData}
	opinfoArray[OpData2] = opinfo{"OpData2", opConstructData, opcodePushData}
	opinfoArray[OpData3] = opinfo{"OpData2", opConstructData, opcodePushData}
	opinfoArray[OpData20] = opinfo{"OpData20", opConstructData, opcodePushData}
	opinfoArray[OpData32] = opinfo{"OpData32", opConstructData, opcodePushData}
	opinfoArray[OpData64] = opinfo{"OpData64", opConstructData, opcodePushData}
	opinfoArray[OpData72] = opinfo{"OpData72", opConstructData, opcodePushData}

	opinfoArray[OpIf] = opinfo{"OpIf", opConstructBranch, opcodeRunBranch}
	opinfoArray[OpElse] = opinfo{"OpElse", opConstructError, opcodeRunError}
	opinfoArray[OpNope] = opinfo{"OpNope", opConstructDefault, opcodeRunNothing}

	opinfoArray[OpDup] = opinfo{"OpDup", opConstructDefault, opcodeDup}
	opinfoArray[OpHash160] = opinfo{"OpHash160", opConstructDefault, opcodeHash160}
	opinfoArray[OpEqualVerify] = opinfo{"OpEqualVerify", opConstructDefault, opcodeEqualVerify}
	opinfoArray[OpCheckSig] = opinfo{"OpCheckSig", opConstructDefault, opcodeCheckSig}
}
