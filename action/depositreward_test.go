// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
)

var (
	_defaultGasPrice      = big.NewInt(1000000000000)
	_errNegativeNumberMsg = "negative value"
)

func TestDepositRewardSerialize(t *testing.T) {
	r := require.New(t)
	t.Run("proto", func(t *testing.T) {
		rp := NewDepositToRewardingFund(big.NewInt(100), []byte{1})
		data := rp.Serialize()
		r.EqualValues("0a03313030120101", hex.EncodeToString(data))

		rp2 := DepositToRewardingFund{}
		r.NoError(rp2.LoadProto(rp.Proto()))
		r.Equal(rp.Amount(), rp2.Amount())
		r.Equal(rp.Data(), rp2.Data())
	})
	t.Run("intrinsic gas", func(t *testing.T) {
		rp := &DepositToRewardingFund{}
		gas, err := rp.IntrinsicGas()
		r.NoError(err)
		r.EqualValues(10000, gas)

		rp.amount = big.NewInt(100000000)
		gas, err = rp.IntrinsicGas()
		r.NoError(err)
		r.EqualValues(10000, gas)

		rp.data = []byte{1}
		gas, err = rp.IntrinsicGas()
		r.NoError(err)
		r.EqualValues(10100, gas)
	})
	t.Run("sanity check", func(t *testing.T) {
		rp := &DepositToRewardingFund{amount: big.NewInt(1)}
		err := rp.SanityCheck()
		r.NoError(err)

		rp.amount = big.NewInt(-1)
		err = rp.SanityCheck()
		r.NotNil(err)
		r.EqualValues(_errNegativeNumberMsg, err.Error())
	})
	t.Run("cost", func(t *testing.T) {
		rp := &DepositToRewardingFund{amount: big.NewInt(100)}
		elp := (&EnvelopeBuilder{}).SetGasPrice(_defaultGasPrice).
			SetAction(rp).Build()
		cost, err := elp.Cost()
		r.NoError(err)
		r.EqualValues("10000000000000100", cost.String())

		rp.data = []byte{1}
		cost, err = elp.Cost()
		r.NoError(err)
		r.EqualValues("10100000000000100", cost.String())
	})
}

func TestDepositRewardEncodeABIBinary(t *testing.T) {
	r := require.New(t)

	rp := &DepositToRewardingFund{}

	rp.amount = big.NewInt(101)
	data, err := rp.EthData()
	r.NoError(err)
	r.EqualValues(
		"27852a6b000000000000000000000000000000000000000000000000000000000000006500000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000000",
		hex.EncodeToString(data),
	)

	rp.data = []byte{1, 2, 3}
	data, err = rp.EthData()
	r.NoError(err)
	r.EqualValues(
		"27852a6b000000000000000000000000000000000000000000000000000000000000006500000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000003000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000003",
		hex.EncodeToString(data),
	)
}

func TestNewRewardingDepositFromABIBinary(t *testing.T) {
	r := require.New(t)

	data, _ := hex.DecodeString("27852a6b000000000000000000000000000000000000000000000000000000000000006500000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000003000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000003")

	rp, err := NewDepositToRewardingFundFromABIBinary(data)
	r.NoError(err)
	r.IsType(&DepositToRewardingFund{}, rp)
	r.EqualValues("101", rp.Amount().String())
	r.EqualValues([]byte{1, 2, 3}, rp.Data())
}
