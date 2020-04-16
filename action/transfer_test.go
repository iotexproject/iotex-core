// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"
	"strings"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/test/identityset"
)

func TestTransferSignVerify(t *testing.T) {
	require := require.New(t)
	recipientAddr := identityset.Address(28)
	senderKey := identityset.PrivateKey(27)

	tsf, err := NewTransfer(big.NewInt(10), recipientAddr.String(), []byte{})
	require.NoError(err)

	tsf.Proto()

	bd := &EnvelopeBuilder{}
	elp, err := bd.SetGasLimit(uint64(100000)).
		SetGasPrice(big.NewInt(10)).
		SetNonce(0).
		SetAction(tsf).Build()
	require.NoError(err)

	elp.Serialize()

	w := AssembleSealedEnvelope(elp, senderKey.PublicKey(), []byte("lol"))
	require.Error(Verify(w))

	// sign the transfer
	selp, err := Sign(elp, senderKey)
	require.NoError(err)
	require.NotNil(selp)

	// verify signature
	require.NoError(Verify(selp))
}

func TestTransfer(t *testing.T) {
	require := require.New(t)
	recipientAddr := identityset.Address(28)
	senderKey := identityset.PrivateKey(27)

	tsf, err := NewTransfer(big.NewInt(10), recipientAddr.String(), []byte{})
	require.NoError(err)

	tsf.Proto()

	bd := &EnvelopeBuilder{}
	elp, err := bd.SetGasLimit(uint64(100000)).
		SetGasPrice(big.NewInt(10)).
		SetNonce(0).
		SetAction(tsf).Build()
	require.NoError(err)

	elp.Serialize()

	w := AssembleSealedEnvelope(elp, senderKey.PublicKey(), []byte("lol"))
	require.Error(Verify(w))

	require.NoError(err)
	require.Equal("10", tsf.Amount().Text(10))
	require.Equal([]byte{}, tsf.Payload())
	require.Equal(uint64(100000), elp.GasLimit())
	require.Equal("10", elp.GasPrice().Text(10))
	require.Equal(uint64(0), elp.Nonce())
	require.Equal(senderKey.PublicKey().HexString(), w.SrcPubkey().HexString())
	require.Equal(recipientAddr.String(), tsf.Recipient())
	require.Equal(recipientAddr.String(), tsf.Destination())
	require.Equal(uint32(1), tsf.TotalSize())

	gas, err := tsf.IntrinsicGas()
	require.NoError(err)
	require.Equal(uint64(10000), gas)
	cs, err := tsf.Cost()
	require.NoError(err)
	require.Equal("10", cs.Text(10))
	ec, err := elp.EstimatedCost()
	require.NoError(err)
	require.Equal("100010", ec.Text(10))

	proto := tsf.Proto()
	tsf2 := &Transfer{}
	require.NoError(tsf2.LoadProto(proto))
	require.Equal("10", tsf2.Amount().Text(10))
	require.Equal([]byte{}, tsf2.Payload())
	require.Equal(recipientAddr.String(), tsf2.Recipient())
	require.Equal(recipientAddr.String(), tsf2.Destination())

	t.Run("Negative amount", func(t *testing.T) {
		tsf, err := NewTransfer(big.NewInt(-100), "2", nil)
		require.NoError(err)
		require.Equal(ErrBalance, errors.Cause(tsf.SanityCheck()))
	})
	t.Run("Invalid recipient address", func(t *testing.T) {
		tsf, err := NewTransfer(
			big.NewInt(1),
			identityset.Address(28).String()+"aaa",
			nil,
		)
		require.NoError(err)
		err = tsf.SanityCheck()
		require.Error(err)
		require.True(strings.Contains(err.Error(), "error when validating recipient's address"))
	})
	t.Run("Negative gas fee", func(t *testing.T) {
		tsf, err := NewTransfer(big.NewInt(100), identityset.Address(28).String(), nil)
		require.NoError(err)
		bd := &EnvelopeBuilder{}
		_, err = bd.SetGasLimit(uint64(100000)).
			SetGasPrice(big.NewInt(-10)).
			SetNonce(0).
			SetAction(tsf).Build()
		require.Error(err)
		require.EqualError(ErrGasPrice, err.Error())
	})
}
