// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsValidCandidateName(t *testing.T) {
	tests := []struct {
		input  string
		output bool
	}{
		{
			input:  "abc",
			output: true,
		},
		{
			input:  "123",
			output: true,
		},
		{
			input:  "abc123abc123",
			output: true,
		},
		{
			input:  "Abc123",
			output: false,
		},
		{
			input:  "Abc 123",
			output: false,
		},
		{
			input:  "Abc-123",
			output: false,
		},
		{
			input:  "abc123abc123abc123",
			output: false,
		},
		{
			input:  "",
			output: false,
		},
	}

	for _, tt := range tests {
		output := IsValidCandidateName(tt.input)
		assert.Equal(t, tt.output, output)
	}
}
