// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package txvm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseRaw(t *testing.T) {
	t.Parallel()

	// Test 1: a noop script
	builder := NewScriptBuilder()
	assert.Nil(t, builder.AddOp(Op0))
	assert.Nil(t, builder.AddOp(OpData2))
	assert.Nil(t, builder.AddData([]byte{0x12, 0x34}))
	assert.Nil(t, builder.AddOp(OpNope))

	assert.NotNil(t, builder.Bytecodes())
	ast, err := ParseRaw(builder.Bytecodes())
	if err != nil {
		t.Errorf("Get an error: %s", err.Error())
	}
	if len(ast.nodes) != 3 || ast.nodes[0].opcode != Op0 || len(ast.nodes[1].data) != 2 || ast.nodes[2].opcode != OpNope {
		t.Errorf("Parsed OpNode is wrong!")
	}

	// Test 2: Pay2AddrHash script

	// Test 3: script with branch

	// Test 4: script with errors

}
