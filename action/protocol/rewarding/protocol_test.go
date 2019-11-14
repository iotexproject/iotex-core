// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"context"
	"math/big"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_chainmanager"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

func testProtocol(t *testing.T, test func(*testing.T, context.Context, protocol.StateManager, *Protocol), withExempt bool) {
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

	chain := mock_chainmanager.NewMockChainManager(ctrl)
	chain.EXPECT().ProductivityByEpoch(gomock.Any()).Return(
		uint64(19),
		map[string]uint64{
			identityset.Address(27).String(): 3,
			identityset.Address(28).String(): 7,
			identityset.Address(29).String(): 1,
			identityset.Address(30).String(): 6,
			identityset.Address(31).String(): 2,
		},
		nil,
	).AnyTimes()
	p := NewProtocol(chain, rolldpos.NewProtocol(
		genesis.Default.NumCandidateDelegates,
		genesis.Default.NumDelegates,
		genesis.Default.NumSubEpochs,
	))
	candidates := []*state.Candidate{
		{
			Address:       identityset.Address(27).String(),
			Votes:         unit.ConvertIotxToRau(4000000),
			RewardAddress: identityset.Address(0).String(),
		},
		{
			Address:       identityset.Address(28).String(),
			Votes:         unit.ConvertIotxToRau(3000000),
			RewardAddress: identityset.Address(28).String(),
		},
		{
			Address:       identityset.Address(29).String(),
			Votes:         unit.ConvertIotxToRau(2000000),
			RewardAddress: identityset.Address(29).String(),
		},
		{
			Address:       identityset.Address(30).String(),
			Votes:         unit.ConvertIotxToRau(1000000),
			RewardAddress: identityset.Address(30).String(),
		},
		{
			Address:       identityset.Address(31).String(),
			Votes:         unit.ConvertIotxToRau(500000),
			RewardAddress: identityset.Address(31).String(),
		},
		{
			Address:       identityset.Address(32).String(),
			Votes:         unit.ConvertIotxToRau(500000),
			RewardAddress: identityset.Address(32).String(),
		},
	}
	// Initialize the protocol
	ctx := protocol.WithRunActionsCtx(
		context.Background(),
		protocol.RunActionsCtx{
			BlockHeight: 0,
			Candidates:  candidates,
		},
	)
	if withExempt {
		require.NoError(
			t,
			p.Initialize(
				ctx,
				sm,
				big.NewInt(0),
				big.NewInt(10),
				big.NewInt(100),
				10,
				[]address.Address{
					identityset.Address(31),
				},
				big.NewInt(5),
				5,
				365,
				50,
			))
	} else {
		require.NoError(
			t,
			p.Initialize(
				ctx,
				sm,
				big.NewInt(0),
				big.NewInt(10),
				big.NewInt(100),
				4,
				nil,
				big.NewInt(5),
				5,
				365,
				50,
			))
	}

	ctx = protocol.WithRunActionsCtx(
		context.Background(),
		protocol.RunActionsCtx{
			Producer:    identityset.Address(27),
			Caller:      identityset.Address(28),
			Candidates:  candidates,
			BlockHeight: genesis.Default.NumDelegates * genesis.Default.NumSubEpochs,
		},
	)

	blockReward, err := p.BlockReward(ctx, sm)
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(10), blockReward)
	epochReward, err := p.EpochReward(ctx, sm)
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(100), epochReward)
	fb, err := p.FoundationBonus(ctx, sm)
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(5), fb)
	ndffb, err := p.NumDelegatesForFoundationBonus(ctx, sm)
	require.NoError(t, err)
	assert.Equal(t, uint64(5), ndffb)
	fble, err := p.FoundationBonusLastEpoch(ctx, sm)
	require.NoError(t, err)
	assert.Equal(t, uint64(365), fble)
	pt, err := p.ProductivityThreshold(ctx, sm)
	require.NoError(t, err)
	assert.Equal(t, uint64(50), pt)

	totalBalance, err := p.TotalBalance(ctx, sm)
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(0), totalBalance)
	availableBalance, err := p.AvailableBalance(ctx, sm)
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(0), availableBalance)

	// Create a test account with 1000 token
	_, err = accountutil.LoadOrCreateAccount(sm, identityset.Address(28).String(), big.NewInt(1000))
	require.NoError(t, err)

	test(t, ctx, sm, p)
}

func TestProtocol_Handle(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.Default

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
	sm.EXPECT().Snapshot().Return(1).AnyTimes()
	sm.EXPECT().Revert(gomock.Any()).Return(nil).AnyTimes()

	chain := mock_chainmanager.NewMockChainManager(ctrl)
	rp := rolldpos.NewProtocol(
		cfg.Genesis.NumCandidateDelegates,
		cfg.Genesis.NumDelegates,
		cfg.Genesis.NumSubEpochs,
	)
	p := NewProtocol(chain, rp)

	ctx := protocol.WithRunActionsCtx(
		context.Background(),
		protocol.RunActionsCtx{
			BlockHeight: 0,
		},
	)
	require.NoError(t, p.Initialize(
		ctx,
		sm,
		big.NewInt(1000000),
		big.NewInt(10),
		big.NewInt(100),
		10,
		nil,
		big.NewInt(5),
		5,
		0,
		50,
	))

	ctx = protocol.WithRunActionsCtx(
		context.Background(),
		protocol.RunActionsCtx{
			Producer:    identityset.Address(0),
			Caller:      identityset.Address(0),
			BlockHeight: genesis.Default.NumDelegates * genesis.Default.NumSubEpochs,
			GasPrice:    big.NewInt(0),
			Candidates: []*state.Candidate{
				{
					Address:       identityset.Address(0).String(),
					Votes:         unit.ConvertIotxToRau(4000000),
					RewardAddress: identityset.Address(0).String(),
				},
			},
		},
	)

	// Create a test account with 1000000 token
	_, err := accountutil.LoadOrCreateAccount(sm, identityset.Address(0).String(), big.NewInt(1000000))
	require.NoError(t, err)

	// Deposit
	db := action.DepositToRewardingFundBuilder{}
	deposit := db.SetAmount(big.NewInt(1000000)).Build()
	eb1 := action.EnvelopeBuilder{}
	e1 := eb1.SetNonce(0).
		SetGasPrice(big.NewInt(0)).
		SetGasLimit(deposit.GasLimit()).
		SetAction(&deposit).
		Build()
	se1, err := action.Sign(e1, identityset.PrivateKey(0))
	require.NoError(t, err)

	receipt, err := p.Handle(ctx, se1.Action(), sm)
	require.NoError(t, err)
	balance, err := p.TotalBalance(ctx, sm)
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(2000000), balance)

	// Grant
	gb := action.GrantRewardBuilder{}
	grant := gb.SetRewardType(action.BlockReward).Build()
	eb2 := action.EnvelopeBuilder{}
	e2 := eb2.SetNonce(0).
		SetGasPrice(big.NewInt(0)).
		SetGasLimit(grant.GasLimit()).
		SetAction(&grant).
		Build()
	se2, err := action.Sign(e2, identityset.PrivateKey(0))
	require.NoError(t, err)

	receipt, err = p.Handle(ctx, se2.Action(), sm)
	require.NoError(t, err)
	assert.Equal(t, uint64(iotextypes.ReceiptStatus_Success), receipt.Status)
	assert.Equal(t, 1, len(receipt.Logs))
	// Grant the block reward again should fail
	receipt, err = p.Handle(ctx, se2.Action(), sm)
	require.NoError(t, err)
	assert.Equal(t, uint64(iotextypes.ReceiptStatus_Failure), receipt.Status)
}
