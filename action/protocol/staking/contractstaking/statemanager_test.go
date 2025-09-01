package contractstaking

import (
	"errors"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_chainmanager"
)

// TestNewContractStakingStateManager tests the creation of a new ContractStakingStateManager.
func TestNewContractStakingStateManager(t *testing.T) {
	mockSM := &mock_chainmanager.MockStateManager{}
	csm := NewContractStakingStateManager(mockSM)
	assert.NotNil(t, csm)
	assert.Equal(t, mockSM, csm.sm)
}

// TestUpsertBucketType_Success tests the successful insertion of a bucket type.
func TestUpsertBucketType_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSM := mock_chainmanager.NewMockStateManager(ctrl)
	mockSM.EXPECT().PutState(gomock.Any(), gomock.Any(), gomock.Any()).Return(uint64(0), nil)
	csm := NewContractStakingStateManager(mockSM)

	contractAddr := identityset.Address(0)
	bucketID := uint64(123)
	bucketType := &BucketType{
		Amount:      big.NewInt(1000),
		Duration:    3600,
		ActivatedAt: 1622547800,
	}

	err := csm.UpsertBucketType(contractAddr, bucketID, bucketType)
	require.NoError(t, err)
}

// TestUpsertBucketType_Error tests the error case for inserting a bucket type.
func TestUpsertBucketType_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSM := mock_chainmanager.NewMockStateManager(ctrl)
	mockSM.EXPECT().PutState(gomock.Any(), gomock.Any(), gomock.Any()).Return(uint64(0), errors.New("putstate error"))
	csm := NewContractStakingStateManager(mockSM)

	contractAddr := identityset.Address(1)
	bucketID := uint64(456)
	bucketType := &BucketType{
		Amount:      big.NewInt(2000),
		Duration:    7200,
		ActivatedAt: 1622548100,
	}

	err := csm.UpsertBucketType(contractAddr, bucketID, bucketType)
	assert.Error(t, err)
	assert.Equal(t, "putstate error", err.Error())
}

func TestDeleteBucket_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSM := mock_chainmanager.NewMockStateManager(ctrl)
	mockSM.EXPECT().DelState(gomock.Any(), gomock.Any()).Return(uint64(0), errors.New("delstate error"))
	csm := NewContractStakingStateManager(mockSM)

	contractAddr := identityset.Address(1)
	bucketID := uint64(456)

	err := csm.DeleteBucket(contractAddr, bucketID)
	assert.Error(t, err)
	assert.Equal(t, "delstate error", err.Error())
}

// TestDeleteBucket_Success tests the successful deletion of a bucket.
func TestDeleteBucket_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSM := mock_chainmanager.NewMockStateManager(ctrl)
	mockSM.EXPECT().DelState(gomock.Any(), gomock.Any()).Return(uint64(0), nil)
	csm := NewContractStakingStateManager(mockSM)

	contractAddr := identityset.Address(1)
	bucketID := uint64(456)

	err := csm.DeleteBucket(contractAddr, bucketID)
	require.NoError(t, err)
}

// TestUpsertBucket_Success tests the successful insertion of a bucket.
func TestUpsertBucket_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSM := mock_chainmanager.NewMockStateManager(ctrl)
	mockSM.EXPECT().PutState(gomock.Any(), gomock.Any(), gomock.Any()).Return(uint64(0), nil)
	csm := NewContractStakingStateManager(mockSM)

	contractAddr := identityset.Address(0)
	bid := uint64(123)
	bucket := &Bucket{
		Owner:          identityset.Address(1),
		Candidate:      identityset.Address(2),
		StakedAmount:   big.NewInt(1000),
		StakedDuration: 3600,
		CreatedAt:      1622547800,
		UnlockedAt:     1622547900,
		UnstakedAt:     1622548000,
		Muted:          false,
	}

	err := csm.UpsertBucket(contractAddr, bid, bucket)
	require.NoError(t, err)
}

// TestUpsertBucket_Error tests the error case for inserting a bucket.
func TestUpsertBucket_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSM := mock_chainmanager.NewMockStateManager(ctrl)
	mockSM.EXPECT().PutState(gomock.Any(), gomock.Any(), gomock.Any()).Return(uint64(0), errors.New("putstate error"))
	csm := NewContractStakingStateManager(mockSM)

	contractAddr := identityset.Address(0)
	bid := uint64(456)
	bucket := &Bucket{
		Owner:          identityset.Address(3),
		Candidate:      identityset.Address(4),
		StakedAmount:   big.NewInt(2000),
		StakedDuration: 7200,
		CreatedAt:      1622548100,
		UnlockedAt:     1622548200,
		UnstakedAt:     1622548300,
		Muted:          true,
	}

	err := csm.UpsertBucket(contractAddr, bid, bucket)
	assert.Error(t, err)
	assert.Equal(t, "putstate error", err.Error())
}

// TestUpdateContractMeta_Success tests the successful update of contract metadata.
func TestUpdateContractMeta_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSM := mock_chainmanager.NewMockStateManager(ctrl)
	mockSM.EXPECT().PutState(gomock.Any(), gomock.Any(), gomock.Any()).Return(uint64(0), nil)
	csm := NewContractStakingStateManager(mockSM)

	contractAddr := identityset.Address(5)
	numOfBuckets := uint64(10)

	err := csm.UpdateNumOfBuckets(contractAddr, numOfBuckets)
	require.NoError(t, err)
}

// TestUpdateContractMeta_Error tests the error case for updating contract metadata.
func TestUpdateContractMeta_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSM := mock_chainmanager.NewMockStateManager(ctrl)
	mockSM.EXPECT().PutState(gomock.Any(), gomock.Any(), gomock.Any()).Return(uint64(0), errors.New("meta error"))
	csm := NewContractStakingStateManager(mockSM)

	contractAddr := identityset.Address(6)
	numOfBuckets := uint64(20)

	err := csm.UpdateNumOfBuckets(contractAddr, numOfBuckets)
	assert.Error(t, err)
	assert.Equal(t, "meta error", err.Error())
}
