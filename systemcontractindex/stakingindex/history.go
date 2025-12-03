package stakingindex

import (
	"context"
	"errors"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/contractstaking"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
)

type genBlockDurationFn func(view uint64) BlocksDurationFn

// historyIndexer implements historical staking indexer
type historyIndexer struct {
	sr           protocol.StateReader
	startHeight  uint64
	contractAddr address.Address
	epb          EventProcessorBuilder
	cuvwFn       CalculateUnmutedVoteWeightAtFn
	gbdFn        genBlockDurationFn
}

// NewHistoryIndexer creates a new instance of historyIndexer
func NewHistoryIndexer(sr protocol.StateReader, contract address.Address, startHeight uint64, epb EventProcessorBuilder, cuvwFn CalculateUnmutedVoteWeightAtFn, gbdFn genBlockDurationFn) staking.ContractStakingIndexer {
	return &historyIndexer{
		sr:           sr,
		contractAddr: contract,
		startHeight:  startHeight,
		epb:          epb,
		cuvwFn:       cuvwFn,
		gbdFn:        gbdFn,
	}
}

func (h *historyIndexer) Start(ctx context.Context) error {
	return nil
}

func (h *historyIndexer) Stop(ctx context.Context) error {
	return nil
}

func (h *historyIndexer) PutBlock(ctx context.Context, blk *block.Block) error {
	return errors.New("not implemented")
}

// StartHeight returns the start height of the indexer
func (h *historyIndexer) StartHeight() uint64 {
	return h.startHeight
}

// Height returns the latest indexed height
func (h *historyIndexer) Height() (uint64, error) {
	return h.sr.Height()
}

func (h *historyIndexer) Buckets(height uint64) ([]*VoteBucket, error) {
	cssr := contractstaking.NewStateReader(h.sr)
	idxs, btks, err := cssr.Buckets(h.contractAddr)
	if err != nil {
		return nil, err
	}
	return batchAssembleVoteBucket(idxs, btks, h.contractAddr.String(), h.gbdFn(height)), nil
}

// BucketsByIndices returns active buckets by indices
func (h *historyIndexer) BucketsByIndices(idxs []uint64, height uint64) ([]*VoteBucket, error) {
	cssr := contractstaking.NewStateReader(h.sr)
	var btks []*contractstaking.Bucket
	for _, idx := range idxs {
		bkt, err := cssr.Bucket(h.contractAddr, idx)
		if err != nil {
			return nil, err
		}
		btks = append(btks, bkt)
	}
	return batchAssembleVoteBucket(idxs, btks, h.contractAddr.String(), h.gbdFn(height)), nil
}

// BucketsByCandidate returns active buckets by candidate
func (h *historyIndexer) BucketsByCandidate(ownerAddr address.Address, height uint64) ([]*VoteBucket, error) {
	cssr := contractstaking.NewStateReader(h.sr)
	idxs, btks, err := cssr.Buckets(h.contractAddr)
	if err != nil {
		return nil, err
	}
	var filteredIdxs []uint64
	var filteredBtks []*contractstaking.Bucket
	for i, bkt := range btks {
		if bkt.Candidate.String() == ownerAddr.String() {
			filteredIdxs = append(filteredIdxs, idxs[i])
			filteredBtks = append(filteredBtks, bkt)
		}
	}
	return batchAssembleVoteBucket(filteredIdxs, filteredBtks, h.contractAddr.String(), h.gbdFn(height)), nil
}

func (h *historyIndexer) TotalBucketCount(height uint64) (uint64, error) {
	cssr := contractstaking.NewStateReader(h.sr)
	ssb, err := cssr.NumOfBuckets(h.contractAddr)
	if err != nil {
		return 0, err
	}
	return ssb, nil
}

func (h *historyIndexer) ContractAddress() address.Address {
	return h.contractAddr
}

func (h *historyIndexer) LoadStakeView(ctx context.Context, sr protocol.StateReader) (staking.ContractStakeView, error) {
	cvm := NewCandidateVotesManager(h.contractAddr)
	cur, err := cvm.Load(ctx, sr)
	if err != nil {
		return nil, err
	}
	height, err := sr.Height()
	if err != nil {
		return nil, err
	}
	return NewVoteView(h, &VoteViewConfig{ContractAddr: h.contractAddr}, height, cur, h.epb, cvm, h.cuvwFn), nil
}

func (h *historyIndexer) CreateEventProcessor(ctx context.Context, handler staking.EventHandler) staking.EventProcessor {
	return h.epb.Build(ctx, handler)
}

func (h *historyIndexer) ContractStakingBuckets() (uint64, map[uint64]*contractstaking.Bucket, error) {
	cssr := contractstaking.NewStateReader(h.sr)
	idxs, btks, err := cssr.Buckets(h.contractAddr)
	if err != nil {
		return 0, nil, err
	}
	buckets := make(map[uint64]*contractstaking.Bucket)
	for i, id := range idxs {
		buckets[id] = btks[i]
	}
	height, err := h.sr.Height()
	if err != nil {
		return 0, nil, err
	}
	return height, buckets, nil
}

func (h *historyIndexer) DeductBucket(addr address.Address, id uint64) (*contractstaking.Bucket, error) {
	cssr := contractstaking.NewStateReader(h.sr)
	return cssr.Bucket(addr, id)
}

func (h *historyIndexer) IndexerAt(sr protocol.StateReader) staking.ContractStakingIndexer {
	return NewHistoryIndexer(sr, h.contractAddr, h.startHeight, h.epb, h.cuvwFn, h.gbdFn)
}
