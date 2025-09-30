package stakingindex

import (
	"context"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/contractstaking"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

func TestVoteView(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIndexer := staking.NewMockContractStakingIndexer(ctrl)
	vv := NewVoteView(
		mockIndexer,
		&VoteViewConfig{
			ContractAddr: identityset.Address(0),
		},
		100,
		nil,
		nil,
		func(b *contractstaking.Bucket, h uint64) *big.Int {
			return big.NewInt(1)
		},
	)
	mockHandler := staking.NewMockEventHandler(ctrl)
	t.Run("Migrate", func(t *testing.T) {
		buckets := make(map[uint64]*contractstaking.Bucket)
		buckets[1] = &contractstaking.Bucket{
			Candidate:        identityset.Address(1),
			Owner:            identityset.Address(11),
			StakedAmount:     big.NewInt(1000000000000000000),
			StakedDuration:   10,
			CreatedAt:        123456,
			UnlockedAt:       0,
			UnstakedAt:       0,
			IsTimestampBased: false,
			Muted:            false,
		}
		buckets[2] = &contractstaking.Bucket{
			Candidate:        identityset.Address(2),
			Owner:            identityset.Address(12),
			StakedAmount:     big.NewInt(1000000000000000000),
			StakedDuration:   10,
			CreatedAt:        123456,
			UnlockedAt:       234567,
			UnstakedAt:       0,
			IsTimestampBased: true,
			Muted:            false,
		}
		migratedBuckets := make(map[uint64]*contractstaking.Bucket)
		ctx := protocol.WithBlockCtx(context.Background(), protocol.BlockCtx{
			BlockHeight: 100,
		})
		mockIndexer.EXPECT().StartHeight().Return(uint64(0)).Times(1)
		mockIndexer.EXPECT().ContractStakingBuckets().Return(uint64(99), buckets, nil)
		mockHandler.EXPECT().PutBucket(gomock.Any(), gomock.Any(), gomock.Any()).Do(func(addr address.Address, id uint64, bucket *contractstaking.Bucket) error {
			require.Equal(identityset.Address(0), addr)
			migratedBuckets[id] = bucket
			return nil
		}).Times(len(buckets))
		require.NoError(vv.Migrate(ctx, mockHandler))
		require.Equal(buckets, migratedBuckets)
	})
}
