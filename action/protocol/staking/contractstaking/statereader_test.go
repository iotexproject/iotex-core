package contractstaking

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_chainmanager"
)

func TestNewContractStateReader(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockSR := mock_chainmanager.NewMockStateReader(ctrl)
	reader := NewStateReader(mockSR)
	require.NotNil(t, reader)
	require.Equal(t, mockSR, reader.sr)
}

func TestNumOfBuckets(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockSR := mock_chainmanager.NewMockStateReader(ctrl)
	reader := NewStateReader(mockSR)

	addr := identityset.Address(0)
	expected := &StakingContract{NumOfBuckets: 10}
	mockSR.EXPECT().State(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(s interface{}, _ ...protocol.StateOption) (uint64, error) {
			*(s.(*StakingContract)) = *expected
			return 0, nil
		},
	)
	num, err := reader.NumOfBuckets(addr)
	require.NoError(t, err)
	require.Equal(t, uint64(10), num)
}

func TestNumOfBuckets_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockSR := mock_chainmanager.NewMockStateReader(ctrl)
	reader := NewStateReader(mockSR)

	addr := identityset.Address(0)
	mockSR.EXPECT().State(gomock.Any(), gomock.Any(), gomock.Any()).Return(uint64(0), errors.New("err"))
	num, err := reader.NumOfBuckets(addr)
	require.Error(t, err)
	require.Equal(t, uint64(0), num)
}

func TestBucket(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockSR := mock_chainmanager.NewMockStateReader(ctrl)
	reader := NewStateReader(mockSR)

	addr := identityset.Address(0)
	bucketID := uint64(123)
	ssb := &Bucket{}
	mockSR.EXPECT().State(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(s interface{}, _ ...protocol.StateOption) (uint64, error) {
			*(s.(*Bucket)) = *ssb
			return 0, nil
		},
	)

	bucket, err := reader.Bucket(addr, bucketID)
	require.NoError(t, err)
	require.NotNil(t, bucket)
}

func TestBucket_ErrorOnState(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockSR := mock_chainmanager.NewMockStateReader(ctrl)
	reader := NewStateReader(mockSR)

	addr := identityset.Address(0)
	bucketID := uint64(123)
	mockSR.EXPECT().State(gomock.Any(), gomock.Any(), gomock.Any()).Return(uint64(0), errors.New("err"))
	bucket, err := reader.Bucket(addr, bucketID)
	require.Error(t, err)
	require.Nil(t, bucket)
}

type dummyIter struct {
	size int
	idx  int
	keys [][]byte
}

func (d *dummyIter) Size() int { return d.size }
func (d *dummyIter) Next(s interface{}) ([]byte, error) {
	if d.idx >= d.size {
		return nil, state.ErrNilValue
	}
	*(s.(*Bucket)) = Bucket{}
	key := d.keys[d.idx]
	d.idx++
	return key, nil
}

func TestBuckets(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockSR := mock_chainmanager.NewMockStateReader(ctrl)
	reader := NewStateReader(mockSR)

	addr := identityset.Address(0)
	iter := &dummyIter{size: 2, keys: [][]byte{{1, 2, 3, 4, 5, 6, 7, 8}, {2, 3, 4, 5, 6, 7, 8, 9}}}
	mockSR.EXPECT().States(gomock.Any()).Return(uint64(0), iter, nil)

	ids, buckets, err := reader.Buckets(addr)
	require.NoError(t, err)
	require.Len(t, ids, 2)
	require.Len(t, buckets, 2)
}

func TestBuckets_StatesError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockSR := mock_chainmanager.NewMockStateReader(ctrl)
	reader := NewStateReader(mockSR)

	addr := identityset.Address(0)
	mockSR.EXPECT().States(gomock.Any()).Return(uint64(0), nil, errors.New("states error"))
	ids, buckets, err := reader.Buckets(addr)
	require.Error(t, err)
	require.Nil(t, ids)
	require.Nil(t, buckets)
}
