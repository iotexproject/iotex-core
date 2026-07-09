// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"math/big"
	"testing"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_factory"
)

func TestReadStateContractStakingBucketTypes(t *testing.T) {
	contractAddr := identityset.Address(20)
	// mockFactory returns a minimal, empty native view so ConstructBaseView succeeds.
	newSF := func(ctrl *gomock.Controller) *mock_factory.MockFactory {
		sf := mock_factory.NewMockFactory(ctrl)
		sf.EXPECT().Height().Return(uint64(5), nil).AnyTimes()
		candCenter, err := NewCandidateCenter(nil)
		require.NoError(t, err)
		view := &viewData{
			candCenter: candCenter,
			bucketPool: &BucketPool{total: &totalAmount{amount: big.NewInt(0)}},
		}
		sf.EXPECT().ReadView(gomock.Any()).Return(view, nil).AnyTimes()
		return sf
	}
	calc := func(v *VoteBucket, selfStake bool) *big.Int { return v.StakedAmount }

	t.Run("contract staking disabled", func(t *testing.T) {
		r := require.New(t)
		ctrl := gomock.NewController(t)
		sf := newSF(ctrl)
		sr, err := newCompositeStakingStateReader(nil, sf, calc)
		r.NoError(err)
		res, height, err := sr.readStateContractStakingBucketTypes(context.Background(),
			&iotexapi.ReadStakingDataRequest_ContractStakingBucketTypes{ContractAddress: contractAddr.String()})
		r.NoError(err)
		r.EqualValues(5, height)
		r.Empty(res.BucketTypes)
	})

	t.Run("matching indexer returns bucket types", func(t *testing.T) {
		r := require.New(t)
		ctrl := gomock.NewController(t)
		sf := newSF(ctrl)
		indexer := NewMockContractStakingIndexerWithBucketType(ctrl)
		indexer.EXPECT().ContractAddress().Return(contractAddr).AnyTimes()
		indexer.EXPECT().BucketTypes(uint64(5)).Return([]*ContractStakingBucketType{
			{Amount: big.NewInt(100), Duration: 10},
			{Amount: big.NewInt(200), Duration: 20},
		}, nil).Times(1)
		sr, err := newCompositeStakingStateReader(nil, sf, calc, indexer)
		r.NoError(err)
		res, height, err := sr.readStateContractStakingBucketTypes(context.Background(),
			&iotexapi.ReadStakingDataRequest_ContractStakingBucketTypes{ContractAddress: contractAddr.String()})
		r.NoError(err)
		r.EqualValues(5, height)
		r.Len(res.BucketTypes, 2)
		r.Equal("100", res.BucketTypes[0].StakedAmount)
		r.EqualValues(10, res.BucketTypes[0].StakedDuration)
		r.Equal("200", res.BucketTypes[1].StakedAmount)
		r.EqualValues(20, res.BucketTypes[1].StakedDuration)
	})

	t.Run("no indexer matches contract address", func(t *testing.T) {
		r := require.New(t)
		ctrl := gomock.NewController(t)
		sf := newSF(ctrl)
		indexer := NewMockContractStakingIndexerWithBucketType(ctrl)
		indexer.EXPECT().ContractAddress().Return(contractAddr).AnyTimes()
		// BucketTypes must NOT be called when the address does not match
		sr, err := newCompositeStakingStateReader(nil, sf, calc, indexer)
		r.NoError(err)
		res, height, err := sr.readStateContractStakingBucketTypes(context.Background(),
			&iotexapi.ReadStakingDataRequest_ContractStakingBucketTypes{ContractAddress: identityset.Address(21).String()})
		r.NoError(err)
		r.EqualValues(5, height)
		r.Empty(res.BucketTypes)
	})

	t.Run("indexer error is propagated", func(t *testing.T) {
		r := require.New(t)
		ctrl := gomock.NewController(t)
		sf := newSF(ctrl)
		indexer := NewMockContractStakingIndexerWithBucketType(ctrl)
		indexer.EXPECT().ContractAddress().Return(contractAddr).AnyTimes()
		indexer.EXPECT().BucketTypes(uint64(5)).Return(nil, errors.New("boom")).Times(1)
		sr, err := newCompositeStakingStateReader(nil, sf, calc, indexer)
		r.NoError(err)
		_, _, err = sr.readStateContractStakingBucketTypes(context.Background(),
			&iotexapi.ReadStakingDataRequest_ContractStakingBucketTypes{ContractAddress: contractAddr.String()})
		r.ErrorContains(err, "boom")
	})
}
