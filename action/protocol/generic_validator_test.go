// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

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

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/identityset"
)

func TestActionProtoAndGenericValidator(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	reg := NewRegistry()
	mp := NewMockProtocol(ctrl)
	require.NoError(reg.Register("1", mp))
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

	ctx = WithBlockchainCtx(
		WithRegistry(ctx, reg),
		BlockchainCtx{
			Genesis: config.Default.Genesis,
			Tip: TipInfo{
				Height:    0,
				Hash:      config.Default.Genesis.Hash(),
				Timestamp: time.Unix(config.Default.Genesis.Timestamp, 0),
			},
		},
	)

	valid := NewGenericValidator(nil, func(sr StateReader, addr string) (*state.Account, error) {
		pk := identityset.PrivateKey(27).PublicKey()
		eAddr, _ := address.FromBytes(pk.Hash())
		if strings.EqualFold(eAddr.String(), addr) {
			return nil, errors.New("MockChainManager nonce error")
		}
		return &state.Account{Nonce: 2}, nil
	})
	data, err := hex.DecodeString("")
	require.NoError(err)
	t.Run("normal", func(t *testing.T) {
		mp.EXPECT().Validate(gomock.Any(), gomock.Any()).Return(nil).Times(1)
		v, err := action.NewExecution("", 0, big.NewInt(10), uint64(10), big.NewInt(10), data)
		require.NoError(err)
		bd := &action.EnvelopeBuilder{}
		elp := bd.SetGasPrice(big.NewInt(10)).
			SetGasLimit(uint64(100000)).
			SetAction(v).Build()
		selp, err := action.Sign(elp, identityset.PrivateKey(28))
		require.NoError(err)
		nselp := action.SealedEnvelope{}
		require.NoError(nselp.LoadProto(selp.Proto()))
		require.NoError(valid.Validate(ctx, nselp))
	})
	t.Run("Gas limit low", func(t *testing.T) {
		v, err := action.NewExecution("", 0, big.NewInt(10), uint64(10), big.NewInt(10), data)
		require.NoError(err)
		bd := &action.EnvelopeBuilder{}
		elp := bd.SetGasPrice(big.NewInt(10)).
			SetGasLimit(uint64(10)).
			SetAction(v).Build()
		selp, err := action.Sign(elp, identityset.PrivateKey(28))
		require.NoError(err)
		nselp := action.SealedEnvelope{}
		require.NoError(nselp.LoadProto(selp.Proto()))
		err = valid.Validate(ctx, nselp)
		require.Error(err)
		require.True(strings.Contains(err.Error(), "insufficient gas"))
	})
	t.Run("state error", func(t *testing.T) {
		v, err := action.NewExecution("", 0, big.NewInt(10), uint64(10), big.NewInt(10), data)
		require.NoError(err)
		bd := &action.EnvelopeBuilder{}
		elp := bd.SetGasPrice(big.NewInt(10)).
			SetGasLimit(uint64(100000)).
			SetAction(v).Build()
		selp, err := action.Sign(elp, identityset.PrivateKey(27))
		require.NoError(err)
		nselp := action.SealedEnvelope{}
		require.NoError(nselp.LoadProto(selp.Proto()))
		err = valid.Validate(ctx, nselp)
		require.Error(err)
		require.True(strings.Contains(err.Error(), "invalid state of account"))
	})
	t.Run("nonce too low", func(t *testing.T) {
		v, err := action.NewExecution("", 1, big.NewInt(10), uint64(10), big.NewInt(10), data)
		require.NoError(err)
		bd := &action.EnvelopeBuilder{}
		elp := bd.SetGasPrice(big.NewInt(10)).
			SetNonce(1).
			SetGasLimit(uint64(100000)).
			SetAction(v).Build()
		selp, err := action.Sign(elp, identityset.PrivateKey(28))
		require.NoError(err)
		nselp := action.SealedEnvelope{}
		require.NoError(nselp.LoadProto(selp.Proto()))
		err = valid.Validate(ctx, nselp)
		require.Error(err)
		require.True(strings.Contains(err.Error(), "nonce is too low"))
	})
	t.Run("wrong recipient", func(t *testing.T) {
		v, err := action.NewTransfer(1, big.NewInt(1), "io1qyqsyqcyq5narhapakcsrhksfajfcpl24us3xp38zwvsep", []byte{}, uint64(100000), big.NewInt(10))
		require.NoError(err)
		bd := &action.EnvelopeBuilder{}
		elp := bd.SetAction(v).SetGasLimit(100000).
			SetGasPrice(big.NewInt(10)).
			SetNonce(1).Build()
		selp, err := action.Sign(elp, identityset.PrivateKey(27))
		require.NoError(err)
		require.Error(valid.Validate(ctx, selp))
	})
	t.Run("wrong signature", func(t *testing.T) {
		unsignedTsf, err := action.NewTransfer(uint64(1), big.NewInt(1), caller.String(), []byte{}, uint64(100000), big.NewInt(0))
		require.NoError(err)

		bd := &action.EnvelopeBuilder{}
		elp := bd.SetNonce(1).
			SetAction(unsignedTsf).
			SetGasLimit(100000).Build()
		selp := action.FakeSeal(elp, identityset.PrivateKey(27).PublicKey())
		require.True(strings.Contains(valid.Validate(ctx, selp).Error(), "failed to verify action signature"))
	})
	t.Run("protocol validation failed", func(t *testing.T) {
		me := errors.New("mock error")
		mp.EXPECT().Validate(gomock.Any(), gomock.Any()).Return(me).Times(1)
		v, err := action.NewExecution("", 0, big.NewInt(10), uint64(10), big.NewInt(10), data)
		require.NoError(err)
		bd := &action.EnvelopeBuilder{}
		elp := bd.SetGasPrice(big.NewInt(10)).
			SetGasLimit(uint64(100000)).
			SetAction(v).Build()
		selp, err := action.Sign(elp, identityset.PrivateKey(28))
		require.NoError(err)
		require.Equal(me, errors.Cause(valid.Validate(ctx, selp)))
	})
}
