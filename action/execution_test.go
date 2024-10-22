// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

func TestExecutionSignVerify(t *testing.T) {
	require := require.New(t)
	contractAddr := identityset.Address(28)
	executorKey := identityset.PrivateKey(27)
	data, err := hex.DecodeString("")
	require.NoError(err)
	ex := NewExecution(contractAddr.String(), big.NewInt(10), data)
	require.EqualValues(66, ex.Size())

	bd := &EnvelopeBuilder{}
	eb := bd.SetNonce(2).SetGasLimit(100000).SetGasPrice(big.NewInt(10)).
		SetAction(ex).Build()
	elp, ok := eb.(*envelope)
	require.True(ok)
	require.EqualValues(87, eb.Size())

	w := AssembleSealedEnvelope(elp, executorKey.PublicKey(), []byte("lol"))
	require.Error(w.VerifySignature())
	ex2, ok := w.Envelope.Action().(*Execution)
	require.True(ok)
	require.Equal(ex, ex2)

	// sign the Execution
	selp, err := Sign(elp, executorKey)
	require.NoError(err)
	require.NotNil(selp)
	// verify signature
	require.NoError(selp.VerifySignature())
}

func TestExecutionSanityCheck(t *testing.T) {
	require := require.New(t)
	t.Run("Negative amount", func(t *testing.T) {
		ex := NewExecution("2", big.NewInt(-100), []byte{})
		require.Equal(ErrInvalidAmount, errors.Cause(ex.SanityCheck()))
	})

	t.Run("Invalid contract address", func(t *testing.T) {
		ex := NewExecution(
			identityset.Address(29).String()+"bbb",
			big.NewInt(0),
			[]byte{},
		)
		require.Contains(ex.SanityCheck().Error(), "error when validating contract's address")
	})

	t.Run("Empty contract address", func(t *testing.T) {
		ex := NewExecution(
			EmptyAddress,
			big.NewInt(0),
			[]byte{},
		)
		require.Nil(ex.To())
		addr, _ := address.FromBytes(_c1.Bytes())
		ex.contract = addr.String()
		require.Equal(_c1, *ex.To())
	})

	t.Run("Negative gas price", func(t *testing.T) {
		ex := NewExecution(identityset.Address(29).String(), big.NewInt(100), []byte{})
		elp := (&EnvelopeBuilder{}).SetGasPrice(big.NewInt(-1)).SetAction(ex).Build()
		require.Equal(ErrNegativeValue, errors.Cause(elp.SanityCheck()))
	})
}

var (
	_c1 = common.HexToAddress("01fc246633470cf62ae2a956d21e8d481c3a69e1")
	_c2 = common.HexToAddress("3470cf62ae2a956d38d481c3a69e121e01fc2466")
	_k1 = common.HexToHash("02e940dd0fd5b5df4cfb8d6bcd9c74ec433e9a5c21acb72cbcb5be9e711b678f")
	_k2 = common.HexToHash("e7709aa7aa161246674919b2f0299e95cbb6c5482e5c348d12dfe226f71f63d6")
	_k3 = common.HexToHash("a618ea5b489eca42f331abcb08394f581f2e9da89c8ee7e72c747204842abe8b")
	_k4 = common.HexToHash("881d3bdf2e13b6e8b6d685d2277a48ff37141495ddd4e3d7289fcfa5570f29f1")
)

func TestExecutionAccessList(t *testing.T) {
	require := require.New(t)

	ex1 := &Execution{}
	for _, v := range []struct {
		list types.AccessList
		gas  uint64
		cost string
	}{
		{nil, 10400, "100000020"},
		{
			types.AccessList{
				{Address: common.Address{}, StorageKeys: nil},
			}, 12800, "2500000020",
		},
		{
			types.AccessList{
				{Address: _c2, StorageKeys: []common.Hash{{}, _k1}},
			}, 16600, "6300000020",
		},
		{
			types.AccessList{
				{Address: common.Address{}, StorageKeys: nil},
				{Address: _c1, StorageKeys: []common.Hash{_k1, {}, _k3}},
				{Address: _c2, StorageKeys: []common.Hash{_k2, _k3, _k4, _k1}},
			}, 30900, "20600000020",
		},
	} {
		ex := NewExecution(
			identityset.Address(29).String(),
			big.NewInt(20),
			[]byte("test"))
		elp := (&EnvelopeBuilder{}).SetTxType(AccessListTxType).SetNonce(1).SetAccessList(v.list).
			SetGasPrice(big.NewInt(1000000)).SetGasLimit(100).SetAction(ex).Build()
		require.NoError(ex1.LoadProto(ex.Proto()))
		require.Equal(ex, ex1)
		gas, err := elp.IntrinsicGas()
		require.NoError(err)
		require.Equal(v.gas, gas)
		cost, err := elp.Cost()
		require.NoError(err)
		require.Equal(v.cost, cost.String())
	}
}
