package poll

import (
	"context"
	"math/big"
	"time"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/staking"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-election/util"
)

type nativeStakingV2 struct {
	addr           address.Address
	stakingV2      *staking.Protocol
	candIndexer    *CandidateIndexer
	slasher        *Slasher
	scoreThreshold *big.Int
}

func newNativeStakingV2(
	candIndexer *CandidateIndexer,
	sh *Slasher,
	scoreThreshold *big.Int,
	stkV2 *staking.Protocol,
) (Protocol, error) {
	h := hash.Hash160b([]byte(_protocolID))
	addr, err := address.FromBytes(h[:])
	if err != nil {
		return nil, err
	}

	return &nativeStakingV2{
		addr:           addr,
		stakingV2:      stkV2,
		candIndexer:    candIndexer,
		slasher:        sh,
		scoreThreshold: scoreThreshold,
	}, nil
}

func (ns *nativeStakingV2) Start(ctx context.Context, sr protocol.StateReader) (interface{}, error) {
	return nil, nil
}

func (ns *nativeStakingV2) CreateGenesisStates(ctx context.Context, sm protocol.StateManager) error {
	if err := ns.slasher.CreateGenesisStates(ctx, sm, ns.candIndexer); err != nil {
		return err
	}
	cands, err := ns.stakingV2.ActiveCandidates(ctx, sm, 0)
	if err != nil {
		return err
	}
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	cands = ns.filterAndSortCandidatesByVoteScore(cands, bcCtx.Tip.Timestamp)
	return setCandidates(ctx, sm, ns.candIndexer, cands, uint64(1))
}

func (ns *nativeStakingV2) CreatePreStates(ctx context.Context, sm protocol.StateManager) error {
	return ns.slasher.CreatePreStates(ctx, sm, ns.candIndexer)
}

func (ns *nativeStakingV2) CreatePostSystemActions(ctx context.Context, sr protocol.StateReader) ([]action.Envelope, error) {
	return createPostSystemActions(ctx, sr, ns)
}

func (ns *nativeStakingV2) Handle(ctx context.Context, act action.Action, sm protocol.StateManager) (*action.Receipt, error) {
	return handle(ctx, act, sm, ns.candIndexer, ns.addr.String())
}

func (ns *nativeStakingV2) Validate(ctx context.Context, act action.Action, sr protocol.StateReader) error {
	return validate(ctx, sr, ns, act)
}

func (ns *nativeStakingV2) CalculateCandidatesByHeight(ctx context.Context, sr protocol.StateReader, height uint64) (state.CandidateList, error) {
	// transition to V2 starting Fairbank
	cands, err := ns.stakingV2.ActiveCandidates(ctx, sr, height)
	if err != nil {
		return cands, err
	}
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	return ns.filterAndSortCandidatesByVoteScore(cands, bcCtx.Tip.Timestamp), nil
}

func (ns *nativeStakingV2) CalculateUnproductiveDelegates(
	ctx context.Context,
	sr protocol.StateReader,
) ([]string, error) {
	return ns.slasher.calculateUnproductiveDelegates(ctx, sr)
}

// Delegates returns exact number of delegates of current epoch
func (ns *nativeStakingV2) Delegates(ctx context.Context, sr protocol.StateReader) (state.CandidateList, error) {
	delegates, _, err := ns.slasher.GetActiveBlockProducers(ctx, sr, false)
	return delegates, err
}

// NextDelegates returns exact number of delegates of next epoch
func (ns *nativeStakingV2) NextDelegates(ctx context.Context, sr protocol.StateReader) (state.CandidateList, error) {
	nextDelegates, _, err := ns.slasher.GetActiveBlockProducers(ctx, sr, true)
	return nextDelegates, err
}

// Candidates returns candidate list from state factory of current epoch
func (ns *nativeStakingV2) Candidates(ctx context.Context, sr protocol.StateReader) (state.CandidateList, error) {
	candidates, _, err := ns.slasher.GetCandidates(ctx, sr, false)
	return candidates, err
}

// NextCandidates returns candidate list from state factory of next epoch
func (ns *nativeStakingV2) NextCandidates(ctx context.Context, sr protocol.StateReader) (state.CandidateList, error) {
	candidates, _, err := ns.slasher.GetCandidates(ctx, sr, true)
	return candidates, err
}

func (ns *nativeStakingV2) ReadState(ctx context.Context, sr protocol.StateReader, method []byte, args ...[]byte) ([]byte, uint64, error) {
	return ns.slasher.ReadState(ctx, sr, ns.candIndexer, method, args...)
}

func (ns *nativeStakingV2) Register(r *protocol.Registry) error {
	return r.Register(_protocolID, ns)
}

// ForceRegister registers the protocol with a unique ID and force replacing the previous protocol if it exists
func (ns *nativeStakingV2) ForceRegister(r *protocol.Registry) error {
	return r.ForceRegister(_protocolID, ns)
}

func (ns *nativeStakingV2) Name() string {
	return _protocolID
}

func (ns *nativeStakingV2) filterAndSortCandidatesByVoteScore(list state.CandidateList, ts time.Time) state.CandidateList {
	candidates := make(map[string]*state.Candidate)
	candidateScores := make(map[string]*big.Int)
	for _, cand := range list {
		if cand.Votes.Cmp(ns.scoreThreshold) >= 0 {
			clone := cand.Clone()
			candidates[string(clone.CanName)] = clone
			candidateScores[string(clone.CanName)] = clone.Votes
		}
	}
	sorted := util.Sort(candidateScores, uint64(ts.Unix()))
	res := make(state.CandidateList, 0, len(sorted))
	for _, name := range sorted {
		res = append(res, candidates[name])
	}
	return res
}
