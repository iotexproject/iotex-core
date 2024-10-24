package staking

import (
	"errors"
	"math/big"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"
)

func TestDelayTolerantIndexer(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	indexer := NewMockContractStakingIndexerWithBucketType(ctrl)
	delayIndexer := NewDelayTolerantIndexerWithBucketType(indexer, time.Second)

	var (
		indexerHeight      = uint64(10)
		indexerBuckets     = []*VoteBucket{{Index: 1}}
		indexerAddress     = address.ZeroAddress
		indexerBucketTypes = []*ContractStakingBucketType{{Amount: big.NewInt(1)}}
	)

	// Height
	indexer.EXPECT().Height().DoAndReturn(func() (uint64, error) {
		return atomic.LoadUint64(&indexerHeight), nil
	}).AnyTimes()

	height, err := delayIndexer.Height()
	r.NoError(err)
	r.Equal(atomic.LoadUint64(&indexerHeight), height)
	// Buckets
	indexer.EXPECT().Buckets(gomock.Any()).DoAndReturn(func(height uint64) ([]*VoteBucket, error) {
		if height <= atomic.LoadUint64(&indexerHeight) {
			return indexerBuckets, nil
		}
		return nil, errors.New("invalid height")
	}).AnyTimes()
	noDelayHeight, delayHeight := atomic.LoadUint64(&indexerHeight), atomic.LoadUint64(&indexerHeight)+1
	bkts, err := delayIndexer.Buckets(noDelayHeight)
	r.NoError(err)
	r.Equal(indexerBuckets, bkts)
	bkts, err = delayIndexer.Buckets(delayHeight)
	r.ErrorContains(err, "invalid height")
	go func() {
		time.Sleep(100 * time.Millisecond)
		atomic.StoreUint64(&indexerHeight, delayHeight)
	}()
	bkts, err = delayIndexer.Buckets(delayHeight)
	r.NoError(err)
	r.Equal(indexerBuckets, bkts)
	// BucketsByIndices
	indexer.EXPECT().BucketsByIndices(gomock.Any(), gomock.Any()).DoAndReturn(func(indices []uint64, height uint64) ([]*VoteBucket, error) {
		if height <= atomic.LoadUint64(&indexerHeight) {
			return indexerBuckets, nil
		}
		return nil, errors.New("invalid height")
	}).AnyTimes()
	bkts, err = delayIndexer.BucketsByIndices(nil, delayHeight)
	r.NoError(err)
	r.Equal(indexerBuckets, bkts)
	// BucketsByCandidate
	indexer.EXPECT().BucketsByCandidate(gomock.Any(), gomock.Any()).DoAndReturn(func(ownerAddr address.Address, height uint64) ([]*VoteBucket, error) {
		if height <= atomic.LoadUint64(&indexerHeight) {
			return indexerBuckets, nil
		}
		return nil, errors.New("invalid height")
	}).AnyTimes()
	bkts, err = delayIndexer.BucketsByCandidate(nil, delayHeight)
	r.NoError(err)
	r.Equal(indexerBuckets, bkts)
	// TotalBucketCount
	indexer.EXPECT().TotalBucketCount(gomock.Any()).DoAndReturn(func(height uint64) (uint64, error) {
		if height <= atomic.LoadUint64(&indexerHeight) {
			return uint64(len(indexerBuckets)), nil
		}
		return 0, errors.New("invalid height")
	}).AnyTimes()
	count, err := delayIndexer.TotalBucketCount(delayHeight)
	r.NoError(err)
	r.Equal(uint64(len(indexerBuckets)), count)
	// ContractAddress
	indexer.EXPECT().ContractAddress().Return(indexerAddress).AnyTimes()
	ca := delayIndexer.ContractAddress()
	r.Equal(indexerAddress, ca)
	// BucketTypes
	indexer.EXPECT().BucketTypes(gomock.Any()).DoAndReturn(func(height uint64) ([]*ContractStakingBucketType, error) {
		if height <= atomic.LoadUint64(&indexerHeight) {
			return indexerBucketTypes, nil
		}
		return nil, errors.New("invalid height")
	})
	bucketTypes, err := delayIndexer.BucketTypes(delayHeight)
	r.NoError(err)
	r.Equal(indexerBucketTypes, bucketTypes)
}
