package staking

import (
	"context"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_chainmanager"
)

func TestViewData_Fork(t *testing.T) {
	viewData, _ := prepareViewData(t)
	fork, ok := viewData.Fork().(*ViewData)
	require.True(t, ok)
	require.NotNil(t, fork)

	require.Equal(t, viewData.candCenter.size, fork.candCenter.size)
	require.Equal(t, viewData.candCenter.base, fork.candCenter.base)
	require.Equal(t, viewData.candCenter.change, fork.candCenter.change)
	require.NotSame(t, viewData.bucketPool, fork.bucketPool)
	require.Equal(t, viewData.snapshots, fork.snapshots)

	sr := mock_chainmanager.NewMockStateReader(gomock.NewController(t))
	sr.EXPECT().Height().Return(uint64(100), nil).Times(1)
	require.NoError(t, viewData.Commit(context.Background(), sr))

	fork, ok = viewData.Fork().(*ViewData)
	require.True(t, ok)
	require.NotNil(t, fork)
	require.Equal(t, viewData.candCenter.size, fork.candCenter.size)
	require.Equal(t, viewData.candCenter.base, fork.candCenter.base)
	require.Equal(t, viewData.candCenter.change, fork.candCenter.change)
	require.Equal(t, viewData.bucketPool, fork.bucketPool)
	require.Equal(t, viewData.snapshots, fork.snapshots)
}

func prepareViewData(t *testing.T) (*ViewData, int) {
	owner := identityset.Address(0)
	cand := &Candidate{
		Owner:              owner,
		Operator:           owner,
		Reward:             owner,
		Identifier:         owner,
		Name:               "name",
		Votes:              big.NewInt(100),
		SelfStakeBucketIdx: 0,
		SelfStake:          big.NewInt(0),
	}
	candCenter, err := NewCandidateCenter([]*Candidate{cand})
	require.NoError(t, err)
	require.NoError(t, candCenter.Upsert(cand))
	bucketPool := &BucketPool{
		enableSMStorage: false,
		dirty:           true,
		total: &totalAmount{
			amount: big.NewInt(100),
			count:  1,
		},
	}
	viewData := &ViewData{
		candCenter: candCenter,
		bucketPool: bucketPool,
		snapshots:  []Snapshot{},
	}
	return viewData, viewData.Snapshot()
}

func TestViewData_Commit(t *testing.T) {
	viewData, _ := prepareViewData(t)
	require.True(t, viewData.IsDirty())
	mockStateReader := mock_chainmanager.NewMockStateReader(gomock.NewController(t))
	mockStateReader.EXPECT().Height().Return(uint64(100), nil).Times(1)
	require.NoError(t, viewData.Commit(context.Background(), mockStateReader))
	require.False(t, viewData.IsDirty())
	require.Empty(t, viewData.candCenter.change.dirty)
	require.False(t, viewData.bucketPool.dirty)
	require.Empty(t, viewData.snapshots)
}

func TestViewData_Snapshot_Revert(t *testing.T) {
	viewData, ss := prepareViewData(t)
	require.Equal(t, 1, len(viewData.snapshots))
	require.NoError(t, viewData.Revert(ss))
	require.Equal(t, 0, len(viewData.snapshots))
}
