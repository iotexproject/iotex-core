// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/test/identityset"
)

func TestExecutionSignVerify(t *testing.T) {
	require := require.New(t)
	contractAddr := identityset.Address(28)
	executorKey := identityset.PrivateKey(27)
	data, err := hex.DecodeString("")
	require.NoError(err)
	ex, err := NewExecution(contractAddr.String(), 2, big.NewInt(10), uint64(100000), big.NewInt(10), data)
	require.NoError(err)
	require.EqualValues(21, ex.BasicActionSize())
	require.EqualValues(87, ex.TotalSize())

	bd := &EnvelopeBuilder{}
	eb := bd.SetNonce(ex.nonce).
		SetGasLimit(ex.gasLimit).
		SetGasPrice(ex.gasPrice).
		SetAction(ex).Build()
	elp, ok := eb.(*envelope)
	require.True(ok)

	w := AssembleSealedEnvelope(elp, executorKey.PublicKey(), []byte("lol"))
	require.Error(w.VerifySignature())
	ex2, ok := w.Envelope.Action().(*Execution)
	require.True(ok)
	require.Equal(ex, ex2)

	// sign the Execution
	selp, err := Sign(elp, executorKey)
	require.NoError(err)
	require.NotNil(selp)
	require.EqualValues(21, ex.BasicActionSize())
	require.EqualValues(87, ex.TotalSize())

	// verify signature
	require.NoError(selp.VerifySignature())
}

func TestExecutionSanityCheck(t *testing.T) {
	require := require.New(t)
	t.Run("Negative amount", func(t *testing.T) {
		ex, err := NewExecution("2", uint64(1), big.NewInt(-100), uint64(0), big.NewInt(0), []byte{})
		require.NoError(err)
		require.Equal(ErrInvalidAmount, errors.Cause(ex.SanityCheck()))
	})

	t.Run("Invalid contract address", func(t *testing.T) {
		ex, err := NewExecution(
			identityset.Address(29).String()+"bbb",
			uint64(1),
			big.NewInt(0),
			uint64(0),
			big.NewInt(0),
			[]byte{},
		)
		require.NoError(err)
		require.Contains(ex.SanityCheck().Error(), "error when validating contract's address")
	})

	t.Run("Negative gas price", func(t *testing.T) {
		ex, err := NewExecution(identityset.Address(29).String(), uint64(1), big.NewInt(100), uint64(0), big.NewInt(-1), []byte{})
		require.NoError(err)
		require.Equal(ErrNegativeValue, errors.Cause(ex.SanityCheck()))
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
	ex, err := NewExecution(
		identityset.Address(29).String(),
		1,
		big.NewInt(20),
		uint64(100),
		big.NewInt(1000000),
		[]byte("test"),
	)
	require.NoError(err)

	ex1 := &Execution{}
	for _, v := range []struct {
		list types.AccessList
		gas  uint64
	}{
		{nil, 10400},
		{
			types.AccessList{
				{common.Address{}, nil},
			}, 12800,
		},
		{
			types.AccessList{
				{_c2, []common.Hash{{}, _k1}},
			}, 16600,
		},
		{
			types.AccessList{
				{common.Address{}, nil},
				{_c1, []common.Hash{_k1, {}, _k3}},
				{_c2, []common.Hash{_k2, _k3, _k4, _k1}},
			}, 30900,
		},
	} {
		ex.accessList = v.list
		require.NoError(ex1.LoadProto(ex.Proto()))
		ex1.AbstractAction = ex.AbstractAction
		require.Equal(ex, ex1)
		gas, err := ex.IntrinsicGas()
		require.NoError(err)
		require.Equal(v.gas, gas)
	}
}
