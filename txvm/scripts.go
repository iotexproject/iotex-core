// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package txvm

import (
	"bytes"
	"fmt"

	"golang.org/x/crypto/blake2b"

	cp "github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/iotxaddress"
)

// printScript formats a disassembled script for one line printing.
func printScript(bytecodes []byte) (string, error) {
	var disbuf bytes.Buffer
	for _, bytecode := range bytecodes {
		// TODO: handle opcode that is unimplemented
		disbuf.WriteString(opinfoArray[bytecode].name)
		disbuf.WriteByte(' ')
	}

	if disbuf.Len() > 0 {
		disbuf.Truncate(disbuf.Len() - 1)
	}
	return disbuf.String(), nil
}

// PayToAddrScript creates a new script to pay a transaction output to a given address.
func PayToAddrScript(addr string) ([]byte, error) {
	b := NewScriptBuilder()
	if err := b.AddOps([]byte{OpDup, OpHash160}); err != nil {
		return nil, err
	}
	err := b.AddOp(OpData20)
	if err != nil {
		return nil, fmt.Errorf("cannot add data: %v", err)
	}
	if err := b.AddData(iotxaddress.GetPubkeyHash(addr)); err != nil {
		return nil, err
	}
	if err := b.AddOps([]byte{OpEqualVerify, OpCheckSig}); err != nil {
		return nil, err
	}
	return b.Bytecodes(), nil
}

// SignatureScript creates an input signature script for a transaction.
func SignatureScript(txin []byte, pubkey []byte, privkey []byte) ([]byte, error) {
	b := NewScriptBuilder()
	err := b.AddOp(OpData64)
	if err != nil {
		return nil, fmt.Errorf("cannot add data: %v", err)
	}
	hash := blake2b.Sum256(txin)
	sig := cp.Sign(privkey, hash[:])
	err = b.AddData(sig)
	if err != nil {
		return nil, fmt.Errorf("cannot add data: %v", err)
	}

	err = b.AddOp(OpData32)
	if err != nil {
		return nil, fmt.Errorf("cannot add data: %v", err)
	}
	err = b.AddData(pubkey)
	if err != nil {
		return nil, fmt.Errorf("cannot add data: %v", err)
	}

	return b.Bytecodes(), nil
}
