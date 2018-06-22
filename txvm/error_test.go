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

func TestScriptError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   ScriptError
		want string
	}{
		{scriptError(ErrUnsupportedOpcode, "It is a mistake"), "It is a mistake"},
		{scriptError(ErrUnsupportedOpcode, "readable"), "readable"},
	}

	t.Logf("Running %d tests for scriptError()", len(tests))
	for _, test := range tests {
		assert.Equal(t, test.want, test.in.Error())
	}
}
