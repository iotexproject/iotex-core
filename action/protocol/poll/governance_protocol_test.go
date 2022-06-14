// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package poll

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-election/test/mock/mock_committee"
	"github.com/iotexproject/iotex-election/types"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/action/protocol/vote"
	"github.com/iotexproject/iotex-core/action/protocol/vote/candidatesutil"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/db/batch"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_chainmanager"
)

func initConstruct(ctrl *gomock.Controller) (Protocol, context.Context, protocol.StateManager, *types.ElectionResult, error) {
	cfg := config.Default
	cfg.Genesis.EasterBlockHeight = 1 // set up testing after Easter Height
	cfg.Genesis.ProbationIntensityRate = 90
	cfg.Genesis.ProbationEpochPeriod = 2
	cfg.Genesis.ProductivityThreshold = 75
	ctx := protocol.WithBlockCtx(
		context.Background(),
		protocol.BlockCtx{
			BlockHeight: 0,
		},
	)
	registry := protocol.NewRegistry()
	rp := rolldpos.NewProtocol(36, 6, 5)
	err := registry.Register("rolldpos", rp)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	epochStartHeight := rp.GetEpochHeight(2)
	ctx = genesis.WithGenesisContext(
		protocol.WithBlockchainCtx(
			protocol.WithRegistry(ctx, registry),
			protocol.BlockchainCtx{
				Tip: protocol.TipInfo{
					Height: epochStartHeight - 1,
				},
			},
		),
		cfg.Genesis,
	)
	ctx = protocol.WithActionCtx(
		ctx,
		protocol.ActionCtx{},
	)

	sm := mock_chainmanager.NewMockStateManager(ctrl)
	committee := mock_committee.NewMockCommittee(ctrl)
	cb := batch.NewCachedBatch()
	sm.EXPECT().State(gomock.Any(), gomock.Any()).DoAndReturn(
		func(account interface{}, opts ...protocol.StateOption) (uint64, error) {
			cfg, err := protocol.CreateStateConfig(opts...)
			if err != nil {
				return 0, err
			}
			val, err := cb.Get(cfg.Namespace, cfg.Key)
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
			cb.Put(cfg.Namespace, cfg.Key, ss, "failed to put state")
			return 0, nil
		}).AnyTimes()
	sm.EXPECT().DelState(gomock.Any()).DoAndReturn(
		func(opts ...protocol.StateOption) (uint64, error) {
			cfg, err := protocol.CreateStateConfig(opts...)
			if err != nil {
				return 0, err
			}
			cb.Delete(cfg.Namespace, cfg.Key, "failed to delete state")
			return 0, nil
		}).AnyTimes()
	sm.EXPECT().Snapshot().Return(1).AnyTimes()
	sm.EXPECT().Height().Return(epochStartHeight-1, nil).AnyTimes()
	r := types.NewElectionResultForTest(time.Now())
	committee.EXPECT().ResultByHeight(uint64(123456)).Return(r, nil).AnyTimes()
	committee.EXPECT().HeightByTime(gomock.Any()).Return(uint64(123456), nil).AnyTimes()
	candidates := []*state.Candidate{
		{
			Address:       identityset.Address(1).String(),
			Votes:         big.NewInt(30),
			RewardAddress: "rewardAddress1",
		},
		{
			Address:       identityset.Address(2).String(),
			Votes:         big.NewInt(22),
			RewardAddress: "rewardAddress2",
		},
		{
			Address:       identityset.Address(3).String(),
			Votes:         big.NewInt(20),
			RewardAddress: "rewardAddress3",
		},
		{
			Address:       identityset.Address(4).String(),
			Votes:         big.NewInt(10),
			RewardAddress: "rewardAddress4",
		},
		{
			Address:       identityset.Address(5).String(),
			Votes:         big.NewInt(5),
			RewardAddress: "rewardAddress5",
		},
		{
			Address:       identityset.Address(6).String(),
			Votes:         big.NewInt(3),
			RewardAddress: "rewardAddress6",
		},
	}
	indexer, err := NewCandidateIndexer(db.NewMemKVStore())
	if err != nil {
		return nil, nil, nil, nil, err
	}
	slasher, err := NewSlasher(
		func(start, end uint64) (map[string]uint64, error) {
			switch start {
			case 1:
				return map[string]uint64{ // [A, B, C]
					identityset.Address(1).String(): 1, // underperformance
					identityset.Address(2).String(): 1, // underperformance
					identityset.Address(3).String(): 1, // underperformance
					identityset.Address(4).String(): 5,
					identityset.Address(5).String(): 5,
					identityset.Address(6).String(): 5,
				}, nil
			case 31:
				return map[string]uint64{ // [B, D]
					identityset.Address(1).String(): 5,
					identityset.Address(2).String(): 1, // underperformance
					identityset.Address(3).String(): 5,
					identityset.Address(4).String(): 1, // underperformance
					identityset.Address(5).String(): 4,
					identityset.Address(6).String(): 4,
				}, nil
			case 61:
				return map[string]uint64{ // [E, F]
					identityset.Address(1).String(): 5,
					identityset.Address(2).String(): 5,
					identityset.Address(3).String(): 5,
					identityset.Address(4).String(): 5,
					identityset.Address(5).String(): 1, // underperformance
					identityset.Address(6).String(): 1, // underperformance
				}, nil
			default:
				return nil, nil
			}
		},
		candidatesutil.CandidatesFromDB,
		candidatesutil.ProbationListFromDB,
		candidatesutil.UnproductiveDelegateFromDB,
		indexer,
		2,
		2,
		cfg.Genesis.DardanellesNumSubEpochs,
		cfg.Genesis.ProductivityThreshold,
		cfg.Genesis.ProbationEpochPeriod,
		cfg.Genesis.UnproductiveDelegateMaxCacheSize,
		cfg.Genesis.ProbationIntensityRate)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	p, err := NewGovernanceChainCommitteeProtocol(
		indexer,
		committee,
		uint64(123456),
		func(uint64) (time.Time, error) { return time.Now(), nil },
		cfg.Chain.PollInitialCandidatesInterval,
		slasher)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	ctx = protocol.WithFeatureWithHeightCtx(ctx)
	if err := setCandidates(ctx, sm, indexer, candidates, 1); err != nil {
		return nil, nil, nil, nil, err
	}
	if err := setNextEpochProbationList(sm, indexer, 1, vote.NewProbationList(cfg.Genesis.ProbationIntensityRate)); err != nil {
		return nil, nil, nil, nil, err
	}
	return p, ctx, sm, r, err
}

func TestCreateGenesisStates(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	p, ctx, sm, r, err := initConstruct(ctrl)
	require.NoError(err)
	require.NoError(p.CreateGenesisStates(ctx, sm))
	var sc state.CandidateList
	candKey := candidatesutil.ConstructKey(candidatesutil.NxtCandidateKey)
	_, err = sm.State(&sc, protocol.KeyOption(candKey[:]), protocol.NamespaceOption(protocol.SystemNamespace))
	require.NoError(err)
	candidates, err := state.CandidatesToMap(sc)
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
}

func TestCreatePostSystemActions(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	p, ctx, sm, r, err := initConstruct(ctrl)
	require.NoError(err)
	_, err = shiftCandidates(sm)
	require.NoError(err)
	psac, ok := p.(protocol.PostSystemActionsCreator)
	require.True(ok)
	elp, err := psac.CreatePostSystemActions(ctx, sm)
	require.NoError(err)
	require.Equal(1, len(elp))
	act, ok := elp[0].Action().(*action.PutPollResult)
	require.True(ok)
	require.Equal(uint64(1), act.Height())
	require.Equal(uint64(0), act.AbstractAction.Nonce())
	delegates := r.Delegates()
	require.Equal(len(act.Candidates()), len(delegates))
	for _, can := range act.Candidates() {
		d := r.DelegateByName(can.CanName)
		require.NotNil(d)
	}
}

func TestCreatePreStates(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	p, ctx, sm, _, err := initConstruct(ctrl)
	require.NoError(err)

	psc, ok := p.(protocol.PreStatesCreator)
	require.True(ok)
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	rp := rolldpos.MustGetProtocol(protocol.MustGetRegistry(ctx))

	test := make(map[uint64](map[string]uint32))
	test[1] = map[string]uint32{}
	test[2] = map[string]uint32{
		identityset.Address(1).String(): 1, // [A, B, C]
		identityset.Address(2).String(): 1,
		identityset.Address(3).String(): 1,
	}
	test[3] = map[string]uint32{
		identityset.Address(1).String(): 1, // [A, B, C, D]
		identityset.Address(2).String(): 2,
		identityset.Address(3).String(): 1,
		identityset.Address(4).String(): 1,
	}
	test[4] = map[string]uint32{
		identityset.Address(2).String(): 1, // [B, D, E, F]
		identityset.Address(4).String(): 1,
		identityset.Address(5).String(): 1,
		identityset.Address(6).String(): 1,
	}

	// testing for probation slashing
	var epochNum uint64
	for epochNum = 1; epochNum <= 3; epochNum++ {
		// at first of epoch
		epochStartHeight := rp.GetEpochHeight(epochNum)
		bcCtx.Tip.Height = epochStartHeight - 1
		ctx = protocol.WithBlockchainCtx(ctx, bcCtx)
		ctx = protocol.WithBlockCtx(
			ctx,
			protocol.BlockCtx{
				BlockHeight: epochStartHeight,
				Producer:    identityset.Address(1),
			},
		)
		ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
		require.NoError(psc.CreatePreStates(ctx, sm)) // shift
		bl := &vote.ProbationList{}
		key := candidatesutil.ConstructKey(candidatesutil.CurProbationKey)
		_, err := sm.State(bl, protocol.KeyOption(key[:]), protocol.NamespaceOption(protocol.SystemNamespace))
		require.NoError(err)
		expected := test[epochNum]
		require.Equal(len(expected), len(bl.ProbationInfo))
		for addr, count := range bl.ProbationInfo {
			val, ok := expected[addr]
			require.True(ok)
			require.Equal(val, count)
		}

		// mid of epoch, set candidatelist into next candidate key
		nextEpochStartHeight := rp.GetEpochHeight(epochNum + 1)
		candidates, err := p.Candidates(ctx, sm)
		require.Equal(len(candidates), 6)
		require.NoError(err)
		require.NoError(setCandidates(ctx, sm, nil, candidates, nextEpochStartHeight)) // set next candidate

		// at last of epoch, set probationList into next probation key
		epochLastHeight := rp.GetEpochLastBlockHeight(epochNum)
		bcCtx.Tip.Height = epochLastHeight - 1
		ctx = protocol.WithBlockchainCtx(ctx, bcCtx)
		ctx = protocol.WithBlockCtx(
			ctx,
			protocol.BlockCtx{
				BlockHeight: epochLastHeight,
				Producer:    identityset.Address(1),
			},
		)
		require.NoError(psc.CreatePreStates(ctx, sm)) // calculate probation list and set next probationlist

		bl = &vote.ProbationList{}
		key = candidatesutil.ConstructKey(candidatesutil.NxtProbationKey)
		_, err = sm.State(bl, protocol.KeyOption(key[:]), protocol.NamespaceOption(protocol.SystemNamespace))
		require.NoError(err)
		expected = test[epochNum+1]
		require.Equal(len(expected), len(bl.ProbationInfo))
		for addr, count := range bl.ProbationInfo {
			val, ok := expected[addr]
			require.True(ok)
			require.Equal(val, count)
		}
	}
}

func TestHandle(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	p, ctx, sm, _, err := initConstruct(ctrl)
	require.NoError(err)
	require.NoError(p.CreateGenesisStates(ctx, sm))
	recipientAddr := identityset.Address(28)
	senderKey := identityset.PrivateKey(27)

	t.Run("wrong action", func(t *testing.T) {
		tsf, err := action.NewTransfer(0, big.NewInt(10), recipientAddr.String(), []byte{}, uint64(100000), big.NewInt(10))
		require.NoError(err)
		bd := &action.EnvelopeBuilder{}
		elp := bd.SetGasLimit(uint64(100000)).
			SetGasPrice(big.NewInt(10)).
			SetAction(tsf).Build()
		selp, err := action.Sign(elp, senderKey)
		require.NoError(err)
		require.NotNil(selp)
		receipt, err := p.Handle(ctx, selp.Action(), nil)
		require.NoError(err)
		require.Nil(receipt)
	})
	candKey := candidatesutil.ConstructKey(candidatesutil.NxtCandidateKey)
	t.Run("All right", func(t *testing.T) {
		p2, ctx2, sm2, _, err := initConstruct(ctrl)
		require.NoError(err)
		require.NoError(p2.CreateGenesisStates(ctx2, sm2))
		var sc2 state.CandidateList
		_, err = sm2.State(&sc2, protocol.KeyOption(candKey[:]), protocol.NamespaceOption(protocol.SystemNamespace))
		require.NoError(err)
		act2 := action.NewPutPollResult(1, 1, sc2)
		bd := &action.EnvelopeBuilder{}
		elp := bd.SetGasLimit(uint64(100000)).
			SetGasPrice(big.NewInt(10)).
			SetAction(act2).Build()
		selp2, err := action.Sign(elp, senderKey)
		require.NoError(err)
		require.NotNil(selp2)
		caller := selp2.SenderAddress()
		require.NotNil(caller)
		ctx2 = protocol.WithBlockCtx(
			ctx2,
			protocol.BlockCtx{
				BlockHeight: 1,
				Producer:    caller,
			},
		)
		ctx2 = protocol.WithActionCtx(
			ctx2,
			protocol.ActionCtx{
				Caller: caller,
			},
		)
		receipt, err := p.Handle(ctx2, selp2.Action(), sm2)
		require.NoError(err)
		require.NotNil(receipt)

		_, err = shiftCandidates(sm2)
		require.NoError(err)
		_, _, err = candidatesutil.CandidatesFromDB(sm2, 1, false, true)
		require.Error(err) // should return stateNotExist error
		candidates, _, err := candidatesutil.CandidatesFromDB(sm2, 1, false, false)
		require.NoError(err)
		require.Equal(2, len(candidates))
		require.Equal(candidates[0].Address, sc2[0].Address)
		require.Equal(candidates[0].Votes, sc2[0].Votes)
		require.Equal(candidates[1].Address, sc2[1].Address)
		require.Equal(candidates[1].Votes, sc2[1].Votes)
	})

	t.Run("Only producer could create this protocol", func(t *testing.T) {
		p2, ctx2, sm2, _, err := initConstruct(ctrl)
		require.NoError(err)
		require.NoError(p2.CreateGenesisStates(ctx2, sm2))
		var sc2 state.CandidateList
		_, err = sm2.State(&sc2, protocol.KeyOption(candKey[:]), protocol.NamespaceOption(protocol.SystemNamespace))
		require.NoError(err)
		act2 := action.NewPutPollResult(1, 1, sc2)
		bd := &action.EnvelopeBuilder{}
		elp := bd.SetGasLimit(uint64(100000)).
			SetGasPrice(big.NewInt(10)).
			SetAction(act2).Build()
		selp2, err := action.Sign(elp, senderKey)
		require.NoError(err)
		require.NotNil(selp2)
		caller := selp2.SenderAddress()
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
		err = p.Validate(ctx2, selp2.Action(), sm2)
		require.Contains(err.Error(), "Only producer could create this protocol")
	})
	t.Run("Duplicate candidate", func(t *testing.T) {
		p3, ctx3, sm3, _, err := initConstruct(ctrl)
		require.NoError(err)
		require.NoError(p3.CreateGenesisStates(ctx3, sm3))
		var sc3 state.CandidateList
		_, err = sm3.State(&sc3, protocol.KeyOption(candKey[:]), protocol.NamespaceOption(protocol.SystemNamespace))
		require.NoError(err)
		sc3 = append(sc3, &state.Candidate{"1", big.NewInt(10), "2", nil})
		sc3 = append(sc3, &state.Candidate{"1", big.NewInt(10), "2", nil})
		act3 := action.NewPutPollResult(1, 1, sc3)
		bd := &action.EnvelopeBuilder{}
		elp := bd.SetGasLimit(uint64(100000)).
			SetGasPrice(big.NewInt(10)).
			SetAction(act3).Build()
		selp3, err := action.Sign(elp, senderKey)
		require.NoError(err)
		require.NotNil(selp3)
		caller := selp3.SenderAddress()
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
		err = p.Validate(ctx3, selp3.Action(), sm3)
		require.Contains(err.Error(), "duplicate candidate")
	})
	t.Run("Delegate's length is not equal", func(t *testing.T) {
		p4, ctx4, sm4, _, err := initConstruct(ctrl)
		require.NoError(err)
		require.NoError(p4.CreateGenesisStates(ctx4, sm4))
		var sc4 state.CandidateList
		_, err = sm4.State(&sc4, protocol.KeyOption(candKey[:]), protocol.NamespaceOption(protocol.SystemNamespace))
		require.NoError(err)
		sc4 = append(sc4, &state.Candidate{"1", big.NewInt(10), "2", nil})
		act4 := action.NewPutPollResult(1, 1, sc4)
		bd4 := &action.EnvelopeBuilder{}
		elp4 := bd4.SetGasLimit(uint64(100000)).
			SetGasPrice(big.NewInt(10)).
			SetAction(act4).Build()
		selp4, err := action.Sign(elp4, senderKey)
		require.NoError(err)
		require.NotNil(selp4)
		caller := selp4.SenderAddress()
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
		err = p4.Validate(ctx4, selp4.Action(), sm4)
		require.Contains(err.Error(), "the proposed delegate list length")
	})
	t.Run("Candidate's vote is not equal", func(t *testing.T) {
		p5, ctx5, sm5, _, err := initConstruct(ctrl)
		require.NoError(err)
		require.NoError(p5.CreateGenesisStates(ctx5, sm5))
		var sc5 state.CandidateList
		_, err = sm5.State(&sc5, protocol.KeyOption(candKey[:]), protocol.NamespaceOption(protocol.SystemNamespace))
		require.NoError(err)
		sc5[0].Votes = big.NewInt(10)
		act5 := action.NewPutPollResult(1, 1, sc5)
		bd5 := &action.EnvelopeBuilder{}
		elp5 := bd5.SetGasLimit(uint64(100000)).
			SetGasPrice(big.NewInt(10)).
			SetAction(act5).Build()
		selp5, err := action.Sign(elp5, senderKey)
		require.NoError(err)
		require.NotNil(selp5)
		caller := selp5.SenderAddress()
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
		err = p5.Validate(ctx5, selp5.Action(), sm5)
		require.Contains(err.Error(), "delegates are not as expected")
	})
}

func TestNextCandidates(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	p, ctx, sm, _, err := initConstruct(ctrl)
	require.NoError(err)
	probationListMap := map[string]uint32{
		identityset.Address(1).String(): 1,
		identityset.Address(2).String(): 1,
	}

	probationList := &vote.ProbationList{
		ProbationInfo: probationListMap,
		IntensityRate: 50,
	}
	require.NoError(setNextEpochProbationList(sm, nil, 721, probationList))
	ctx = protocol.WithFeatureWithHeightCtx(ctx)
	filteredCandidates, err := p.NextCandidates(ctx, sm)
	require.NoError(err)
	require.Equal(6, len(filteredCandidates))

	for _, cand := range filteredCandidates {
		if cand.Address == identityset.Address(1).String() {
			require.Equal(0, cand.Votes.Cmp(big.NewInt(15)))
		}
		if cand.Address == identityset.Address(2).String() {
			require.Equal(0, cand.Votes.Cmp(big.NewInt(11)))
		}
	}

	// change intensity rate to be 0
	probationList = &vote.ProbationList{
		ProbationInfo: probationListMap,
		IntensityRate: 0,
	}
	require.NoError(setNextEpochProbationList(sm, nil, 721, probationList))
	filteredCandidates, err = p.NextCandidates(ctx, sm)
	require.NoError(err)
	require.Equal(6, len(filteredCandidates))

	for _, cand := range filteredCandidates {
		if cand.Address == identityset.Address(1).String() {
			require.Equal(0, cand.Votes.Cmp(big.NewInt(30)))
		}
		if cand.Address == identityset.Address(2).String() {
			require.Equal(0, cand.Votes.Cmp(big.NewInt(22)))
		}
	}

}

func TestDelegatesAndNextDelegates(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	p, ctx, sm, _, err := initConstruct(ctrl)
	require.NoError(err)

	// 1: empty probationList NextDelegates()
	probationListMap := map[string]uint32{}
	probationList := &vote.ProbationList{
		ProbationInfo: probationListMap,
		IntensityRate: 90,
	}
	require.NoError(setNextEpochProbationList(sm, nil, 31, probationList))

	ctx = protocol.WithFeatureWithHeightCtx(ctx)
	delegates, err := p.NextDelegates(ctx, sm)
	require.NoError(err)
	require.Equal(2, len(delegates))
	require.Equal(identityset.Address(1).String(), delegates[0].Address)
	require.Equal(identityset.Address(2).String(), delegates[1].Address)

	// 2: not empty probationList NextDelegates()
	probationListMap2 := map[string]uint32{
		identityset.Address(1).String(): 1,
		identityset.Address(2).String(): 1,
	}
	probationList2 := &vote.ProbationList{
		ProbationInfo: probationListMap2,
		IntensityRate: 90,
	}
	require.NoError(setNextEpochProbationList(sm, nil, 721, probationList2))
	delegates2, err := p.NextDelegates(ctx, sm)
	require.NoError(err)
	require.Equal(2, len(delegates2))
	// even though the address 1, 2 have larger amount of votes, it got probated because it's on probation list
	require.Equal(identityset.Address(3).String(), delegates2[0].Address)
	require.Equal(identityset.Address(4).String(), delegates2[1].Address)

	// 3: probation with different probationList
	probationListMap3 := map[string]uint32{
		identityset.Address(1).String(): 1,
		identityset.Address(3).String(): 2,
	}
	probationList3 := &vote.ProbationList{
		ProbationInfo: probationListMap3,
		IntensityRate: 90,
	}
	require.NoError(setNextEpochProbationList(sm, nil, 721, probationList3))

	delegates3, err := p.NextDelegates(ctx, sm)
	require.NoError(err)

	require.Equal(2, len(delegates3))
	require.Equal(identityset.Address(2).String(), delegates3[0].Address)
	require.Equal(identityset.Address(4).String(), delegates3[1].Address)

	// 4: test hard probation
	probationListMap4 := map[string]uint32{
		identityset.Address(1).String(): 1,
		identityset.Address(2).String(): 2,
		identityset.Address(3).String(): 2,
		identityset.Address(4).String(): 2,
		identityset.Address(5).String(): 2,
	}
	probationList4 := &vote.ProbationList{
		ProbationInfo: probationListMap4,
		IntensityRate: 100, // hard probation
	}
	require.NoError(setNextEpochProbationList(sm, nil, 721, probationList4))

	delegates4, err := p.NextDelegates(ctx, sm)
	require.NoError(err)

	require.Equal(1, len(delegates4)) // exclude all of them
	require.Equal(identityset.Address(6).String(), delegates4[0].Address)

	// 5: shift probation list and Delegates()
	_, err = shiftCandidates(sm)
	require.NoError(err)
	_, err = shiftProbationList(sm)
	require.NoError(err)
	delegates5, err := p.Delegates(ctx, sm)
	require.NoError(err)
	require.Equal(len(delegates5), len(delegates4))
	for i, d := range delegates4 {
		require.True(d.Equal(delegates5[i]))
	}
}
