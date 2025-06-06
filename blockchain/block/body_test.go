// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package block

import (
	"math/big"
	"testing"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

func TestProto(t *testing.T) {
	require := require.New(t)
	body := Body{}
	blockBody := body.Proto()
	require.NotNil(blockBody)
	require.Equal(0, len(blockBody.Actions))

	body, err := makeBody()
	require.NoError(err)
	blockBody = body.Proto()
	require.NotNil(blockBody)
	require.Equal(1, len(blockBody.Actions))
}

func TestSerDer(t *testing.T) {
	require := require.New(t)
	body := Body{}
	ser, err := body.Serialize()
	require.NoError(err)
	body2, err := (&Deserializer{}).DeserializeBody(ser)
	require.NoError(err)
	require.Equal(0, len(body2.Actions))

	body, err = makeBody()
	require.NoError(err)
	ser, err = body.Serialize()
	require.NoError(err)
	body2, err = (&Deserializer{}).DeserializeBody(ser)
	require.NoError(err)
	require.Equal(1, len(body2.Actions))
	require.Equal(&body, body2)
}

func TestLoadProto(t *testing.T) {
	require := require.New(t)
	body := Body{}
	blockBody := body.Proto()
	require.NotNil(blockBody)
	body2, err := (&Deserializer{}).fromBodyProto(blockBody)
	require.NoError(err)
	require.Equal(0, len(body2.Actions))

	body, err = makeBody()
	require.NoError(err)
	blockBody = body.Proto()
	require.NotNil(blockBody)
	body2, err = (&Deserializer{}).fromBodyProto(blockBody)
	require.NoError(err)
	require.Equal(1, len(body2.Actions))
	require.Equal(body, body2)
}

func TestCalculateTxRoot(t *testing.T) {
	require := require.New(t)
	body := Body{}
	h, err := body.CalculateTxRoot()
	require.NoError(err)
	require.Equal(h, hash.ZeroHash256)

	body, err = makeBody()
	require.NoError(err)
	h, err = body.CalculateTxRoot()
	require.NoError(err)
	require.NotEqual(h, hash.ZeroHash256)
}

func TestCalculateTransferAmount(t *testing.T) {
	require := require.New(t)
	body := Body{}
	i := body.CalculateTransferAmount()
	require.Equal(i, big.NewInt(0))

	body, err := makeBody()
	require.NoError(err)
	i = body.CalculateTransferAmount()
	require.NotEqual(i, big.NewInt(0))
	require.Equal(i, big.NewInt(20))
}

func makeBody() (body Body, err error) {
	A := make([]*action.SealedEnvelope, 0)
	t := action.NewTransfer(big.NewInt(20), "", []byte("payload"))

	bd := &action.EnvelopeBuilder{}
	elp := bd.SetGasPrice(big.NewInt(10)).SetGasLimit(uint64(100000)).
		SetAction(t).Build()

	selp, err := action.Sign(elp, identityset.PrivateKey(28))
	if err != nil {
		return
	}
	A = append(A, selp)
	body = Body{A}
	return
}
