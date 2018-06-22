// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package txvm

const (
	// maxScriptSize is the maximum allowed length of a raw script.
	maxScriptSize = 10000
)

// ScriptBuilder builds a script using opcodes
type ScriptBuilder interface {
	// AddOp adds one opcode
	AddOp(op byte) error

	// AddOps adds several opcodes
	AddOps(op []byte) error

	// AddData adds data
	AddData(data []byte) error

	// Bytecodes returns byte codes built
	Bytecodes() []byte

	// Reset resets all internal states of the builder.
	Reset() error
}

// scriptBuilder implements ScriptBuilder infercace
type scriptBuilder struct {
	bytecodes []byte
}

// NewScriptBuilder creates an ScriptBuilder
func NewScriptBuilder() ScriptBuilder {
	return &scriptBuilder{}
}

// AddOp adds an opcode
func (b *scriptBuilder) AddOp(opcode byte) error {
	if len(b.bytecodes)+1 > maxScriptSize {
		return scriptError(ErrInvalidOpcode, "opcode cannot be added as script size reaches the maximum")
	}
	b.bytecodes = append(b.bytecodes, opcode)
	return nil
}

// AddOps adds several opcodes
func (b *scriptBuilder) AddOps(opcodes []byte) error {
	if len(b.bytecodes)+len(opcodes) > maxScriptSize {
		return scriptError(ErrInvalidOpcode, "opcode cannot be added as script size reaches the maximum")
	}
	b.bytecodes = append(b.bytecodes, opcodes...)
	return nil
}

// AddData adds data
func (b *scriptBuilder) AddData(data []byte) error {
	if len(data) == 0 {
		return scriptError(ErrInvalidOpdata, "invalid data for scriptBuilder")
	}
	// TODO: check agasint maxScriptSize
	b.bytecodes = append(b.bytecodes, data...)
	return nil
}

// Bytecodes returns the byte codes the builder has so far
func (b *scriptBuilder) Bytecodes() []byte {
	return b.bytecodes
}

// Reset resets all internal states of the builder.
func (b *scriptBuilder) Reset() error {
	b.bytecodes = b.bytecodes[0:0]
	return nil
}
