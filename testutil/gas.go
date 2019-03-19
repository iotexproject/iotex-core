// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package testutil

import (
	"math/big"

	"github.com/iotexproject/iotex-core/pkg/unit"
)

// TestGasLimit represents the gas limit used for test actions
const TestGasLimit uint64 = 20000

// TestGasPriceInt64 represents the gas price for test actions in int64
const TestGasPriceInt64 = 0

// TestGasPrice represents the gas price for test actions in big int
var TestGasPrice = big.NewInt(unit.Qev)
