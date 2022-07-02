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
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	"github.com/iotexproject/iotex-core/action/protocol/poll"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db/batch"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_chainmanager"
	"github.com/iotexproject/iotex-core/test/mock/mock_poll"
	"github.com/iotexproject/iotex-core/testutil/testdb"
)

func TestValidateExtension(t *testing.T) {
	r := require.New(t)

	g := config.Default.Genesis.Rewarding
	r.NoError(validateFoundationBonusExtension(g))

	last := g.FoundationBonusP2StartEpoch
	g.FoundationBonusP2StartEpoch = g.FoundationBonusLastEpoch - 1
	r.Equal(errInvalidEpoch, validateFoundationBonusExtension(g))
	g.FoundationBonusP2StartEpoch = last

	last = g.FoundationBonusP2EndEpoch
	g.FoundationBonusP2EndEpoch = g.FoundationBonusP2StartEpoch - 1
	r.Equal(errInvalidEpoch, validateFoundationBonusExtension(g))
	g.FoundationBonusP2EndEpoch = last
}

func testProtocol(t *testing.T, test func(*testing.T, context.Context, protocol.StateManager, *Protocol), withExempt bool) {
	ctrl := gomock.NewController(t)

	registry := protocol.NewRegistry()
	sm := testdb.NewMockStateManager(ctrl)

	g := config.Default.Genesis
	// Create a test account with 1000 token
	g.InitBalanceMap[identityset.Address(28).String()] = "1000"
	g.Rewarding.InitBalanceStr = "0"
	g.Rewarding.ExemptAddrStrsFromEpochReward = []string{}
	g.Rewarding.BlockRewardStr = "10"
	g.Rewarding.EpochRewardStr = "100"
	g.Rewarding.NumDelegatesForEpochReward = 4
	g.Rewarding.FoundationBonusStr = "5"
	g.Rewarding.NumDelegatesForFoundationBonus = 5
	g.Rewarding.FoundationBonusLastEpoch = 365
	g.Rewarding.ProductivityThreshold = 50
	// Initialize the protocol
	if withExempt {
		g.Rewarding.ExemptAddrStrsFromEpochReward = []string{
			identityset.Address(31).String(),
		}
		g.Rewarding.NumDelegatesForEpochReward = 10
	}
	rp := rolldpos.NewProtocol(
		g.NumCandidateDelegates,
		g.NumDelegates,
		g.NumSubEpochs,
		rolldpos.EnableDardanellesSubEpoch(g.DardanellesBlockHeight, g.DardanellesNumSubEpochs),
	)
	p := NewProtocol(g.Rewarding)
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
	abps := []*state.Candidate{
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
	}
	pp := mock_poll.NewMockProtocol(ctrl)
	pp.EXPECT().Candidates(gomock.Any(), gomock.Any()).Return(candidates, nil).AnyTimes()
	pp.EXPECT().Delegates(gomock.Any(), gomock.Any()).Return(abps, nil).AnyTimes()
	pp.EXPECT().Register(gomock.Any()).DoAndReturn(func(reg *protocol.Registry) error {
		return reg.Register("poll", pp)
	}).AnyTimes()
	pp.EXPECT().CalculateUnproductiveDelegates(gomock.Any(), gomock.Any()).Return(
		[]string{
			identityset.Address(29).String(),
			identityset.Address(31).String(),
		}, nil,
	).AnyTimes()
	require.NoError(t, rp.Register(registry))
	require.NoError(t, pp.Register(registry))
	require.NoError(t, p.Register(registry))

	ctx := protocol.WithBlockCtx(
		context.Background(),
		protocol.BlockCtx{
			BlockHeight: 0,
		},
	)
	ctx = genesis.WithGenesisContext(ctx, g)
	ctx = protocol.WithFeatureCtx(ctx)
	ap := account.NewProtocol(DepositGas)
	require.NoError(t, ap.Register(registry))
	require.NoError(t, ap.CreateGenesisStates(ctx, sm))
	require.NoError(t, p.CreateGenesisStates(ctx, sm))

	ctx = protocol.WithBlockCtx(
		ctx, protocol.BlockCtx{
			Producer:    identityset.Address(27),
			BlockHeight: genesis.Default.NumDelegates * genesis.Default.NumSubEpochs,
		},
	)
	ctx = protocol.WithActionCtx(
		ctx, protocol.ActionCtx{
			Caller: identityset.Address(28),
		},
	)
	ctx = protocol.WithBlockchainCtx(
		protocol.WithRegistry(ctx, registry), protocol.BlockchainCtx{
			Tip: protocol.TipInfo{
				Height: 20,
			},
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

	totalBalance, _, err := p.TotalBalance(ctx, sm)
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(0), totalBalance)
	availableBalance, _, err := p.AvailableBalance(ctx, sm)
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(0), availableBalance)

	test(t, ctx, sm, p)
}

func TestProtocol_Validate(t *testing.T) {
	g := config.Default.Genesis
	g.NewfoundlandBlockHeight = 0
	p := NewProtocol(g.Rewarding)
	act := createGrantRewardAction(0, uint64(0)).Action()
	ctx := protocol.WithBlockCtx(
		context.Background(),
		protocol.BlockCtx{
			Producer:    identityset.Address(0),
			BlockHeight: genesis.Default.NumDelegates * genesis.Default.NumSubEpochs,
		},
	)
	ctx = genesis.WithGenesisContext(
		ctx,
		genesis.Genesis{},
	)
	ctx = protocol.WithActionCtx(
		protocol.WithFeatureCtx(ctx),
		protocol.ActionCtx{
			Caller:   identityset.Address(0),
			GasPrice: big.NewInt(0),
		},
	)
	require.NoError(t, p.Validate(ctx, act, nil))
	ctx = protocol.WithActionCtx(
		ctx,
		protocol.ActionCtx{
			Caller:   identityset.Address(1),
			GasPrice: big.NewInt(0),
		},
	)
	require.Error(t, p.Validate(ctx, act, nil))
	ctx = protocol.WithActionCtx(
		ctx,
		protocol.ActionCtx{
			Caller:   identityset.Address(0),
			GasPrice: big.NewInt(1),
		},
	)
	require.Error(t, p.Validate(ctx, act, nil))
	require.Error(t, p.Validate(ctx, act, nil))
	ctx = protocol.WithActionCtx(
		ctx,
		protocol.ActionCtx{
			Caller:       identityset.Address(0),
			GasPrice:     big.NewInt(0),
			IntrinsicGas: 1,
		},
	)
	require.Error(t, p.Validate(ctx, act, nil))
}

func TestProtocol_Handle(t *testing.T) {
	ctrl := gomock.NewController(t)

	g := config.Default.Genesis
	registry := protocol.NewRegistry()
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
	sm.EXPECT().Snapshot().Return(1).AnyTimes()
	sm.EXPECT().Revert(gomock.Any()).Return(nil).AnyTimes()

	g.Rewarding.InitBalanceStr = "1000000"
	g.Rewarding.BlockRewardStr = "10"
	g.Rewarding.EpochRewardStr = "100"
	g.Rewarding.NumDelegatesForEpochReward = 10
	g.Rewarding.ExemptAddrStrsFromEpochReward = []string{}
	g.Rewarding.FoundationBonusStr = "5"
	g.Rewarding.NumDelegatesForFoundationBonus = 5
	g.Rewarding.FoundationBonusLastEpoch = 0
	g.Rewarding.ProductivityThreshold = 50
	// Create a test account with 1000000 token
	g.InitBalanceMap[identityset.Address(0).String()] = "1000000"
	g.NumSubEpochs = 15
	rp := rolldpos.NewProtocol(
		g.NumCandidateDelegates,
		g.NumDelegates,
		g.NumSubEpochs,
		rolldpos.EnableDardanellesSubEpoch(g.DardanellesBlockHeight, g.DardanellesNumSubEpochs),
	)
	require.Equal(t, g.FairbankBlockHeight, rp.GetEpochHeight(g.FoundationBonusP2StartEpoch))
	require.Equal(t, g.FoundationBonusP2StartEpoch, rp.GetEpochNum(g.FairbankBlockHeight))
	require.Equal(t, g.FoundationBonusP2EndEpoch, g.FoundationBonusP2StartEpoch+24*365)
	require.NoError(t, rp.Register(registry))
	pp := poll.NewLifeLongDelegatesProtocol(g.Delegates)
	require.NoError(t, pp.Register(registry))
	p := NewProtocol(g.Rewarding)
	require.NoError(t, p.Register(registry))
	// Test for ForceRegister
	require.NoError(t, p.ForceRegister(registry))

	// address package also defined protocol address, make sure they match
	require.Equal(t, p.addr.Bytes(), address.RewardingProtocolAddrHash[:])

	ctx := protocol.WithBlockCtx(
		context.Background(),
		protocol.BlockCtx{
			BlockHeight: 0,
		},
	)

	ctx = genesis.WithGenesisContext(protocol.WithRegistry(ctx, registry), g)
	ctx = protocol.WithFeatureCtx(ctx)
	ap := account.NewProtocol(DepositGas)
	require.NoError(t, ap.Register(registry))
	require.NoError(t, ap.CreateGenesisStates(ctx, sm))
	require.NoError(t, p.CreateGenesisStates(ctx, sm))

	ctx = protocol.WithBlockCtx(
		ctx,
		protocol.BlockCtx{
			Producer:    identityset.Address(0),
			BlockHeight: genesis.Default.NumDelegates * genesis.Default.NumSubEpochs,
		},
	)
	ctx = protocol.WithActionCtx(
		ctx,
		protocol.ActionCtx{
			Caller:   identityset.Address(0),
			GasPrice: big.NewInt(0),
			Nonce:    1,
		},
	)

	// Deposit
	db := action.DepositToRewardingFundBuilder{}
	deposit := db.SetAmount(big.NewInt(1000000)).Build()
	eb1 := action.EnvelopeBuilder{}
	e1 := eb1.SetNonce(1).
		SetGasPrice(big.NewInt(0)).
		SetGasLimit(deposit.GasLimit()).
		SetAction(&deposit).
		Build()
	se1, err := action.Sign(e1, identityset.PrivateKey(0))
	require.NoError(t, err)

	_, err = p.Handle(ctx, se1.Action(), sm)
	require.NoError(t, err)
	balance, _, err := p.TotalBalance(ctx, sm)
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(2000000), balance)

	// Grant
	// Test for createGrantRewardAction
	e2 := createGrantRewardAction(0, uint64(0))
	se2, err := action.Sign(e2, identityset.PrivateKey(0))
	require.NoError(t, err)
	ctx = protocol.WithActionCtx(
		ctx,
		protocol.ActionCtx{
			Caller:   identityset.Address(0),
			GasPrice: big.NewInt(0),
			Nonce:    0,
		},
	)
	receipt, err := p.Handle(ctx, se2.Action(), sm)
	require.NoError(t, err)
	assert.Equal(t, uint64(iotextypes.ReceiptStatus_Success), receipt.Status)
	assert.Equal(t, 1, len(receipt.Logs()))
	ctx = protocol.WithActionCtx(
		ctx,
		protocol.ActionCtx{
			Caller:   identityset.Address(0),
			GasPrice: big.NewInt(0),
			Nonce:    0,
		},
	)
	// Grant the block reward again should fail
	receipt, err = p.Handle(ctx, se2.Action(), sm)
	require.NoError(t, err)
	assert.Equal(t, uint64(iotextypes.ReceiptStatus_Failure), receipt.Status)

	// Claim
	claimBuilder := action.ClaimFromRewardingFundBuilder{}
	claim := claimBuilder.SetAmount(big.NewInt(1000000)).Build()
	eb3 := action.EnvelopeBuilder{}
	e3 := eb3.SetNonce(4).
		SetGasPrice(big.NewInt(0)).
		SetGasLimit(claim.GasLimit()).
		SetAction(&claim).
		Build()
	se3, err := action.Sign(e3, identityset.PrivateKey(0))
	require.NoError(t, err)
	ctx = protocol.WithActionCtx(
		ctx,
		protocol.ActionCtx{
			Caller:   identityset.Address(0),
			GasPrice: big.NewInt(0),
			Nonce:    2,
		},
	)
	_, err = p.Handle(ctx, se3.Action(), sm)
	require.NoError(t, err)
	balance, _, err = p.TotalBalance(ctx, sm)
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(1000000), balance)

	// Test CreatePreStates
	ctx = protocol.WithBlockCtx(
		ctx,
		protocol.BlockCtx{
			BlockHeight: 1816201,
		},
	)
	require.NoError(t, p.CreatePreStates(ctx, sm))
	blockReward, err := p.BlockReward(ctx, sm)
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(8000000000000000000), blockReward)

	ctx = protocol.WithBlockCtx(
		ctx,
		protocol.BlockCtx{
			BlockHeight: 864001,
		},
	)
	require.NoError(t, p.CreatePreStates(ctx, sm))
	BlockReward, err := p.BlockReward(ctx, sm)
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(8000000000000000000), BlockReward)

	// Test for CreatePostSystemActions
	grants, err := p.CreatePostSystemActions(ctx, sm)
	require.NoError(t, err)
	require.NotNil(t, grants)

	// Test for ReadState
	testMethods := []struct {
		input  string
		expect []byte
	}{
		{
			input:  "AvailableBalance",
			expect: []byte{49, 57, 57, 57, 57, 57, 48},
		},
		{
			input:  "TotalBalance",
			expect: []byte{49, 48, 48, 48, 48, 48, 48},
		},
		{
			input:  "UnclaimedBalance",
			expect: []byte{48},
		},
	}

	for _, ts := range testMethods {

		if ts.input == "UnclaimedBalance" {
			UnclaimedBalance, _, err := p.ReadState(ctx, sm, []byte(ts.input), nil)
			require.Nil(t, UnclaimedBalance)
			require.Error(t, err)

			arg1 := []byte("io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqd39ym7")
			arg2 := []byte("io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqd39ym8")
			UnclaimedBalance, _, err = p.ReadState(ctx, sm, []byte(ts.input), arg1, arg2)
			require.Nil(t, UnclaimedBalance)
			require.Error(t, err)

			UnclaimedBalance, _, err = p.ReadState(ctx, sm, []byte(ts.input), arg1)
			require.Equal(t, ts.expect, UnclaimedBalance)
			require.NoError(t, err)
			continue
		}

		output, _, err := p.ReadState(ctx, sm, []byte(ts.input), nil)
		require.NoError(t, err)
		require.Equal(t, ts.expect, output)
	}

	// Test for deleteState
	sm.EXPECT().DelState(gomock.Any()).DoAndReturn(func(addrHash hash.Hash160) error {
		cb.Delete("state", addrHash[:], "failed to delete state")
		return nil
	}).AnyTimes()
}

func TestStateCheckLegacy(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)
	p := NewProtocol(genesis.Default.Rewarding)
	chainCtx := genesis.WithGenesisContext(
		context.Background(),
		genesis.Genesis{
			Blockchain: genesis.Blockchain{GreenlandBlockHeight: 3},
		},
	)
	ctx := protocol.WithBlockCtx(chainCtx, protocol.BlockCtx{
		BlockHeight: 2,
	})
	ctx = protocol.WithFeatureCtx(ctx)

	tests := []struct {
		before, add *big.Int
		v1          [2]bool
	}{
		{
			big.NewInt(100), big.NewInt(20), [2]bool{true, true},
		},
		{
			big.NewInt(120), big.NewInt(30), [2]bool{true, false},
		},
	}

	// put V1 value
	addr := identityset.Address(1)
	acc := rewardAccount{
		balance: tests[0].before,
	}
	accKey := append(_adminKey, addr.Bytes()...)
	require.NoError(p.putState(ctx, sm, accKey, &acc))

	for useV2 := 0; useV2 < 2; useV2++ {
		if useV2 == 0 {
			require.False(useV2Storage(ctx))
		} else {
			require.True(useV2Storage(ctx))
		}
		for i := 0; i < 2; i++ {
			_, v1, err := p.stateCheckLegacy(ctx, sm, accKey, &acc)
			require.Equal(tests[useV2].v1[i], v1)
			require.NoError(err)
			if i == 0 {
				require.Equal(tests[useV2].before, acc.balance)
			} else {
				require.Equal(tests[useV2].before.Add(tests[useV2].before, tests[useV2].add), acc.balance)
			}
			if i == 0 {
				require.NoError(p.grantToAccount(ctx, sm, addr, tests[useV2].add))
			}
		}

		if useV2 == 0 {
			// switch to test V2
			ctx = protocol.WithBlockCtx(chainCtx, protocol.BlockCtx{
				BlockHeight: 3,
			})
			ctx = protocol.WithFeatureCtx(ctx)
		}
	}

	// verify V1 deleted
	_, err := p.stateV1(sm, accKey, &acc)
	require.Equal(state.ErrStateNotExist, err)
	_, err = p.stateV2(sm, accKey, &acc)
	require.NoError(err)
	require.Equal(tests[1].before, acc.balance)
}

func TestMigrateValue(t *testing.T) {
	r := require.New(t)

	a1 := admin{
		blockReward:                    big.NewInt(10),
		epochReward:                    big.NewInt(100),
		numDelegatesForEpochReward:     10,
		foundationBonus:                big.NewInt(5),
		numDelegatesForFoundationBonus: 5,
		foundationBonusLastEpoch:       365,
		productivityThreshold:          50,
	}
	f1 := fund{
		totalBalance:     new(big.Int),
		unclaimedBalance: new(big.Int),
	}
	e1 := exempt{
		[]address.Address{identityset.Address(31)},
	}
	g := genesis.Default

	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		// verify v1 state
		a := admin{}
		_, err := p.stateV1(sm, _adminKey, &a)
		r.NoError(err)
		r.Equal(a1, a)

		f := fund{}
		_, err = p.stateV1(sm, _fundKey, &f)
		r.NoError(err)
		r.Equal(f1, f)

		e := exempt{}
		_, err = p.stateV1(sm, _exemptKey, &e)
		r.NoError(err)
		r.Equal(e1, e)

		// use numSubEpochs = 15
		rp := rolldpos.NewProtocol(
			g.NumCandidateDelegates,
			g.NumDelegates,
			15,
			rolldpos.EnableDardanellesSubEpoch(g.DardanellesBlockHeight, g.DardanellesNumSubEpochs),
		)
		reg, ok := protocol.GetRegistry(ctx)
		r.True(ok)
		r.NoError(rp.ForceRegister(reg))

		for _, v := range []struct {
			height, lastEpoch uint64
		}{
			{g.GreenlandBlockHeight, a1.foundationBonusLastEpoch},
			{1641601, g.FoundationBonusP2EndEpoch},
			{g.KamchatkaBlockHeight, 30473},
		} {
			fCtx := ctx
			if v.height == 1641601 {
				// test the case where newStartEpoch < cfg.FoundationBonusP2StartEpoch
				g1 := g
				g1.GreenlandBlockHeight = v.height - 720
				g1.KamchatkaBlockHeight = v.height
				fCtx = genesis.WithGenesisContext(ctx, g1)
			}
			blkCtx := protocol.MustGetBlockCtx(ctx)
			blkCtx.BlockHeight = v.height
			fCtx = protocol.WithFeatureCtx(protocol.WithBlockCtx(fCtx, blkCtx))
			r.NoError(p.CreatePreStates(fCtx, sm))

			// verify v1 is deleted
			_, err = p.stateV1(sm, _adminKey, &a)
			r.Equal(state.ErrStateNotExist, err)
			_, err = p.stateV1(sm, _fundKey, &f)
			r.Equal(state.ErrStateNotExist, err)
			_, err = p.stateV1(sm, _exemptKey, &e)
			r.Equal(state.ErrStateNotExist, err)

			// verify v2 exist
			_, err = p.stateV2(sm, _adminKey, &a)
			r.NoError(err)
			_, err = p.stateV2(sm, _fundKey, &f)
			r.NoError(err)
			r.Equal(f1, f)
			_, err = p.stateV2(sm, _exemptKey, &e)
			r.NoError(err)
			r.Equal(e1, e)

			switch v.height {
			case g.GreenlandBlockHeight:
				r.Equal(a1, a)
				// test migrate with no data
				r.NoError(p.migrateValueGreenland(ctx, sm))
			default:
				r.Equal(v.lastEpoch, a.foundationBonusLastEpoch)
				r.True(a.grantFoundationBonus(v.lastEpoch))
				r.False(a.grantFoundationBonus(v.lastEpoch + 1))
				a.foundationBonusLastEpoch = a1.foundationBonusLastEpoch
				r.Equal(a1, a)
				a.foundationBonusLastEpoch = v.lastEpoch
				// test migrate with no data
				r.NoError(p.migrateValueGreenland(ctx, sm))
			}
		}
	}, true)
}
