// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package poll

import (
	"context"
	"encoding/hex"
	"math/big"
	"time"

	"github.com/iotexproject/iotex-election/committee"
	"github.com/iotexproject/iotex-election/types"
	"github.com/iotexproject/iotex-election/util"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/prometheustimer"
	"github.com/iotexproject/iotex-core/state"
)

type stakingCommittee struct {
	hu                   config.HeightUpgrade
	getTipBlockTime      GetTipBlockTime
	getEpochHeight       GetEpochHeight
	getEpochNum          GetEpochNum
	electionCommittee    committee.Committee
	governanceStaking    Protocol
	nativeStaking        *NativeStaking
	rp                   *rolldpos.Protocol
	scoreThreshold       *big.Int
	currentNativeBuckets []*types.Bucket
	timerFactory         *prometheustimer.TimerFactory
}

// NewStakingCommittee creates a staking committee which fetch result from governance chain and native staking
func NewStakingCommittee(
	hu config.HeightUpgrade,
	ec committee.Committee,
	gs Protocol,
	cm protocol.ChainManager,
	getTipBlockTime GetTipBlockTime,
	getEpochHeight GetEpochHeight,
	getEpochNum GetEpochNum,
	nativeStakingContractAddress string,
	nativeStakingContractCode string,
	rp *rolldpos.Protocol,
	scoreThreshold *big.Int,
) (Protocol, error) {
	if getEpochHeight == nil {
		return nil, errors.New("failed to create native staking: empty getEpochHeight")
	}
	if getEpochNum == nil {
		return nil, errors.New("failed to create native staking: empty getEpochNum")
	}
	var ns *NativeStaking
	if nativeStakingContractAddress != "" || nativeStakingContractCode != "" {
		var err error
		if ns, err = NewNativeStaking(cm, getTipBlockTime); err != nil {
			return nil, errors.New("failed to create native staking")
		}
		if nativeStakingContractAddress != "" {
			ns.SetContract(nativeStakingContractAddress)
		}
	}

	timerFactory, err := prometheustimer.New(
		"iotex_staking_perf",
		"Performance of staking module",
		[]string{"type"},
		[]string{"default"},
	)
	if err != nil {
		return nil, err
	}

	sc := stakingCommittee{
		hu:                hu,
		electionCommittee: ec,
		governanceStaking: gs,
		nativeStaking:     ns,
		getTipBlockTime:   getTipBlockTime,
		getEpochHeight:    getEpochHeight,
		getEpochNum:       getEpochNum,
		rp:                rp,
		scoreThreshold:    scoreThreshold,
	}
	sc.timerFactory = timerFactory

	return &sc, nil
}

func (sc *stakingCommittee) Initialize(ctx context.Context, sm protocol.StateManager) error {
	return sc.governanceStaking.Initialize(ctx, sm)
}

func (sc *stakingCommittee) Handle(ctx context.Context, act action.Action, sm protocol.StateManager) (*action.Receipt, error) {
	receipt, err := sc.governanceStaking.Handle(ctx, act, sm)
	if err := sc.persistNativeBuckets(ctx, receipt, err); err != nil {
		return nil, err
	}
	return receipt, err
}

func (sc *stakingCommittee) Validate(ctx context.Context, act action.Action) error {
	return validate(ctx, sc, act)
}

func (sc *stakingCommittee) DelegatesByHeight(height uint64) (state.CandidateList, error) {
	timer := sc.timerFactory.NewTimer("Governance")
	cand, err := sc.governanceStaking.DelegatesByHeight(height)
	timer.End()
	if err != nil {
		return nil, err
	}
	// convert to epoch start height
	epochHeight := sc.getEpochHeight(sc.getEpochNum(height))
	if sc.hu.IsPre(config.Cook, epochHeight) {
		return sc.filterDelegates(cand), nil
	}
	// native staking starts from Cook
	if sc.nativeStaking == nil {
		return nil, errors.New("native staking was not set after cook height")
	}

	timer = sc.timerFactory.NewTimer("Native")
	nativeVotes, ts, err := sc.nativeStaking.Votes(height, sc.hu.IsPost(config.Daytona, height))
	timer.End()
	if err == ErrNoData {
		// no native staking data
		return sc.filterDelegates(cand), nil
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to get native chain candidates")
	}
	sc.currentNativeBuckets = nativeVotes.Buckets
	return sc.mergeDelegates(cand, nativeVotes, ts), nil
}

func (sc *stakingCommittee) ReadState(ctx context.Context, sm protocol.StateManager, method []byte, args ...[]byte) ([]byte, error) {
	return sc.governanceStaking.ReadState(ctx, sm, method, args...)
}

// SetNativeStakingContract sets the address of native staking contract
func (sc *stakingCommittee) SetNativeStakingContract(contract string) {
	sc.nativeStaking.SetContract(contract)
}

// return candidates whose votes are above threshold
func (sc *stakingCommittee) filterDelegates(candidates state.CandidateList) state.CandidateList {
	var cand state.CandidateList
	for _, c := range candidates {
		if c.Votes.Cmp(sc.scoreThreshold) >= 0 {
			cand = append(cand, c)
		}
	}
	return cand
}

func (sc *stakingCommittee) mergeDelegates(list state.CandidateList, votes *VoteTally, ts time.Time) state.CandidateList {
	// as of now, native staking does not have register contract, only voting/staking contract
	// it is assumed that all votes done on native staking target for delegates registered on Ethereum
	// votes cast to all outside address will not be counted and simply ignored
	candidates := make(map[string]*state.Candidate)
	candidateScores := make(map[string]*big.Int)
	for _, cand := range list {
		clone := cand.Clone()
		name := to12Bytes(clone.CanName)
		if v, ok := votes.Candidates[name]; ok {
			clone.Votes.Add(clone.Votes, v.Votes)
		}
		if clone.Votes.Cmp(sc.scoreThreshold) >= 0 {
			candidates[hex.EncodeToString(name[:])] = clone
			candidateScores[hex.EncodeToString(name[:])] = clone.Votes
		}
	}
	sorted := util.Sort(candidateScores, uint64(ts.Unix()))
	var merged state.CandidateList
	for _, name := range sorted {
		merged = append(merged, candidates[name])
	}
	return merged
}

func (sc *stakingCommittee) persistNativeBuckets(ctx context.Context, receipt *action.Receipt, err error) error {
	// Start to write native buckets archive after cook and only when the action is executed successfully
	raCtx := protocol.MustGetRunActionsCtx(ctx)
	epochHeight := sc.getEpochHeight(sc.getEpochNum(raCtx.BlockHeight))
	if sc.hu.IsPre(config.Cook, epochHeight) {
		return nil
	}
	if err != nil {
		return nil
	}
	if receipt == nil || receipt.Status != uint64(iotextypes.ReceiptStatus_Success) {
		return nil
	}
	log.L().Info("Store native buckets to election db", zap.Int("size", len(sc.currentNativeBuckets)))
	ts, err := sc.getTipBlockTime()
	if err != nil {
		return err
	}
	if err := sc.electionCommittee.PutNativePollByEpoch(
		sc.rp.GetEpochNum(raCtx.BlockHeight)+1, // The native buckets recorded in this epoch will be used in next one
		ts,                                     // The timestamp of last block is used to represent the current buckets timestamp
		sc.currentNativeBuckets,
	); err != nil {
		return err
	}
	sc.currentNativeBuckets = nil
	return nil
}
