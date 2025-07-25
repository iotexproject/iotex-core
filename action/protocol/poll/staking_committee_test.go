// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package poll

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-election/test/mock/mock_committee"
	"github.com/iotexproject/iotex-election/types"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/v2/action/protocol/vote/candidatesutil"
	"github.com/iotexproject/iotex-core/v2/blockchain"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/db/batch"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_chainmanager"
)

// TODO: we need something like mock_nativestaking to test properly with native buckets
// now in the unit tests, native bucket is empty
func initConstructStakingCommittee(ctrl *gomock.Controller) (Protocol, context.Context, protocol.StateManager, *types.ElectionResult, error) {
	cfg := struct {
		Genesis genesis.Genesis
		Chain   blockchain.Config
	}{
		Genesis: genesis.TestDefault(),
		Chain:   blockchain.DefaultConfig,
	}
	cfg.Genesis.NativeStakingContractAddress = "io1xpq62aw85uqzrccg9y5hnryv8ld2nkpycc3gza"
	producer := identityset.Address(0)
	ctx := protocol.WithBlockCtx(
		context.Background(),
		protocol.BlockCtx{
			BlockHeight: 0,
			Producer:    producer,
		},
	)
	registry := protocol.NewRegistry()
	err := registry.Register("rolldpos", rolldpos.NewProtocol(36, 36, 20))
	if err != nil {
		return nil, nil, nil, nil, err
	}
	ctx = genesis.WithGenesisContext(
		protocol.WithRegistry(ctx, registry),
		genesis.TestDefault(),
	)
	ctx = protocol.WithBlockchainCtx(ctx, protocol.BlockchainCtx{})
	ctx = protocol.WithActionCtx(
		ctx,
		protocol.ActionCtx{
			Caller: producer,
		},
	)
	ctx = protocol.WithFeatureCtx(ctx)

	sm := mock_chainmanager.NewMockStateManager(ctrl)
	committee := mock_committee.NewMockCommittee(ctrl)
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
	sm.EXPECT().Height().Return(uint64(123456), nil).AnyTimes()
	sm.EXPECT().Snapshot().Return(1).AnyTimes()
	r := types.NewElectionResultForTest(time.Now())
	committee.EXPECT().ResultByHeight(uint64(123456)).Return(r, nil).AnyTimes()
	committee.EXPECT().HeightByTime(gomock.Any()).Return(uint64(123456), nil).AnyTimes()
	slasher, err := NewSlasher(
		func(uint64, uint64) (map[string]uint64, error) {
			return nil, nil
		},
		func(protocol.StateReader, uint64, bool, bool) ([]*state.Candidate, uint64, error) {
			return nil, 0, state.ErrStateNotExist
		},
		nil,
		nil,
		nil,
		cfg.Genesis.NumCandidateDelegates,
		cfg.Genesis.NumDelegates,
		cfg.Genesis.ProductivityThreshold,
		cfg.Genesis.ProbationEpochPeriod,
		cfg.Genesis.UnproductiveDelegateMaxCacheSize,
		cfg.Genesis.ProbationIntensityRate)
	gs, err := NewGovernanceChainCommitteeProtocol(
		nil,
		committee,
		uint64(123456),
		func(uint64) (time.Time, error) { return time.Now(), nil },
		cfg.Chain.PollInitialCandidatesInterval,
		slasher,
	)
	scoreThreshold, ok := new(big.Int).SetString("0", 10)
	if !ok {
		return nil, nil, nil, nil, errors.Errorf("failed to parse score threshold")
	}

	p, err := NewStakingCommittee(
		committee,
		gs,
		func(context.Context, string, []byte, bool) ([]byte, error) {
			return nil, nil
		},
		cfg.Genesis.NativeStakingContractAddress,
		cfg.Genesis.NativeStakingContractCode,
		scoreThreshold,
	)

	return p, ctx, sm, r, err
}

func TestCreateGenesisStates_StakingCommittee(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	p, ctx, sm, r, err := initConstructStakingCommittee(ctrl)
	require.NoError(err)
	ctx = protocol.WithFeatureWithHeightCtx(ctx)
	require.NoError(p.CreateGenesisStates(ctx, sm))
	var candlist state.CandidateList
	_, err = sm.State(&candlist, protocol.LegacyKeyOption(candidatesutil.ConstructLegacyKey(1)))
	require.NoError(err)
	candidates, err := state.CandidatesToMap(candlist)
	require.NoError(err)
	require.Equal(2, len(candidates))
	for _, d := range r.Delegates() {
		operator := string(d.OperatorAddress())
		addr, err := address.FromString(operator)
		require.NoError(err)
		c, ok := candidates[hash.BytesToHash160(addr.Bytes())]
		require.True(ok)
		require.Equal(addr.String(), c.Address)
	}
	sc, ok := p.(*stakingCommittee)
	require.True(ok)
	require.NotNil(sc.nativeStaking.contract)
}

func TestCreatePostSystemActions_StakingCommittee(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	p, ctx, sr, r, err := initConstructStakingCommittee(ctrl)
	require.NoError(err)
	psac, ok := p.(protocol.PostSystemActionsCreator)
	require.True(ok)
	ctx = protocol.WithFeatureWithHeightCtx(ctx)
	ctx = protocol.WithBlockchainCtx(ctx, protocol.BlockchainCtx{
		GetBlockHash: func(uint64) (hash.Hash256, error) {
			return hash.ZeroHash256, nil
		},
		GetBlockTime: func(uint64) (time.Time, error) {
			return time.Now(), nil
		},
	})
	elp, err := psac.CreatePostSystemActions(ctx, sr)
	require.NoError(err)
	require.Equal(1, len(elp))
	act, ok := elp[0].Action().(*action.PutPollResult)
	require.True(ok)
	require.Equal(uint64(1), act.Height())
	require.Equal(uint64(0), elp[0].Nonce())
	delegates := r.Delegates()
	require.Equal(len(act.Candidates()), len(delegates))
	for _, can := range act.Candidates() {
		d := r.DelegateByName(can.CanName)
		require.NotNil(d)
	}
}

func TestHandle_StakingCommittee(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	p, ctx, sm, _, err := initConstructStakingCommittee(ctrl)
	require.NoError(err)
	ctx = protocol.WithFeatureWithHeightCtx(ctx)
	require.NoError(p.CreateGenesisStates(ctx, sm))

	// wrong action
	recipientAddr := identityset.Address(28)
	senderKey := identityset.PrivateKey(27)
	t.Run("Wrong action type", func(t *testing.T) {
		tsf := action.NewTransfer(big.NewInt(10), recipientAddr.String(), []byte{})
		bd := &action.EnvelopeBuilder{}
		elp := bd.SetGasLimit(100000).SetGasPrice(big.NewInt(10)).
			SetAction(tsf).Build()
		receipt, err := p.Handle(ctx, elp, nil)
		require.NoError(err)
		require.Nil(receipt)
	})
	t.Run("All right", func(t *testing.T) {
		p2, ctx2, sm2, _, err := initConstructStakingCommittee(ctrl)
		require.NoError(err)
		ctx2 = protocol.WithFeatureWithHeightCtx(ctx2)
		require.NoError(p2.CreateGenesisStates(ctx2, sm2))
		var sc2 state.CandidateList
		_, err = sm2.State(&sc2, protocol.LegacyKeyOption(candidatesutil.ConstructLegacyKey(1)))
		require.NoError(err)
		act2 := action.NewPutPollResult(1, sc2)
		elp := (&action.EnvelopeBuilder{}).SetNonce(1).SetGasLimit(uint64(100000)).
			SetGasPrice(big.NewInt(10)).SetAction(act2).Build()
		receipt, err := p.Handle(ctx2, elp, sm2)
		require.NoError(err)
		require.NotNil(receipt)

		candidates, _, err := candidatesutil.CandidatesFromDB(sm2, 1, true, false)
		require.NoError(err)
		require.Equal(2, len(candidates))
		require.Equal(candidates[0].Address, sc2[0].Address)
		require.Equal(candidates[0].Votes, sc2[0].Votes)
		require.Equal(candidates[1].Address, sc2[1].Address)
		require.Equal(candidates[1].Votes, sc2[1].Votes)
	})

	t.Run("Only producer could create this protocol", func(t *testing.T) {
		// Case 2: Only producer could create this protocol
		p2, ctx2, sm2, _, err := initConstructStakingCommittee(ctrl)
		require.NoError(err)
		ctx2 = protocol.WithFeatureWithHeightCtx(ctx2)
		require.NoError(p2.CreateGenesisStates(ctx2, sm2))
		var sc2 state.CandidateList
		_, err = sm2.State(&sc2, protocol.LegacyKeyOption(candidatesutil.ConstructLegacyKey(1)))
		require.NoError(err)
		act2 := action.NewPutPollResult(1, sc2)
		elp := (&action.EnvelopeBuilder{}).SetNonce(1).SetGasLimit(uint64(100000)).
			SetGasPrice(big.NewInt(10)).SetAction(act2).Build()
		caller := senderKey.PublicKey().Address()
		require.NotNil(caller)
		ctx2 = protocol.WithBlockCtx(
			ctx2,
			protocol.BlockCtx{
				BlockHeight: 1,
				Producer:    recipientAddr,
			},
		)
		ctx2 = protocol.WithActionCtx(
			ctx2,
			protocol.ActionCtx{
				Caller: caller,
			},
		)
		err = p.Validate(ctx2, elp, sm2)
		require.Contains(err.Error(), "Only producer could create this protocol")
	})

	t.Run("Duplicate candidate", func(t *testing.T) {
		p3, ctx3, sm3, _, err := initConstructStakingCommittee(ctrl)
		require.NoError(err)
		ctx3 = protocol.WithFeatureWithHeightCtx(ctx3)
		require.NoError(p3.CreateGenesisStates(ctx3, sm3))
		var sc3 state.CandidateList
		_, err = sm3.State(&sc3, protocol.LegacyKeyOption(candidatesutil.ConstructLegacyKey(1)))
		require.NoError(err)
		sc3 = append(sc3, &state.Candidate{Address: "1", Votes: big.NewInt(10), RewardAddress: "2", CanName: nil})
		sc3 = append(sc3, &state.Candidate{Address: "1", Votes: big.NewInt(10), RewardAddress: "2", CanName: nil})
		act3 := action.NewPutPollResult(1, sc3)
		elp := (&action.EnvelopeBuilder{}).SetNonce(1).SetGasLimit(uint64(100000)).
			SetGasPrice(big.NewInt(10)).SetAction(act3).Build()
		caller := senderKey.PublicKey().Address()
		require.NotNil(caller)
		ctx3 = protocol.WithBlockCtx(
			ctx3,
			protocol.BlockCtx{
				BlockHeight: 1,
				Producer:    identityset.Address(27),
			},
		)
		ctx3 = protocol.WithActionCtx(
			ctx3,
			protocol.ActionCtx{
				Caller: caller,
			},
		)
		err = p.Validate(ctx3, elp, sm3)
		require.Contains(err.Error(), "duplicate candidate")
	})

	t.Run("Delegate's length is not equal", func(t *testing.T) {
		p4, ctx4, sm4, _, err := initConstructStakingCommittee(ctrl)
		require.NoError(err)
		ctx4 = protocol.WithFeatureWithHeightCtx(ctx4)
		require.NoError(p4.CreateGenesisStates(ctx4, sm4))
		var sc4 state.CandidateList
		_, err = sm4.State(&sc4, protocol.LegacyKeyOption(candidatesutil.ConstructLegacyKey(1)))
		require.NoError(err)
		sc4 = append(sc4, &state.Candidate{Address: "1", Votes: big.NewInt(10), RewardAddress: "2", CanName: nil})
		act4 := action.NewPutPollResult(1, sc4)
		elp4 := (&action.EnvelopeBuilder{}).SetNonce(1).SetGasLimit(uint64(100000)).
			SetGasPrice(big.NewInt(10)).SetAction(act4).Build()
		caller := senderKey.PublicKey().Address()
		require.NotNil(caller)
		ctx4 = protocol.WithBlockCtx(
			ctx4,
			protocol.BlockCtx{
				BlockHeight: 1,
				Producer:    identityset.Address(27),
			},
		)
		ctx4 = protocol.WithActionCtx(
			ctx4,
			protocol.ActionCtx{
				Caller: caller,
			},
		)
		ctx4 = protocol.WithFeatureWithHeightCtx(ctx4)
		ctx4 = protocol.WithBlockchainCtx(ctx4, protocol.BlockchainCtx{
			GetBlockHash: func(uint64) (hash.Hash256, error) {
				return hash.ZeroHash256, nil
			},
			GetBlockTime: func(uint64) (time.Time, error) {
				return time.Now(), nil
			},
		})
		err = p4.Validate(ctx4, elp4, sm4)
		require.Contains(err.Error(), "the proposed delegate list length")
	})

	t.Run("Candidate's vote is not equal", func(t *testing.T) {
		p5, ctx5, sm5, _, err := initConstructStakingCommittee(ctrl)
		require.NoError(err)
		ctx5 = protocol.WithFeatureWithHeightCtx(ctx5)
		require.NoError(p5.CreateGenesisStates(ctx5, sm5))
		var sc5 state.CandidateList
		_, err = sm5.State(&sc5, protocol.LegacyKeyOption(candidatesutil.ConstructLegacyKey(1)))
		sc5[0].Votes = big.NewInt(10)
		act5 := action.NewPutPollResult(1, sc5)
		elp5 := (&action.EnvelopeBuilder{}).SetNonce(1).SetGasLimit(uint64(100000)).
			SetGasPrice(big.NewInt(10)).SetAction(act5).Build()
		caller := senderKey.PublicKey().Address()
		require.NotNil(caller)
		ctx5 = protocol.WithBlockCtx(
			ctx5,
			protocol.BlockCtx{
				BlockHeight: 1,
				Producer:    identityset.Address(27),
			},
		)
		ctx5 = protocol.WithActionCtx(
			ctx5,
			protocol.ActionCtx{
				Caller: caller,
			},
		)
		ctx5 = protocol.WithBlockchainCtx(ctx5, protocol.BlockchainCtx{
			GetBlockHash: func(uint64) (hash.Hash256, error) {
				return hash.ZeroHash256, nil
			},
			GetBlockTime: func(uint64) (time.Time, error) {
				return time.Now(), nil
			},
		})
		err = p5.Validate(ctx5, elp5, sm5)
		require.Contains(err.Error(), "delegates are not as expected")
	})
}
