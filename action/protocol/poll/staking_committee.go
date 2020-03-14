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

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-election/committee"
	"github.com/iotexproject/iotex-election/types"
	"github.com/iotexproject/iotex-election/util"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/action/protocol/staking"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/prometheustimer"
	"github.com/iotexproject/iotex-core/state"
)

var nativeStakingContractCreator = address.ZeroAddress
var nativeStakingContractNonce = uint64(0)

type stakingCommittee struct {
	electionCommittee    committee.Committee
	governanceStaking    Protocol
	nativeStaking        *NativeStaking
	enableStakingV2      bool
	nativeStakingV2      *staking.Protocol
	scoreThreshold       *big.Int
	currentNativeBuckets []*types.Bucket
	timerFactory         *prometheustimer.TimerFactory
}

// NewStakingCommittee creates a staking committee which fetch result from governance chain and native staking
func NewStakingCommittee(
	ec committee.Committee,
	gs Protocol,
	enableV2 bool,
	stakingV2 *staking.Protocol,
	readContract ReadContract,
	nativeStakingContractAddress string,
	nativeStakingContractCode string,
	scoreThreshold *big.Int,
) (Protocol, error) {
	var ns *NativeStaking
	if nativeStakingContractAddress != "" || nativeStakingContractCode != "" {
		var err error
		if ns, err = NewNativeStaking(readContract); err != nil {
			return nil, errors.New("failed to create native staking")
		}
		if nativeStakingContractAddress != "" {
			ns.SetContract(nativeStakingContractAddress)
		}
	}

	if enableV2 && stakingV2 == nil {
		return nil, errors.New("native staking V2 protocol cannot be empty when enabled")
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
		electionCommittee: ec,
		governanceStaking: gs,
		nativeStaking:     ns,
		enableStakingV2:   enableV2,
		nativeStakingV2:   stakingV2,
		scoreThreshold:    scoreThreshold,
	}
	sc.timerFactory = timerFactory

	return &sc, nil
}

func (sc *stakingCommittee) CreateGenesisStates(ctx context.Context, sm protocol.StateManager) error {
	if gsc, ok := sc.governanceStaking.(protocol.GenesisStateCreator); ok {
		if err := gsc.CreateGenesisStates(ctx, sm); err != nil {
			return err
		}
	}
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)
	if blkCtx.BlockHeight != 0 {
		return errors.Errorf("Cannot create genesis state for height %d", blkCtx.BlockHeight)
	}
	if bcCtx.Genesis.NativeStakingContractCode == "" || bcCtx.Genesis.NativeStakingContractAddress != "" {
		return nil
	}
	blkCtx.Producer, _ = address.FromString(address.ZeroAddress)
	blkCtx.GasLimit = bcCtx.Genesis.BlockGasLimit
	bytes, err := hexutil.Decode(bcCtx.Genesis.NativeStakingContractCode)
	if err != nil {
		return err
	}
	execution, err := action.NewExecution(
		"",
		nativeStakingContractNonce,
		big.NewInt(0),
		bcCtx.Genesis.BlockGasLimit,
		big.NewInt(0),
		bytes,
	)
	if err != nil {
		return err
	}
	actionCtx := protocol.ActionCtx{}
	actionCtx.Caller, err = address.FromString(nativeStakingContractCreator)
	if err != nil {
		return err
	}
	actionCtx.Nonce = nativeStakingContractNonce
	actionCtx.ActionHash = execution.Hash()
	actionCtx.GasPrice = execution.GasPrice()
	actionCtx.IntrinsicGas, err = execution.IntrinsicGas()
	if err != nil {
		return err
	}
	ctx = protocol.WithActionCtx(ctx, actionCtx)
	ctx = protocol.WithBlockCtx(ctx, blkCtx)
	// deploy native staking contract
	_, receipt, err := evm.ExecuteContract(
		ctx,
		sm,
		execution,
		func(height uint64) (hash.Hash256, error) {
			return hash.ZeroHash256, nil
		},
	)
	if err != nil {
		return err
	}
	if receipt.Status != uint64(iotextypes.ReceiptStatus_Success) {
		return errors.Errorf("error when deploying native staking contract, status=%d", receipt.Status)
	}
	sc.SetNativeStakingContract(receipt.ContractAddress)
	log.L().Info("Deployed native staking contract", zap.String("address", receipt.ContractAddress))

	return nil
}

func (sc *stakingCommittee) Start(ctx context.Context) error {
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	if bcCtx.Genesis.NativeStakingContractAddress == "" && bcCtx.Genesis.NativeStakingContractCode != "" {
		caller, _ := address.FromString(nativeStakingContractCreator)
		ethAddr := crypto.CreateAddress(common.BytesToAddress(caller.Bytes()), nativeStakingContractNonce)
		iotxAddr, _ := address.FromBytes(ethAddr.Bytes())
		sc.SetNativeStakingContract(iotxAddr.String())
		log.L().Info("Loaded native staking contract", zap.String("address", iotxAddr.String()))
	}

	return nil
}

func (sc *stakingCommittee) CreatePreStates(ctx context.Context, sm protocol.StateManager) error {
	if psc, ok := sc.governanceStaking.(protocol.PreStatesCreator); ok {
		return psc.CreatePreStates(ctx, sm)
	}

	return nil
}

func (sc *stakingCommittee) CreatePostSystemActions(ctx context.Context) ([]action.Envelope, error) {
	return createPostSystemActions(ctx, sc)
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

// CalculateCandidatesByHeight calculates delegates with native staking and returns merged list
func (sc *stakingCommittee) CalculateCandidatesByHeight(ctx context.Context, height uint64) (state.CandidateList, error) {
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	hu := config.NewHeightUpgrade(&bcCtx.Genesis)
	timer := sc.timerFactory.NewTimer("Governance")
	var (
		cand state.CandidateList
		err  error
	)
	if !sc.enableStakingV2 || hu.IsPre(config.Fairbank, height) {
		cand, err = sc.governanceStaking.CalculateCandidatesByHeight(ctx, height)
	} else {
		// starting Fairbank height all candidates come from native staking V2
		cand, err = sc.nativeStakingV2.ActiveCandidates(ctx)
		timer.End()
		return cand, err
	}
	timer.End()
	if err != nil {
		return nil, err
	}

	// convert to epoch start height
	rp := rolldpos.MustGetProtocol(bcCtx.Registry)
	if hu.IsPre(config.Cook, rp.GetEpochHeight(rp.GetEpochNum(height))) {
		return sc.filterCandidates(cand), nil
	}
	// native staking using contract starts from Cook
	if sc.nativeStaking == nil {
		return nil, errors.New("native staking was not set after cook height")
	}

	// TODO: extract tip info inside of Votes function
	timer = sc.timerFactory.NewTimer("Native")
	nativeVotes, err := sc.nativeStaking.Votes(ctx, bcCtx.Tip.Timestamp, hu.IsPost(config.Daytona, height))
	timer.End()
	if err == ErrNoData {
		// no native staking data
		return sc.filterCandidates(cand), nil
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to get native chain candidates")
	}
	sc.currentNativeBuckets = nativeVotes.Buckets

	return sc.mergeCandidates(cand, nativeVotes, bcCtx.Tip.Timestamp), nil
}

// DelegatesByEpoch returns exact number of delegates according to epoch number
func (sc *stakingCommittee) DelegatesByEpoch(ctx context.Context, epochNum uint64) (state.CandidateList, error) {
	return sc.governanceStaking.DelegatesByEpoch(ctx, epochNum)
}

// CandidatesByHeight returns candidate list from state factory according to height
func (sc *stakingCommittee) CandidatesByHeight(ctx context.Context, height uint64) (state.CandidateList, error) {
	return sc.governanceStaking.CandidatesByHeight(ctx, height)
}

func (sc *stakingCommittee) ReadState(ctx context.Context, sr protocol.StateReader, method []byte, args ...[]byte) ([]byte, error) {
	return sc.governanceStaking.ReadState(ctx, sr, method, args...)
}

// Register registers the protocol with a unique ID
func (sc *stakingCommittee) Register(r *protocol.Registry) error {
	return r.Register(protocolID, sc)
}

// ForceRegister registers the protocol with a unique ID and force replacing the previous protocol if it exists
func (sc *stakingCommittee) ForceRegister(r *protocol.Registry) error {
	return r.ForceRegister(protocolID, sc)
}

// SetNativeStakingContract sets the address of native staking contract
func (sc *stakingCommittee) SetNativeStakingContract(contract string) {
	sc.nativeStaking.SetContract(contract)
}

// return candidates whose votes are above threshold
func (sc *stakingCommittee) filterCandidates(candidates state.CandidateList) state.CandidateList {
	var cand state.CandidateList
	for _, c := range candidates {
		if c.Votes.Cmp(sc.scoreThreshold) >= 0 {
			cand = append(cand, c)
		}
	}
	return cand
}

func (sc *stakingCommittee) mergeCandidates(list state.CandidateList, votes *VoteTally, ts time.Time) state.CandidateList {
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
	blkCtx := protocol.MustGetBlockCtx(ctx)
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	rp := rolldpos.MustGetProtocol(bcCtx.Registry)
	epochHeight := rp.GetEpochHeight(rp.GetEpochNum(blkCtx.BlockHeight))
	hu := config.NewHeightUpgrade(&bcCtx.Genesis)
	if hu.IsPre(config.Cook, epochHeight) {
		return nil
	}
	if receipt == nil || receipt.Status != uint64(iotextypes.ReceiptStatus_Success) {
		return nil
	}
	log.L().Info("Store native buckets to election db", zap.Int("size", len(sc.currentNativeBuckets)))
	if err := sc.electionCommittee.PutNativePollByEpoch(
		rp.GetEpochNum(blkCtx.BlockHeight)+1, // The native buckets recorded in this epoch will be used in next one
		bcCtx.Tip.Timestamp,                  // The timestamp of last block is used to represent the current buckets timestamp
		sc.currentNativeBuckets,
	); err != nil {
		return err
	}
	sc.currentNativeBuckets = nil
	return nil
}
