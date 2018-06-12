// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package txvm

// IVM defines the struct of IoTeX Virtual Machine
type IVM struct {
	ast    *IAST
	dstack [][]byte
	txin   []byte
}

// Execute executes IoTeX Virtual Machine
func (vm *IVM) Execute() (err error) {
	// TODO: evaluate AST recursively
	for _, node := range vm.ast.nodes {
		if err := opinfoArray[node.opcode].runfunc(&node, vm); err != nil {
			return err
		}
	}
	return nil
}

// NewIVM creates a new IoTeX Virtual Machine
func NewIVM(txin, bytecodes []byte) (*IVM, error) {
	ast, err := ParseRaw(bytecodes)
	if err != nil {
		return nil, err
	}
	vm := IVM{ast: ast, txin: txin}
	return &vm, nil
}
