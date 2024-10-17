// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package protocol

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
)

func VerifyEIP1559Header(g genesis.Blockchain, parent *TipInfo, header blockHeader) error {
	if header.BaseFee() == nil {
		return errors.New("header is missing baseFee")
	}
	// Verify the baseFee is correct based on the parent header.
	expectedBaseFee := CalcBaseFee(g, parent)
	if header.BaseFee().Cmp(expectedBaseFee) != 0 {
		return errors.Errorf("invalid baseFee: have %s, want %s, parentBaseFee %s, parentGasUsed %d",
			header.BaseFee(), expectedBaseFee, parent.BaseFee, parent.GasUsed)
	}
	return nil
}

// CalcBaseFee calculates the basefee of the header.
func CalcBaseFee(g genesis.Blockchain, parent *TipInfo) *big.Int {
	if parent.Height == g.VanuatuBlockHeight-1 {
		// If the current block is the first EIP-1559 block, return the InitialBaseFee.
		return new(big.Int).SetUint64(action.InitialBaseFee)
	} else if parent.Height < g.VanuatuBlockHeight-1 {
		// return nil for no base fee block
		return nil
	}

	parentGasTarget := g.BlockGasLimitByHeight(parent.Height) / action.DefaultElasticityMultiplier
	// If the parent gasUsed is the same as the target, the baseFee remains unchanged.
	if parent.GasUsed == parentGasTarget {
		return new(big.Int).Set(parent.BaseFee)
	}

	var (
		num   = new(big.Int)
		denom = new(big.Int)
	)
	if parent.GasUsed > parentGasTarget {
		// If the parent block used more gas than its target, the baseFee should increase.
		// max(1, parentBaseFee * gasUsedDelta / parentGasTarget / baseFeeChangeDenominator)
		num.SetUint64(parent.GasUsed - parentGasTarget)
		num.Mul(num, parent.BaseFee)
		num.Div(num, denom.SetUint64(parentGasTarget))
		num.Div(num, denom.SetUint64(action.DefaultBaseFeeChangeDenominator))
		baseFeeDelta := math.BigMax(num, common.Big1)
		return num.Add(parent.BaseFee, baseFeeDelta)
	} else {
		// Otherwise if the parent block used less gas than its target, the baseFee should decrease.
		// max(0, parentBaseFee * gasUsedDelta / parentGasTarget / baseFeeChangeDenominator)
		num.SetUint64(parentGasTarget - parent.GasUsed)
		num.Mul(num, parent.BaseFee)
		num.Div(num, denom.SetUint64(parentGasTarget))
		num.Div(num, denom.SetUint64(action.DefaultBaseFeeChangeDenominator))
		baseFee := num.Sub(parent.BaseFee, num)
		return math.BigMax(baseFee, new(big.Int).SetUint64(action.InitialBaseFee))
	}
}
