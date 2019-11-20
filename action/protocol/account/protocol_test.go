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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_chainmanager"
)

func TestLoadOrCreateAccountState(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	sm := mock_chainmanager.NewMockStateManager(ctrl)
	cb := db.NewCachedBatch()
	sm.EXPECT().State(gomock.Any(), gomock.Any()).DoAndReturn(
		func(addrHash hash.Hash160, account interface{}) error {
			val, err := cb.Get("state", addrHash[:])
			if err != nil {
				return state.ErrStateNotExist
			}
			return state.Deserialize(account, val)
		}).AnyTimes()
	sm.EXPECT().PutState(gomock.Any(), gomock.Any()).DoAndReturn(
		func(addrHash hash.Hash160, account interface{}) error {
			ss, err := state.Serialize(account)
			if err != nil {
				return err
			}
			cb.Put("state", addrHash[:], ss, "failed to put state")
			return nil
		}).AnyTimes()

	addrv1 := identityset.Address(27)
	s, err := accountutil.LoadAccount(sm, hash.BytesToHash160(addrv1.Bytes()))
	require.NoError(err)
	require.Equal(s.Balance, state.EmptyAccount().Balance)
	s, err = accountutil.LoadOrCreateAccount(sm, addrv1.String(), big.NewInt(5))
	require.NoError(err)
	require.Equal("5", s.Balance.String())
	s, err = accountutil.LoadAccount(sm, hash.BytesToHash160(addrv1.Bytes()))
	require.NoError(err)
	require.Equal(uint64(0x0), s.Nonce)
	require.Equal("5", s.Balance.String())
}

func TestProtocol_Initialize(t *testing.T) {
	p := NewProtocol()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	sm := mock_chainmanager.NewMockStateManager(ctrl)
	cb := db.NewCachedBatch()
	sm.EXPECT().State(gomock.Any(), gomock.Any()).DoAndReturn(
		func(addrHash hash.Hash160, account interface{}) error {
			val, err := cb.Get("state", addrHash[:])
			if err != nil {
				return state.ErrStateNotExist
			}
			return state.Deserialize(account, val)
		}).AnyTimes()
	sm.EXPECT().PutState(gomock.Any(), gomock.Any()).DoAndReturn(
		func(addrHash hash.Hash160, account interface{}) error {
			ss, err := state.Serialize(account)
			if err != nil {
				return err
			}
			cb.Put("state", addrHash[:], ss, "failed to put state")
			return nil
		}).AnyTimes()

	require.NoError(
		t,
		p.Initialize(
			protocol.WithRunActionsCtx(context.Background(), protocol.RunActionsCtx{
				BlockHeight: 0,
				Genesis:     config.Default.Genesis,
			}),
			sm,
			[]address.Address{
				identityset.Address(0),
				identityset.Address(1),
			},
			[]*big.Int{
				big.NewInt(100),
				big.NewInt(200),
			},
		),
	)
	acc0, err := accountutil.LoadOrCreateAccount(sm, identityset.Address(0).String(), big.NewInt(0))
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(100), acc0.Balance)
	acc1, err := accountutil.LoadOrCreateAccount(sm, identityset.Address(1).String(), big.NewInt(0))
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(200), acc1.Balance)
}
