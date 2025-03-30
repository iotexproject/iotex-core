// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package protocol

import (
	"context"
	"encoding/hex"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

const _evmNetworkID uint32 = 4689

func TestActionProtoAndGenericValidator(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	caller, err := address.FromString("io1mflp9m6hcgm2qcghchsdqj3z3eccrnekx9p0ms")
	require.NoError(err)
	producer, err := address.FromString("io1emxf8zzqckhgjde6dqd97ts0y3q496gm3fdrl6")
	require.NoError(err)

	ctx := WithBlockCtx(context.Background(),
		BlockCtx{
			BlockHeight: 1,
			Producer:    producer,
		})
	ctx = WithActionCtx(ctx,
		ActionCtx{
			Caller: caller,
		})

	g := genesis.TestDefault()
	ctx = WithBlockchainCtx(
		ctx,
		BlockchainCtx{
			Tip: TipInfo{
				Height:    0,
				Hash:      g.Hash(),
				Timestamp: time.Unix(g.Timestamp, 0),
			},
		},
	)

	ctx = WithFeatureCtx(genesis.WithGenesisContext(ctx, genesis.TestDefault()))

	valid := NewGenericValidator(nil, func(_ context.Context, sr StateReader, addr address.Address) (*state.Account, error) {
		pk := identityset.PrivateKey(27).PublicKey()
		eAddr := pk.Address()
		if strings.EqualFold(eAddr.String(), addr.String()) {
			return nil, errors.New("MockChainManager nonce error")
		}
		acct, err := state.NewAccount()
		if err != nil {
			return nil, err
		}
		if err := acct.SetPendingNonce(1); err != nil {
			return nil, err
		}
		if err := acct.SetPendingNonce(2); err != nil {
			return nil, err
		}
		if err := acct.SetPendingNonce(3); err != nil {
			return nil, err
		}

		return acct, nil
	})
	data, err := hex.DecodeString("")
	require.NoError(err)
	t.Run("normal", func(t *testing.T) {
		v := action.NewExecution("", big.NewInt(10), data)
		elp := (&action.EnvelopeBuilder{}).SetGasPrice(big.NewInt(10)).SetNonce(3).
			SetGasLimit(uint64(100000)).SetAction(v).Build()
		selp, err := action.Sign(elp, identityset.PrivateKey(28))
		require.NoError(err)
		nselp, err := (&action.Deserializer{}).SetEvmNetworkID(_evmNetworkID).ActionToSealedEnvelope(selp.Proto())
		require.NoError(err)
		require.NoError(valid.Validate(ctx, nselp))
	})
	t.Run("Gas limit low", func(t *testing.T) {
		v := action.NewExecution("", big.NewInt(10), data)
		elp := (&action.EnvelopeBuilder{}).SetGasPrice(big.NewInt(10)).
			SetGasLimit(10).SetAction(v).Build()
		selp, err := action.Sign(elp, identityset.PrivateKey(28))
		require.NoError(err)
		nselp, err := (&action.Deserializer{}).SetEvmNetworkID(_evmNetworkID).ActionToSealedEnvelope(selp.Proto())
		require.NoError(err)
		err = valid.Validate(ctx, nselp)
		require.Error(err)
		require.Contains(err.Error(), action.ErrIntrinsicGas.Error())
	})
	t.Run("state error", func(t *testing.T) {
		v := action.NewExecution("", big.NewInt(10), data)
		elp := (&action.EnvelopeBuilder{}).SetGasPrice(big.NewInt(10)).SetNonce(1).
			SetGasLimit(uint64(100000)).SetAction(v).Build()
		selp, err := action.Sign(elp, identityset.PrivateKey(27))
		require.NoError(err)
		nselp, err := (&action.Deserializer{}).SetEvmNetworkID(_evmNetworkID).ActionToSealedEnvelope(selp.Proto())
		require.NoError(err)
		err = valid.Validate(ctx, nselp)
		require.Error(err)
		require.Contains(err.Error(), "invalid state of account")
	})
	t.Run("invalid system action nonce", func(t *testing.T) {
		gr := action.GrantReward{}
		elp := (&action.EnvelopeBuilder{}).SetGasPrice(big.NewInt(10)).SetNonce(1).
			SetGasLimit(uint64(100000)).SetAction(&gr).Build()
		selp, err := action.Sign(elp, identityset.PrivateKey(28))
		require.NoError(err)
		nselp, err := (&action.Deserializer{}).SetEvmNetworkID(_evmNetworkID).ActionToSealedEnvelope(selp.Proto())
		require.NoError(err)
		err = valid.Validate(ctx, nselp)
		require.Equal(action.ErrSystemActionNonce, errors.Cause(err))
	})
	t.Run("nonce too low", func(t *testing.T) {
		v := action.NewExecution("", big.NewInt(10), data)
		elp := (&action.EnvelopeBuilder{}).SetGasPrice(big.NewInt(10)).SetNonce(1).
			SetGasLimit(uint64(100000)).SetAction(v).Build()
		selp, err := action.Sign(elp, identityset.PrivateKey(28))
		require.NoError(err)
		nselp, err := (&action.Deserializer{}).SetEvmNetworkID(_evmNetworkID).ActionToSealedEnvelope(selp.Proto())
		require.NoError(err)
		err = valid.Validate(ctx, nselp)
		require.Error(err)
		require.Equal(action.ErrNonceTooLow, errors.Cause(err))
	})
	t.Run("wrong recipient", func(t *testing.T) {
		v := action.NewTransfer(big.NewInt(1), "io1qyqsyqcyq5narhapakcsrhksfajfcpl24us3xp38zwvsep", []byte{})
		bd := &action.EnvelopeBuilder{}
		elp := bd.SetAction(v).SetGasLimit(100000).
			SetGasPrice(big.NewInt(10)).
			SetNonce(1).Build()
		selp, err := action.Sign(elp, identityset.PrivateKey(27))
		require.NoError(err)
		require.Error(valid.Validate(ctx, selp))
	})
	t.Run("wrong signature", func(t *testing.T) {
		unsignedTsf := action.NewTransfer(big.NewInt(1), caller.String(), []byte{})
		bd := &action.EnvelopeBuilder{}
		elp := bd.SetNonce(1).
			SetAction(unsignedTsf).
			SetGasLimit(100000).Build()
		selp := action.FakeSeal(elp, identityset.PrivateKey(27).PublicKey())
		err = valid.Validate(ctx, selp)
		require.Contains(err.Error(), action.ErrInvalidSender.Error())
	})
}
