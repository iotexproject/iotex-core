// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestActionProtoAndVerify(t *testing.T) {
	require := require.New(t)
	data, err := hex.DecodeString("")
	require.NoError(err)
	v, err := NewExecution("", 0, big.NewInt(10), uint64(10), big.NewInt(10), data)
	require.NoError(err)
	t.Run("no error", func(t *testing.T) {
		bd := &EnvelopeBuilder{}
		elp := bd.SetGasPrice(big.NewInt(10)).
			SetGasLimit(uint64(100000)).
			SetAction(v).Build()

		selp, err := Sign(elp, identityset.PrivateKey(28))
		require.NoError(err)
		require.Equal(65, len(selp.SrcPubkey().Bytes()))
		require.NoError(selp.VerifySignature())

		nselp := &SealedEnvelope{}
		require.NoError(nselp.loadProto(selp.Proto(), _evmNetworkID))

		selpHash, err := selp.Hash()
		require.NoError(err)
		nselpHash, err := nselp.Hash()
		require.NoError(err)
		require.Equal(selpHash, nselpHash)
	})
	t.Run("empty public key", func(t *testing.T) {
		bd := &EnvelopeBuilder{}
		elp := bd.SetGasPrice(big.NewInt(10)).
			SetGasLimit(uint64(100000)).
			SetAction(v).Build()

		selp, err := Sign(elp, identityset.PrivateKey(28))
		require.NoError(err)

		selp.srcPubkey = nil

		require.EqualError(selp.VerifySignature(), "empty public key")
	})
	t.Run("invalid signature", func(t *testing.T) {
		bd := &EnvelopeBuilder{}
		elp := bd.SetGasPrice(big.NewInt(10)).
			SetGasLimit(uint64(100000)).
			SetAction(v).Build()

		selp, err := Sign(elp, identityset.PrivateKey(28))
		require.NoError(err)
		selp.signature = []byte("invalid signature")
		require.Equal(ErrInvalidSender, errors.Cause(selp.VerifySignature()))
	})
}

func TestActionFakeSeal(t *testing.T) {
	require := require.New(t)
	priKey := identityset.PrivateKey(27)
	selp1, err := SignedExecution(identityset.Address(31).String(), identityset.PrivateKey(28), 2,
		big.NewInt(1), testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64), []byte{1})
	require.NoError(err)
	selp := FakeSeal(selp1.Envelope, priKey.PublicKey())
	require.Equal(selp.srcPubkey, priKey.PublicKey())
}

func TestAbstractActionSetter(t *testing.T) {
	require := require.New(t)
	t.Run("set nonce", func(t *testing.T) {
		ex, err := NewExecution("2", uint64(1), big.NewInt(-100), uint64(0), big.NewInt(0), []byte{})
		require.NoError(err)
		require.Equal(uint64(1), ex.nonce)
		ex.SetNonce(2)
		require.Equal(uint64(2), ex.nonce)
	})

	t.Run("set gaslimit", func(t *testing.T) {
		ex, err := NewExecution("2", uint64(1), big.NewInt(-100), uint64(0), big.NewInt(0), []byte{})
		require.NoError(err)
		require.Equal(uint64(0), ex.gasLimit)
		ex.SetGasLimit(10000)
		require.Equal(uint64(10000), ex.gasLimit)
	})

	t.Run("set gasPrice", func(t *testing.T) {
		ex, err := NewExecution("2", uint64(1), big.NewInt(-100), uint64(0), big.NewInt(10), []byte{})
		require.NoError(err)
		require.Equal(big.NewInt(10), ex.gasPrice)
		ex.SetGasPrice(big.NewInt(0))
		require.Equal(big.NewInt(0), ex.gasPrice)
	})
}

func TestIsSystemAction(t *testing.T) {
	require := require.New(t)
	builder := EnvelopeBuilder{}
	cf := ClaimFromRewardingFundBuilder{}
	actClaimFromRewarding := cf.Build()
	act := builder.SetAction(&actClaimFromRewarding).Build()
	sel, err := Sign(act, identityset.PrivateKey(1))
	require.NoError(err)
	require.False(IsSystemAction(sel))

	gb := GrantRewardBuilder{}
	actGrantReward := gb.Build()
	act = builder.SetAction(&actGrantReward).Build()
	sel, err = Sign(act, identityset.PrivateKey(1))
	require.NoError(err)
	require.True(IsSystemAction(sel))

	actPollResult := NewPutPollResult(1, 1, nil)
	act = builder.SetAction(actPollResult).Build()
	sel, err = Sign(act, identityset.PrivateKey(1))
	require.NoError(err)
	require.True(IsSystemAction(sel))
}
