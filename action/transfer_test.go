// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/hex"
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

	tsf, err := NewTransfer(1, big.NewInt(10), recipientAddr.String(), []byte{}, uint64(100000), big.NewInt(10))
	require.NoError(err)
	require.Nil(tsf.srcPubkey)

	bd := &EnvelopeBuilder{}
	elp := bd.SetNonce(tsf.nonce).
		SetGasLimit(tsf.gasLimit).
		SetGasPrice(tsf.gasPrice).
		SetAction(tsf).Build()

	require.Equal("0801100118a08d0622023130522f0a0231301229696f3172633264326465377274757563616c7371763464396e673068323937743633773777766c7068", hex.EncodeToString(elp.Serialize()))

	w := AssembleSealedEnvelope(elp, senderKey.PublicKey(), []byte("lol"))
	require.Error(Verify(w))
	tsf2, ok := w.Envelope.payload.(*Transfer)
	require.True(ok)
	require.Equal(tsf, tsf2)
	require.NotNil(tsf.srcPubkey)

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

	tsf, err := NewTransfer(0, big.NewInt(10), recipientAddr.String(), []byte{}, uint64(100000), big.NewInt(10))
	require.NoError(err)

	tsf.Proto()

	bd := &EnvelopeBuilder{}
	elp := bd.SetGasLimit(uint64(100000)).
		SetGasPrice(big.NewInt(10)).
		SetAction(tsf).Build()

	elp.Serialize()

	w := AssembleSealedEnvelope(elp, senderKey.PublicKey(), []byte("lol"))
	require.Error(Verify(w))

	require.NoError(err)
	require.Equal("10", tsf.Amount().Text(10))
	require.Equal([]byte{}, tsf.Payload())
	require.Equal(uint64(100000), tsf.GasLimit())
	require.Equal("10", tsf.GasPrice().Text(10))
	require.Equal(uint64(0), tsf.Nonce())
	require.Equal(senderKey.PublicKey().HexString(), w.SrcPubkey().HexString())
	require.Equal(recipientAddr.String(), tsf.Recipient())
	require.Equal(recipientAddr.String(), tsf.Destination())
	require.Equal(uint32(87), tsf.TotalSize())

	gas, err := tsf.IntrinsicGas()
	require.NoError(err)
	require.Equal(uint64(10000), gas)
	cs, err := tsf.Cost()
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
		tsf, err := NewTransfer(uint64(1), big.NewInt(-100), "2", nil,
			uint64(100000), big.NewInt(0))
		require.NoError(err)
		require.Equal(ErrBalance, errors.Cause(tsf.SanityCheck()))
	})
	t.Run("Invalid recipient address", func(t *testing.T) {
		tsf, err := NewTransfer(
			1,
			big.NewInt(1),
			identityset.Address(28).String()+"aaa",
			nil,
			uint64(100000),
			big.NewInt(0),
		)
		require.NoError(err)
		err = tsf.SanityCheck()
		require.Error(err)
		require.True(strings.Contains(err.Error(), "error when validating recipient's address"))
	})
	t.Run("Negative gas fee", func(t *testing.T) {
		tsf, err := NewTransfer(uint64(1), big.NewInt(100), identityset.Address(28).String(), nil,
			uint64(100000), big.NewInt(-1))
		require.NoError(err)
		require.Equal(ErrGasPrice, errors.Cause(tsf.SanityCheck()))
	})
}
