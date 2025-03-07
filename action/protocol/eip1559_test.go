// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package protocol

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
)

func TestCalcBaseFee(t *testing.T) {
	r := require.New(t)
	tests := []struct {
		parentBaseFee   int64
		parentGasUsed   uint64
		expectedBaseFee int64
	}{
		{action.InitialBaseFee, 25000000, action.InitialBaseFee}, // usage == target
		{2 * action.InitialBaseFee, 24000000, 1990000000000},     // usage below target
		{action.InitialBaseFee, 24000000, 1000000000000},         // usage below target but capped to default
		{action.InitialBaseFee, 26000000, 1005000000000},         // usage above target
	}
	g := genesis.TestDefault().Blockchain
	for _, test := range tests {
		parent := &TipInfo{
			Height:  g.VanuatuBlockHeight,
			GasUsed: test.parentGasUsed,
			BaseFee: big.NewInt(test.parentBaseFee),
		}
		expect := big.NewInt(test.expectedBaseFee)
		r.Equal(expect, CalcBaseFee(g, parent))
	}
}
