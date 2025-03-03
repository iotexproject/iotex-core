// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

type (
	// TxDataForSimulation is the interface to run a simulation
	TxDataForSimulation interface {
		TxData
		SanityCheck() error
		Proto() *iotextypes.ActionCore
	}
)

// EffectiveGasTip returns the effective gas
func EffectiveGasTip(tx TxDynamicGas, baseFee *big.Int) (*big.Int, error) {
	tip := tx.GasTipCap()
	if baseFee == nil {
		return tip, nil
	}
	effectiveGas := tx.GasFeeCap()
	effectiveGas.Sub(effectiveGas, baseFee)
	if effectiveGas.Sign() < 0 {
		return effectiveGas, ErrGasFeeCapTooLow
	}
	// effective gas = min(tip, feeCap - baseFee)
	if effectiveGas.Cmp(tip) <= 0 {
		return effectiveGas, nil
	}
	return tip, nil
}
