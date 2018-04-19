// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package txvm

// OpNode defines the struct of operation node
type OpNode struct {
	opcode byte
	data   []byte
	asts   []IAST
}

// IAST is a Abstract Syntax Tree.
type IAST struct {
	nodes []OpNode
}

// ParseRaw builds an AST from byte codes.
func ParseRaw(bytecodes []byte) (*IAST, error) {
	ast := IAST{}
	start := 0
	for start < len(bytecodes) {
		newNode, offset, err := BuildOpNodeFromBytes(bytecodes[start:])
		if err != nil {
			return nil, err
		}

		ast.nodes = append(ast.nodes, *newNode)
		start += offset
	}
	return &ast, nil
}
