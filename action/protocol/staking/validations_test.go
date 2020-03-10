// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"math/big"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/pkg/unit"
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

func TestProtocol_ValidateCreateStake(t *testing.T) {
	require := require.New(t)

	p := NewProtocol(nil, nil, genesis.Staking{
		MinStakeAmount: 100,
	})
	candidate := testCandidates[0].d.Clone()
	require.NoError(p.inMemCandidates.Upsert(candidate))
	candidateName := candidate.Name

	tests := []struct {
		// action fields
		candName  string
		amount    string
		duration  uint32
		autoStake bool
		gasPrice  *big.Int
		gasLimit  uint64
		nonce     uint64
		// expected results
		errorCause error
	}{
		{
			"",
			"100000000000000000000",
			1,
			false,
			big.NewInt(unit.Qev),
			10000,
			1,
			ErrInvalidCanName,
		},
		{
			"$$$",
			"100000000000000000000",
			1,
			false,
			big.NewInt(unit.Qev),
			10000,
			1,
			ErrInvalidCanName,
		},
		{
			"123",
			"200000000000000000000",
			1,
			false,
			big.NewInt(unit.Qev),
			10000,
			1,
			ErrInvalidCanName,
		},
		{
			candidateName,
			"1000000000000000000",
			1,
			false,
			big.NewInt(unit.Qev),
			10000,
			1,
			ErrInvalidAmount,
		},
		{
			candidateName,
			"200000000000000000000",
			1,
			false,
			big.NewInt(-unit.Qev),
			10000,
			1,
			action.ErrGasPrice,
		},
		{
			candidateName,
			"200000000000000000000",
			1,
			false,
			big.NewInt(unit.Qev),
			10000,
			1,
			nil,
		},
	}

	for _, test := range tests {
		act, err := action.NewCreateStake(test.nonce, test.candName, test.amount, test.duration, test.autoStake,
			nil, test.gasLimit, test.gasPrice)
		require.NoError(err)
		require.Equal(test.errorCause, errors.Cause(p.validateCreateStake(context.Background(), act)))
	}
}
