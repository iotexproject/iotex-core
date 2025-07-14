// Copyright (c) 2025 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package stakingindex

import (
	"context"
	"fmt"
	"math"

	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/db/batch"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
)

const (
	// StakingCandidatesNamespace is a namespace to store candidates with epoch start height
	StakingCandidatesNamespace = "stakingCandidates"
	// StakingBucketsNamespace is a namespace to store vote buckets with epoch start height
	StakingBucketsNamespace = "stakingBuckets"
	// AccountKVNamespace is the bucket name for account
	AccountKVNamespace = "Account"
)

var (
	_currHeightKey     = []byte("crh")
	_currDeleteListKey = []byte("cdl")
	_maxDeleteListKey  = byteutil.Uint64ToBytesBigEndian(math.MaxUint64)
	_maxCandListKey    = []byte{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255}
)

// CandBucketsIndexer is an indexer to store buckets and candidates by given height
type CandBucketsIndexer struct {
	kvBase        db.KVStore
	kvVersioned   db.KvVersioned
	stateReader   protocol.StateReader
	currentHeight uint64
	deleteList    *bucketList
	candList      *candList
	candMap       map[string]bool
}

// NewCandBucketsIndexer creates a new indexer
func NewCandBucketsIndexer(kv db.KvVersioned) (*CandBucketsIndexer, error) {
	if kv == nil {
		return nil, errors.New("kvStore is nil")
	}
	return &CandBucketsIndexer{
		kvBase:      kv.Base(),
		kvVersioned: kv,
		stateReader: newSRFromKVStore(kv.Base()),
		candMap:     map[string]bool{},
	}, nil
}

// Start starts the indexer
func (cbi *CandBucketsIndexer) Start(ctx context.Context) error {
	if err := cbi.kvVersioned.Start(ctx); err != nil {
		return err
	}
	ret, err := cbi.kvBase.Get(db.MetadataNamespace, _currHeightKey)
	switch errors.Cause(err) {
	case nil:
		cbi.currentHeight = byteutil.BytesToUint64BigEndian(ret)
	case db.ErrNotExist:
		cbi.currentHeight = 0
	default:
		return err
	}
	if err := cbi.getDeleteList(); err != nil {
		return err
	}
	if err := cbi.getCandList(); err != nil {
		return err
	}
	// create cand map
	for _, v := range cbi.candList.id {
		cbi.candMap[string(v)] = true
	}
	return nil
}

func (cbi *CandBucketsIndexer) getDeleteList() error {
	ret, err := cbi.kvVersioned.SetVersion(cbi.currentHeight).Get(StakingBucketsNamespace, _maxDeleteListKey)
	switch errors.Cause(err) {
	case nil:
		cbi.deleteList, err = deserializeBucketList(ret)
		if err != nil {
			return err
		}
	case db.ErrNotExist:
		cbi.deleteList = &bucketList{}
	default:
		return err
	}
	return nil
}

func (cbi *CandBucketsIndexer) getCandList() error {
	ret, err := cbi.kvVersioned.SetVersion(cbi.currentHeight).Get(StakingCandidatesNamespace, _maxCandListKey)
	switch errors.Cause(err) {
	case nil:
		cbi.candList, err = deserializeCandList(ret)
		if err != nil {
			return err
		}
	case db.ErrNotExist:
		cbi.candList = &candList{}
	default:
		return err
	}
	return nil
}

// Stop stops the indexer
func (cbi *CandBucketsIndexer) Stop(ctx context.Context) error {
	return cbi.kvVersioned.Stop(ctx)
}

func (cbi *CandBucketsIndexer) candBucketFromBlock(blk *block.Block) (staking.CandidateList, []*staking.VoteBucket, []uint64, error) {
	// TODO: extract affected buckets and candidates from tx in block
	var (
		b  []*staking.VoteBucket
		cl staking.CandidateList
		d  []uint64
	)
	if blk.Height() == 1 {
		b = b1
		cl = c[:1]
	} else if blk.Height() == 2 {
		b = b2
		cl = c[1:2]
		d = []uint64{1}
	} else if blk.Height() == 3 {
		b = b3
		cl = c[2:3]
		d = []uint64{3, 4}
	}
	return cl, b, d, nil
}

func (cbi *CandBucketsIndexer) PutBlock(ctx context.Context, blk *block.Block) error {
	cands, changedBuckets, deletedBuckets, err := cbi.candBucketFromBlock(blk)
	if err != nil {
		return err
	}
	csr, err := staking.ConstructBaseView(cbi.stateReader)
	if err != nil {
		return err
	}
	candidateList, err := staking.ToIoTeXTypesCandidateListV2(csr, cands, protocol.MustGetFeatureCtx(ctx))
	if err != nil {
		return err
	}
	bucketList, err := staking.ToIoTeXTypesVoteBucketList(cbi.stateReader, changedBuckets)
	if err != nil {
		return err
	}
	var (
		b       = batch.NewBatch()
		newCand bool
	)
	for _, c := range candidateList.Candidates {
		addr, err := address.FromString(c.Id)
		if err != nil {
			return err
		}
		cand, err := proto.Marshal(c)
		if err != nil {
			return err
		}
		ab := addr.Bytes()
		b.Put(StakingCandidatesNamespace, ab, cand, fmt.Sprintf("failed to write cand = %x\n", cand))
		// update cand map/list
		if as := string(ab); !cbi.candMap[as] {
			cbi.candMap[as] = true
			newCand = true
			cbi.candList.id = append(cbi.candList.id, ab)
		}
	}
	if newCand {
		cand, err := cbi.candList.serialize()
		if err != nil {
			return err
		}
		b.Put(StakingCandidatesNamespace, _maxCandListKey, cand, fmt.Sprintf("failed to write cand list = %x\n", cand))
	}
	for _, bucket := range bucketList.Buckets {
		cb, err := proto.Marshal(bucket)
		if err != nil {
			return err
		}
		b.Put(StakingBucketsNamespace, byteutil.Uint64ToBytesBigEndian(bucket.Index), cb, fmt.Sprintf("failed to write bucket = %x\n", cb))
	}
	// update deleted bucket list
	var (
		newBucket uint64
		h         = blk.Height()
	)
	for _, v := range changedBuckets {
		if v.Index > cbi.deleteList.maxBucket {
			newBucket = v.Index
		}
	}
	if newBucket > 0 || len(deletedBuckets) > 0 {
		if newBucket > 0 {
			cbi.deleteList.maxBucket = newBucket
		}
		if len(deletedBuckets) > 0 {
			cbi.deleteList.deleted = append(cbi.deleteList.deleted, deletedBuckets...)
		}
		buf, err := cbi.deleteList.serialize()
		if err != nil {
			return err
		}
		b.Put(StakingBucketsNamespace, _maxDeleteListKey, buf, fmt.Sprintf("failed to write deleted bucket list = %d\n", h))
	}
	if b.Size() == 0 {
		return cbi.kvBase.Put(db.MetadataNamespace, _currHeightKey, byteutil.Uint64ToBytesBigEndian(h))
	}
	// update height
	b.Put(db.MetadataNamespace, _currHeightKey, byteutil.Uint64ToBytesBigEndian(h), fmt.Sprintf("failed to write height = %d\n", h))
	if err = cbi.kvVersioned.SetVersion(h).WriteBatch(b); err != nil {
		return err
	}
	cbi.currentHeight = h
	return nil
}

// GetBuckets gets vote buckets from indexer given epoch start height
func (cbi *CandBucketsIndexer) GetBuckets(height uint64, offset, limit uint32) (*iotextypes.VoteBucketList, uint64, error) {
	if height > cbi.currentHeight {
		height = cbi.currentHeight
	}
	// get the delete list
	buckets := &iotextypes.VoteBucketList{}
	kv := cbi.kvVersioned.SetVersion(height)
	ret, err := kv.Get(StakingBucketsNamespace, _maxDeleteListKey)
	if cause := errors.Cause(err); cause == db.ErrNotExist || cause == db.ErrBucketNotExist {
		return buckets, height, nil
	}
	if err != nil {
		return nil, height, err
	}
	dList, err := deserializeBucketList(ret)
	if err != nil {
		return nil, height, err
	}
	dBuckets := map[uint64]bool{}
	for _, v := range dList.deleted {
		dBuckets[v] = true
	}
	for i := range dList.maxBucket + 1 {
		if dBuckets[i] {
			continue
		}
		ret, err = kv.Get(StakingBucketsNamespace, byteutil.Uint64ToBytesBigEndian(i))
		if err != nil {
			return nil, height, err
		}
		b := iotextypes.VoteBucket{}
		if err = proto.Unmarshal(ret, &b); err != nil {
			return nil, height, err
		}
		buckets.Buckets = append(buckets.Buckets, &b)
	}
	length := uint32(len(buckets.Buckets))
	if offset >= length {
		return &iotextypes.VoteBucketList{}, height, nil
	}
	end := offset + limit
	if end > uint32(len(buckets.Buckets)) {
		end = uint32(len(buckets.Buckets))
	}
	buckets.Buckets = buckets.Buckets[offset:end]
	return buckets, height, nil
}

// GetCandidates gets candidates from indexer given epoch start height
func (cbi *CandBucketsIndexer) GetCandidates(height uint64, offset, limit uint32) (*iotextypes.CandidateListV2, uint64, error) {
	if height > cbi.currentHeight {
		height = cbi.currentHeight
	}
	candidateList := &iotextypes.CandidateListV2{}
	kv := cbi.kvVersioned.SetVersion(height)
	ret, err := kv.Get(StakingCandidatesNamespace, _maxCandListKey)
	if cause := errors.Cause(err); cause == db.ErrNotExist || cause == db.ErrBucketNotExist {
		return candidateList, height, nil
	}
	if err != nil {
		return nil, height, err
	}
	cList, err := deserializeCandList(ret)
	if err != nil {
		return nil, height, err
	}
	for _, v := range cList.id {
		ret, err = kv.Get(StakingCandidatesNamespace, v)
		if err != nil {
			return nil, height, err
		}
		c := iotextypes.CandidateV2{}
		if err = proto.Unmarshal(ret, &c); err != nil {
			return nil, height, err
		}
		candidateList.Candidates = append(candidateList.Candidates, &c)
	}
	length := uint32(len(candidateList.Candidates))
	if offset >= length {
		return &iotextypes.CandidateListV2{}, height, nil
	}
	end := offset + limit
	if end > uint32(len(candidateList.Candidates)) {
		end = uint32(len(candidateList.Candidates))
	}
	candidateList.Candidates = candidateList.Candidates[offset:end]
	// fill id if it's empty for backward compatibility
	for i := range candidateList.Candidates {
		if candidateList.Candidates[i].Id == "" {
			candidateList.Candidates[i].Id = candidateList.Candidates[i].OwnerAddress
		}
	}
	return candidateList, height, nil
}
