// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"context"
	"math/big"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db/batch"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_chainmanager"
)

func TestLoadOrCreateAccountState(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	sm := mock_chainmanager.NewMockStateManager(ctrl)
	cb := batch.NewCachedBatch()
	sm.EXPECT().State(gomock.Any(), gomock.Any()).DoAndReturn(
		func(account interface{}, opts ...protocol.StateOption) (uint64, error) {
			cfg, err := protocol.CreateStateConfig(opts...)
			if err != nil {
				return 0, err
			}
			val, err := cb.Get("state", cfg.Key)
			if err != nil {
				return 0, state.ErrStateNotExist
			}
			return 0, state.Deserialize(account, val)
		}).AnyTimes()
	sm.EXPECT().PutState(gomock.Any(), gomock.Any()).DoAndReturn(
		func(account interface{}, opts ...protocol.StateOption) (uint64, error) {
			cfg, err := protocol.CreateStateConfig(opts...)
			if err != nil {
				return 0, err
			}
			ss, err := state.Serialize(account)
			if err != nil {
				return 0, err
			}
			cb.Put("state", cfg.Key, ss, "failed to put state")
			return 0, nil
		}).AnyTimes()

	addrv1 := identityset.Address(27)
	s, err := accountutil.LoadAccount(sm, hash.BytesToHash160(addrv1.Bytes()))
	require.NoError(err)
	require.Equal(s.Balance, state.EmptyAccount().Balance)
	require.NoError(createAccount(sm, addrv1.String(), big.NewInt(5)))
	s, err = accountutil.LoadAccount(sm, hash.BytesToHash160(addrv1.Bytes()))
	require.NoError(err)
	require.Equal("5", s.Balance.String())
	s, err = accountutil.LoadAccount(sm, hash.BytesToHash160(addrv1.Bytes()))
	require.NoError(err)
	require.Equal(uint64(0x0), s.Nonce)
	require.Equal("5", s.Balance.String())
}

func TestProtocol_Initialize(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	sm := mock_chainmanager.NewMockStateManager(ctrl)
	cb := batch.NewCachedBatch()
	sm.EXPECT().State(gomock.Any(), gomock.Any()).DoAndReturn(
		func(account interface{}, opts ...protocol.StateOption) (uint64, error) {
			cfg, err := protocol.CreateStateConfig(opts...)
			if err != nil {
				return 0, err
			}
			val, err := cb.Get("state", cfg.Key)
			if err != nil {
				return 0, state.ErrStateNotExist
			}
			return 0, state.Deserialize(account, val)
		}).AnyTimes()
	sm.EXPECT().PutState(gomock.Any(), gomock.Any()).DoAndReturn(
		func(account interface{}, opts ...protocol.StateOption) (uint64, error) {
			cfg, err := protocol.CreateStateConfig(opts...)
			if err != nil {
				return 0, err
			}
			ss, err := state.Serialize(account)
			if err != nil {
				return 0, err
			}
			cb.Put("state", cfg.Key, ss, "failed to put state")
			return 0, nil
		}).AnyTimes()

	ge := config.Default.Genesis
	ge.Account.InitBalanceMap = map[string]string{
		identityset.Address(0).String(): "100",
	}
	ctx := protocol.WithBlockCtx(context.Background(), protocol.BlockCtx{
		BlockHeight: 0,
	})
	ctx = protocol.WithBlockchainCtx(ctx, protocol.BlockchainCtx{
		Genesis: ge,
	})
	p := NewProtocol(rewarding.DepositGas)
	require.NoError(
		p.CreateGenesisStates(
			ctx,
			sm,
		),
	)
	require.Error(createAccount(sm, identityset.Address(0).String(), big.NewInt(0)))
	acc0, err := accountutil.LoadAccount(sm, hash.BytesToHash160(identityset.Address(0).Bytes()))
	require.NoError(err)
	require.Equal(big.NewInt(100), acc0.Balance)
}
