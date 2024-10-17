// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

func TestTransferSignVerify(t *testing.T) {
	require := require.New(t)
	recipientAddr := identityset.Address(28)
	senderKey := identityset.PrivateKey(27)

	tsf := NewTransfer(big.NewInt(10), recipientAddr.String(), []byte{})
	require.EqualValues(66, tsf.Size())

	bd := &EnvelopeBuilder{}
	eb := bd.SetNonce(1).SetGasLimit(100000).SetGasPrice(big.NewInt(10)).
		SetAction(tsf).Build()
	elp, ok := eb.(*envelope)
	require.True(ok)
	require.EqualValues(87, eb.Size())

	w := AssembleSealedEnvelope(elp, senderKey.PublicKey(), []byte("lol"))
	require.Error(w.VerifySignature())
	tsf2, ok := w.Envelope.Action().(*Transfer)
	require.True(ok)
	require.Equal(tsf, tsf2)

	// sign the transfer
	selp, err := Sign(elp, senderKey)
	require.NoError(err)
	require.NotNil(selp)
	// verify signature
	require.NoError(selp.VerifySignature())
}

func TestTransfer(t *testing.T) {
	require := require.New(t)
	recipientAddr := identityset.Address(28)
	senderKey := identityset.PrivateKey(27)

	tsf := NewTransfer(big.NewInt(10), recipientAddr.String(), []byte{})
	bd := &EnvelopeBuilder{}
	eb := bd.SetGasLimit(uint64(100000)).SetGasPrice(big.NewInt(10)).
		SetAction(tsf).Build()
	elp, ok := eb.(*envelope)
	require.True(ok)
	require.EqualValues(87, eb.Size())

	w := AssembleSealedEnvelope(elp, senderKey.PublicKey(), []byte("lol"))
	require.Error(w.VerifySignature())
	require.Equal("10", tsf.Amount().Text(10))
	require.Equal([]byte{}, tsf.Payload())
	require.Equal(uint64(100000), elp.Gas())
	require.Equal("10", elp.GasPrice().Text(10))
	require.Zero(elp.Nonce())
	require.Equal(senderKey.PublicKey().HexString(), w.SrcPubkey().HexString())
	require.Equal(recipientAddr.String(), tsf.Recipient())
	require.Equal(recipientAddr.String(), tsf.Destination())
	require.EqualValues(66, tsf.Size())

	gas, err := tsf.IntrinsicGas()
	require.NoError(err)
	require.Equal(uint64(10000), gas)
	cs, err := elp.Cost()
	require.NoError(err)
	require.Equal("100010", cs.Text(10))

	proto := tsf.Proto()
	tsf2 := &Transfer{}
	require.NoError(tsf2.LoadProto(proto))
	require.Equal("10", tsf2.Amount().Text(10))
	require.Equal([]byte{}, tsf2.Payload())
	require.Equal(recipientAddr.String(), tsf2.Recipient())
	require.Equal(recipientAddr.String(), tsf2.Destination())

	t.Run("Negative amount", func(t *testing.T) {
		tsf := NewTransfer(big.NewInt(-100), "2", nil)
		require.Equal(ErrNegativeValue, errors.Cause(tsf.SanityCheck()))
	})
	t.Run("Negative gas fee", func(t *testing.T) {
		tsf := NewTransfer(big.NewInt(100), identityset.Address(28).String(), nil)
		elp := (&EnvelopeBuilder{}).SetGasLimit(100000).
			SetGasPrice(big.NewInt(-1)).SetAction(tsf).Build()
		require.Equal(ErrNegativeValue, errors.Cause(elp.SanityCheck()))
	})
}
