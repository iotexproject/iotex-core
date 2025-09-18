package staking

import (
	"math"
	"math/big"
	"testing"

	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/contractstaking"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil/testdb"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func testCreateViewData(t *testing.T) *viewData {
	testCandidates := CandidateList{
		{
			Owner:              identityset.Address(1),
			Operator:           identityset.Address(1),
			Reward:             identityset.Address(1),
			Name:               "cand1",
			Votes:              big.NewInt(1000),
			SelfStakeBucketIdx: candidateNoSelfStakeBucketIndex,
			SelfStake:          big.NewInt(10),
		},
		{
			Owner:              identityset.Address(2),
			Operator:           identityset.Address(2),
			Reward:             identityset.Address(2),
			Name:               "cand2",
			Votes:              big.NewInt(1000),
			SelfStakeBucketIdx: candidateNoSelfStakeBucketIndex,
			SelfStake:          big.NewInt(0),
		},
	}
	candCenter, err := NewCandidateCenter(testCandidates)
	require.NoError(t, err)
	return &viewData{
		candCenter: candCenter,
		bucketPool: &BucketPool{
			total: &totalAmount{
				amount: big.NewInt(100),
				count:  1,
			},
		},
	}
}

// TestNewNFTBucketEventHandler tests the creation of a new NFT bucket event handler
func TestNewNFTBucketEventHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	sm := testdb.NewMockStateManager(ctrl)
	sm.EXPECT().Height().Return(uint64(100), nil).AnyTimes()
	sm.EXPECT().ReadView(gomock.Any()).Return(testCreateViewData(t), nil).AnyTimes()
	handler, err := newNFTBucketEventHandler(sm, func(bkt *contractstaking.Bucket, height uint64) *big.Int {
		return big.NewInt(100)
	})
	require.NoError(t, err)
	require.NotNil(t, handler)
}

func TestNewNFTBucketEventHandlerSecondaryOnly(t *testing.T) {
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)
	sm.EXPECT().Height().Return(uint64(100), nil).AnyTimes()
	sm.EXPECT().ReadView(gomock.Any()).Return(testCreateViewData(t), nil).AnyTimes()
	handler, err := newNFTBucketEventHandlerSecondaryOnly(sm, func(bkt *contractstaking.Bucket, height uint64) *big.Int {
		return big.NewInt(100)
	})
	require.NoError(t, err)
	require.NotNil(t, handler)
}

func TestPutBucketType(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	sm := testdb.NewMockStateManager(ctrl)
	sm.EXPECT().Height().Return(uint64(100), nil).AnyTimes()
	sm.EXPECT().ReadView(gomock.Any()).Return(testCreateViewData(t), nil).AnyTimes()
	handler, err := newNFTBucketEventHandler(sm, func(bkt *contractstaking.Bucket, height uint64) *big.Int {
		return big.NewInt(100)
	})
	require.NoError(t, err)
	require.NotNil(t, handler)
	iter, err := state.NewIterator([][]byte{}, [][]byte{})
	require.NoError(t, err)
	sm.EXPECT().PutState(gomock.Any(), gomock.Any(), gomock.Any()).Return(uint64(100), nil).Times(1)
	sm.EXPECT().States(gomock.Any()).Return(uint64(100), iter, nil).Times(1)
	contractAddr := identityset.Address(11)
	require.NoError(t, handler.PutBucketType(contractAddr, &contractstaking.BucketType{
		Amount:      big.NewInt(100),
		Duration:    10,
		ActivatedAt: 0,
	}))
	m, ok := handler.bucketTypes[contractAddr]
	require.True(t, ok)
	require.Equal(t, 1, len(m))
	bts, ok := handler.bucketTypesLookup[contractAddr]
	require.True(t, ok)
	require.Equal(t, uint64(0), bts[100][10])
}

func TestNFTEventHandlerBucket(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	sm := testdb.NewMockStateManager(ctrl)
	sm.EXPECT().Height().Return(uint64(100), nil).AnyTimes()
	require.NoError(t, sm.WriteView(_protocolID, testCreateViewData(t)))
	handler, err := newNFTBucketEventHandler(sm, func(bkt *contractstaking.Bucket, height uint64) *big.Int {
		return big.NewInt(100)
	})
	require.NoError(t, err)
	require.NotNil(t, handler)

	bucket := &contractstaking.Bucket{
		Candidate:        identityset.Address(1),
		Owner:            identityset.Address(5),
		StakedAmount:     big.NewInt(100),
		StakedDuration:   10,
		CreatedAt:        1000,
		UnlockedAt:       1010,
		UnstakedAt:       math.MaxUint64,
		IsTimestampBased: false,
	}
	contractAddr := identityset.Address(11)
	require.NoError(t, handler.PutBucket(contractAddr, 1, bucket))
	csm, err := NewCandidateStateManager(sm)
	require.NoError(t, err)
	candidate := csm.GetByIdentifier(identityset.Address(1))
	require.NotNil(t, candidate)
	require.Equal(t, 0, big.NewInt(1100).Cmp(candidate.Votes))
	bucket, err = handler.DeductBucket(contractAddr, 1)
	require.NoError(t, err)
	require.NotNil(t, bucket)
	candidate = csm.GetByIdentifier(identityset.Address(1))
	require.NotNil(t, candidate)
	require.Equal(t, 0, big.NewInt(1000).Cmp(candidate.Votes))
	require.NoError(t, handler.DeleteBucket(contractAddr, 1))
}
