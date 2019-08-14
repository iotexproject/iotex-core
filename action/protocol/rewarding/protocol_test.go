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

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_chainmanager"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

func testProtocol(t *testing.T, test func(*testing.T, context.Context, factory.Factory, *Protocol), withExempt bool) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.Default
	stateDB, err := factory.NewStateDB(cfg, factory.InMemStateDBOption())
	require.NoError(t, err)
	require.NoError(t, stateDB.Start(context.Background()))
	defer func() {
		require.NoError(t, stateDB.Stop(context.Background()))
	}()

	chain := mock_chainmanager.NewMockChainManager(ctrl)
	chain.EXPECT().CandidatesByHeight(gomock.Any()).Return([]*state.Candidate{
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
	}, nil).AnyTimes()
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

	// Initialize the protocol
	ctx := protocol.WithRunActionsCtx(
		context.Background(),
		protocol.RunActionsCtx{
			BlockHeight: 0,
		},
	)
	ws, err := stateDB.NewWorkingSet()
	require.NoError(t, err)
	if withExempt {
		require.NoError(
			t,
			p.Initialize(
				ctx,
				ws,
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
				ws,
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

	require.NoError(t, stateDB.Commit(ws))

	ctx = protocol.WithRunActionsCtx(
		context.Background(),
		protocol.RunActionsCtx{
			Producer:    identityset.Address(27),
			Caller:      identityset.Address(28),
			BlockHeight: genesis.Default.NumDelegates * genesis.Default.NumSubEpochs,
		},
	)
	ws, err = stateDB.NewWorkingSet()
	require.NoError(t, err)
	blockReward, err := p.BlockReward(ctx, ws)
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(10), blockReward)
	epochReward, err := p.EpochReward(ctx, ws)
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(100), epochReward)
	fb, err := p.FoundationBonus(ctx, ws)
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(5), fb)
	ndffb, err := p.NumDelegatesForFoundationBonus(ctx, ws)
	require.NoError(t, err)
	assert.Equal(t, uint64(5), ndffb)
	fble, err := p.FoundationBonusLastEpoch(ctx, ws)
	require.NoError(t, err)
	assert.Equal(t, uint64(365), fble)
	pt, err := p.ProductivityThreshold(ctx, ws)
	require.NoError(t, err)
	assert.Equal(t, uint64(50), pt)

	totalBalance, err := p.TotalBalance(ctx, ws)
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(0), totalBalance)
	availableBalance, err := p.AvailableBalance(ctx, ws)
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(0), availableBalance)

	// Create a test account with 1000 token
	ws, err = stateDB.NewWorkingSet()
	require.NoError(t, err)
	_, err = accountutil.LoadOrCreateAccount(ws, identityset.Address(28).String(), big.NewInt(1000))
	require.NoError(t, err)
	require.NoError(t, stateDB.Commit(ws))

	test(t, ctx, stateDB, p)
}

func TestProtocol_Handle(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.Default
	stateDB, err := factory.NewStateDB(cfg, factory.InMemStateDBOption())
	require.NoError(t, err)
	require.NoError(t, stateDB.Start(context.Background()))
	defer func() {
		require.NoError(t, stateDB.Stop(context.Background()))
	}()
	chain := mock_chainmanager.NewMockChainManager(ctrl)
	chain.EXPECT().CandidatesByHeight(gomock.Any()).Return(
		[]*state.Candidate{
			{
				Address:       identityset.Address(0).String(),
				Votes:         unit.ConvertIotxToRau(4000000),
				RewardAddress: identityset.Address(0).String(),
			},
		},
		nil,
	).Times(1)
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
	ws, err := stateDB.NewWorkingSet()
	require.NoError(t, err)
	require.NoError(t, p.Initialize(
		ctx,
		ws,
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
	require.NoError(t, stateDB.Commit(ws))

	ctx = protocol.WithRunActionsCtx(
		context.Background(),
		protocol.RunActionsCtx{
			Producer:    identityset.Address(0),
			Caller:      identityset.Address(0),
			BlockHeight: genesis.Default.NumDelegates * genesis.Default.NumSubEpochs,
			GasPrice:    big.NewInt(0),
		},
	)

	// Create a test account with 1000000 token
	ws, err = stateDB.NewWorkingSet()
	require.NoError(t, err)
	_, err = accountutil.LoadOrCreateAccount(ws, identityset.Address(0).String(), big.NewInt(1000000))
	require.NoError(t, err)
	require.NoError(t, stateDB.Commit(ws))

	// Deposit
	ws, err = stateDB.NewWorkingSet()
	require.NoError(t, err)
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

	receipt, err := p.Handle(ctx, se1.Action(), ws)
	require.NoError(t, err)
	balance, err := p.TotalBalance(ctx, ws)
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(2000000), balance)

	// Grant
	ws, err = stateDB.NewWorkingSet()
	require.NoError(t, err)
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

	receipt, err = p.Handle(ctx, se2.Action(), ws)
	require.NoError(t, err)
	assert.Equal(t, uint64(iotextypes.ReceiptStatus_Success), receipt.Status)
	assert.Equal(t, 1, len(receipt.Logs))
	// Grant the block reward again should fail
	receipt, err = p.Handle(ctx, se2.Action(), ws)
	require.NoError(t, err)
	assert.Equal(t, uint64(iotextypes.ReceiptStatus_Failure), receipt.Status)
}
