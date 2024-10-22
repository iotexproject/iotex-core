// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

func TestActionProtoAndVerify(t *testing.T) {
	require := require.New(t)
	data, err := hex.DecodeString("")
	require.NoError(err)
	v := NewExecution("", big.NewInt(10), data)
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
		ab := AbstractAction{}
		require.Zero(ab.nonce)
		ab.SetNonce(2)
		require.EqualValues(2, ab.nonce)
	})

	t.Run("set gaslimit", func(t *testing.T) {
		ab := AbstractAction{}
		require.Zero(ab.gasLimit)
		ab.SetGasLimit(10000)
		require.EqualValues(10000, ab.gasLimit)
	})

	t.Run("set gasPrice", func(t *testing.T) {
		ab := AbstractAction{}
		require.Zero(ab.gasPrice)
		ab.SetGasPrice(big.NewInt(10))
		require.Equal(big.NewInt(10), ab.gasPrice)
	})
}

func TestIsSystemAction(t *testing.T) {
	require := require.New(t)
	builder := EnvelopeBuilder{}
	actClaimFromRewarding := ClaimFromRewardingFund{}
	act := builder.SetAction(&actClaimFromRewarding).Build()
	sel, err := Sign(act, identityset.PrivateKey(1))
	require.NoError(err)
	require.False(IsSystemAction(sel))

	actGrantReward := NewGrantReward(EpochReward, 1)
	act = builder.SetAction(actGrantReward).Build()
	sel, err = Sign(act, identityset.PrivateKey(1))
	require.NoError(err)
	require.True(IsSystemAction(sel))

	actPollResult := NewPutPollResult(1, nil)
	act = builder.SetAction(actPollResult).Build()
	sel, err = Sign(act, identityset.PrivateKey(1))
	require.NoError(err)
	require.True(IsSystemAction(sel))
}

func TestNewActionType(t *testing.T) {
	r := require.New(t)
	r.Equal(types.LegacyTxType, LegacyTxType)
	r.Equal(types.AccessListTxType, AccessListTxType)
	r.Equal(types.DynamicFeeTxType, DynamicFeeTxType)
	r.Equal(types.BlobTxType, BlobTxType)
}
