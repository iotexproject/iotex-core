// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package txvm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewIVM(t *testing.T) {
	t.Parallel()

	builder := NewScriptBuilder()
	assert.Nil(t, builder.AddOp(Op0))
	assert.Nil(t, builder.AddOp(OpData3))
	assert.Nil(t, builder.AddData([]byte{0x12, 0x34, 0x56}))
	assert.Nil(t, builder.AddOp(OpNope))

	bytecodes := builder.Bytecodes()
	assert.NotNil(t, bytecodes)

	vm, err := NewIVM([]byte{}, bytecodes)
	assert.Nil(t, err)
	assert.Equal(t, 3, len(vm.ast.nodes))

	err = vm.Execute()
	assert.Nil(t, err)
}
