// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package account

import (
	"context"
	"math/big"
	"testing"

	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/db/batch"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_chainmanager"
)

func TestLoadOrCreateAccountState(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)
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

	for _, test := range []struct {
		accountType   int32
		addr          address.Address
		expectedNonce uint64
	}{
		{
			0,
			identityset.Address(27),
			uint64(0x1),
		},
		{
			1,
			identityset.Address(28),
			uint64(0x0),
		},
	} {
		var (
			s   *state.Account
			err error
		)
		if test.accountType == 0 {
			s, err = accountutil.LoadAccount(sm, test.addr, state.LegacyNonceAccountTypeOption())
		} else {
			s, err = accountutil.LoadAccount(sm, test.addr)
		}
		require.NoError(err)
		require.Equal(big.NewInt(0), s.Balance)
		require.Equal(test.accountType, s.AccountType())
		require.Equal(test.expectedNonce, s.PendingNonce())

		if test.accountType == 0 {
			require.NoError(createAccount(sm, test.addr.String(), big.NewInt(5), state.LegacyNonceAccountTypeOption()))
			s, err = accountutil.LoadAccount(sm, test.addr, state.LegacyNonceAccountTypeOption())
		} else {
			require.NoError(createAccount(sm, test.addr.String(), big.NewInt(5)))
			s, err = accountutil.LoadAccount(sm, test.addr)
		}
		require.NoError(err)
		require.Equal("5", s.Balance.String())
		require.Equal(test.accountType, s.AccountType())
		require.Equal(test.expectedNonce, s.PendingNonce())
	}
}

func TestProtocol_Initialize(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
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

	ge := genesis.TestDefault()
	ge.Account.InitBalanceMap = map[string]string{
		identityset.Address(0).String(): "100",
	}
	ctx := protocol.WithBlockCtx(context.Background(), protocol.BlockCtx{
		BlockHeight: 0,
	})
	ctx = protocol.WithFeatureCtx(genesis.WithGenesisContext(ctx, ge))
	p := NewProtocol(rewarding.DepositGas)
	require.NoError(
		p.CreateGenesisStates(
			ctx,
			sm,
		),
	)

	ge.Account.InitBalanceMap[identityset.Address(1).String()] = "-1"
	require.Error(
		p.CreateGenesisStates(
			ctx,
			sm,
		),
	)

	require.Error(createAccount(sm, identityset.Address(0).String(), big.NewInt(0)))
	acc0, err := accountutil.LoadAccount(sm, identityset.Address(0))
	require.NoError(err)
	require.Equal(big.NewInt(100), acc0.Balance)
}

func TestRegisterOrForceRegister(t *testing.T) {
	require := require.New(t)

	p := NewProtocol(rewarding.DepositGas)
	require.Equal(protocolID, p.Name())

	registry := protocol.NewRegistry()
	require.NoError(p.Register(registry))
	require.Error(p.Register(registry))
	require.NoError(p.ForceRegister(registry))

	foundP := FindProtocol(registry)
	require.Equal(p, foundP)

	require.Nil(FindProtocol(nil))

	registry = protocol.NewRegistry()
	require.Nil(FindProtocol(registry))
}

// TestAssertZeroBlockHeight tests the assertZeroBlockHeight funcs
func TestAssertZeroBlockHeight(t *testing.T) {
	require := require.New(t)

	p := NewProtocol(rewarding.DepositGas)
	require.NoError(p.assertZeroBlockHeight(0))
	// should return an error if height is not zero
	require.Error(p.assertZeroBlockHeight(1))
}

func TestAssertAmountsAndEqualLength(t *testing.T) {
	require := require.New(t)

	p := NewProtocol(rewarding.DepositGas)
	amounts := []*big.Int{
		big.NewInt(0),
		big.NewInt(1),
		big.NewInt(2),
	}
	require.NoError(p.assertAmounts(amounts))
	amounts[0] = big.NewInt(-1)
	require.Error(p.assertAmounts(amounts))

	addrs := []address.Address{
		&address.AddrV1{},
		&address.AddrV1{},
		&address.AddrV1{},
	}
	require.NoError(p.assertEqualLength(addrs, amounts))
	addrs = addrs[:2]
	require.Error(p.assertEqualLength(addrs, amounts))
}
