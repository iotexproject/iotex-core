// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package block

import (
	"math/big"
	"testing"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/test/testaddress"
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
	require.NoError(body.Deserialize(ser))
	require.Equal(0, len(body.Actions))

	body, err = makeBody()
	require.NoError(err)
	ser, err = body.Serialize()
	require.NoError(err)
	require.NoError(body.Deserialize(ser))
	require.Equal(1, len(body.Actions))
}

func TestLoadProto(t *testing.T) {
	require := require.New(t)
	body := Body{}
	blockBody := body.Proto()
	require.NotNil(blockBody)
	require.NoError(body.LoadProto(blockBody))
	require.Equal(0, len(body.Actions))

	body, err := makeBody()
	require.NoError(err)
	blockBody = body.Proto()
	require.NotNil(blockBody)
	require.NoError(body.LoadProto(blockBody))
	require.Equal(1, len(body.Actions))
}

func TestCalculateTxRoot(t *testing.T) {
	require := require.New(t)
	body := Body{}
	h := body.CalculateTxRoot()
	require.Equal(h, hash.ZeroHash256)

	body, err := makeBody()
	require.NoError(err)
	h = body.CalculateTxRoot()
	require.NotEqual(h, hash.ZeroHash256)
}

func makeBody() (body Body, err error) {
	A := make([]action.SealedEnvelope, 0)
	v, err := action.NewExecution("", 0, big.NewInt(10), uint64(10), big.NewInt(10), []byte("data"))
	if err != nil {
		return
	}
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetGasPrice(big.NewInt(10)).
		SetGasLimit(uint64(100000)).
		SetAction(v).Build()

	selp, err := action.Sign(elp, testaddress.Keyinfo["alfa"].PriKey)
	if err != nil {
		return
	}
	A = append(A, selp)
	body = Body{A}
	return
}
