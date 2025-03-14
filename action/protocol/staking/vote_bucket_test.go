// Copyright (c) 2020 IoTeX
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"math/big"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/consensus/consensusfsm"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil/testdb"
)

const (
	_stateDBPath = "stateDB.test"
)

func TestGetPutStaking(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)
	csm := newCandidateStateManager(sm)
	csr := newCandidateStateReader(sm)
	sm.PutState(
		&totalBucketCount{count: 0},
		protocol.NamespaceOption(_stakingNameSpace),
		protocol.KeyOption(TotalBucketKey),
	)

	tests := []struct {
		name  hash.Hash160
		index uint64
	}{
		{
			hash.BytesToHash160([]byte{1, 2, 3, 4}),
			0,
		},
		{
			hash.BytesToHash160([]byte{1, 2, 3, 4}),
			1,
		},
		{
			hash.BytesToHash160([]byte{2, 3, 4, 5}),
			2,
		},
		{
			hash.BytesToHash160([]byte{2, 3, 4, 5}),
			3,
		},
	}

	// put buckets and get
	for _, e := range tests {
		addr, _ := address.FromBytes(e.name[:])
		_, err := csr.getBucket(e.index)
		require.Equal(state.ErrStateNotExist, errors.Cause(err))

		vb := NewVoteBucket(addr, identityset.Address(1), big.NewInt(2100000000), 21*uint32(e.index+1), time.Now(), true)

		count, err := csr.getTotalBucketCount()
		require.NoError(err)
		require.Equal(e.index, count)
		count, err = csm.putBucket(vb)
		require.NoError(err)
		require.Equal(e.index, count)
		count, err = csr.getTotalBucketCount()
		require.NoError(err)
		require.Equal(e.index+1, count)
		vb1, err := csr.getBucket(e.index)
		require.NoError(err)
		require.Equal(e.index, vb1.Index)
		require.Equal(vb, vb1)
	}

	vb, err := csr.getBucket(2)
	require.NoError(err)
	vb.AutoStake = false
	vb.StakedAmount.Sub(vb.StakedAmount, big.NewInt(100))
	vb.UnstakeStartTime = time.Now().UTC()
	require.True(vb.isUnstaked())
	require.NoError(csm.updateBucket(2, vb))
	vb1, err := csr.getBucket(2)
	require.NoError(err)
	require.Equal(vb, vb1)

	// delete buckets and get
	for _, e := range tests {
		require.NoError(csm.delBucket(e.index))
		_, err := csr.getBucket(e.index)
		require.Equal(state.ErrStateNotExist, errors.Cause(err))
	}
}

func TestCalculateVoteWeight(t *testing.T) {
	// Define test cases
	blockInterval := consensusfsm.DefaultDardanellesUpgradeConfig.BlockInterval
	consts := genesis.TestDefault().VoteWeightCalConsts
	tests := []struct {
		name       string
		consts     genesis.VoteWeightCalConsts
		voteBucket *VoteBucket
		selfStake  bool
		expected   *big.Int
	}{
		{
			name:   "Native, auto-stake enabled, self-stake enabled",
			consts: consts,
			voteBucket: &VoteBucket{
				StakedDuration: time.Duration(100) * 24 * time.Hour,
				AutoStake:      true,
				StakedAmount:   big.NewInt(100),
			},
			selfStake: true,
			expected:  big.NewInt(136),
		},
		{
			name:   "Native, auto-stake enabled, self-stake disabled",
			consts: consts,
			voteBucket: &VoteBucket{
				StakedDuration: time.Duration(100) * 24 * time.Hour,
				AutoStake:      true,
				StakedAmount:   big.NewInt(100),
			},
			selfStake: false,
			expected:  big.NewInt(129),
		},
		{
			name:   "Native, auto-stake disabled, self-stake enabled",
			consts: consts,
			voteBucket: &VoteBucket{
				StakedDuration: time.Duration(100) * 24 * time.Hour,
				AutoStake:      false,
				StakedAmount:   big.NewInt(100),
			},
			selfStake: true,
			expected:  big.NewInt(125),
		},
		{
			name:   "Native, auto-stake disabled, self-stake disabled",
			consts: consts,
			voteBucket: &VoteBucket{
				StakedDuration: time.Duration(100) * 24 * time.Hour,
				AutoStake:      false,
				StakedAmount:   big.NewInt(100),
			},
			selfStake: false,
			expected:  big.NewInt(125),
		},
		{
			name:   "NFT, auto-stake enabled",
			consts: consts,
			voteBucket: &VoteBucket{
				StakedDuration:            30 * 17280 * blockInterval,
				StakedDurationBlockNumber: 30 * 17280,
				AutoStake:                 true,
				StakedAmount:              big.NewInt(10000),
				ContractAddress:           identityset.Address(1).String(),
			},
			selfStake: false,
			expected:  big.NewInt(12245),
		},
		{
			name:   "NFT, auto-stake disabled",
			consts: consts,
			voteBucket: &VoteBucket{
				StakedDuration:            30 * 17280 * blockInterval,
				StakedDurationBlockNumber: 30 * 17280,
				AutoStake:                 false,
				StakedAmount:              big.NewInt(10000),
				ContractAddress:           identityset.Address(1).String(),
			},
			selfStake: false,
			expected:  big.NewInt(11865),
		},
		{
			name:   "NFT-I, auto-stake enabled",
			consts: consts,
			voteBucket: &VoteBucket{
				StakedDuration:            91 * 17280 * blockInterval,
				StakedDurationBlockNumber: 91 * 17280,
				AutoStake:                 true,
				StakedAmount:              big.NewInt(10000),
				ContractAddress:           identityset.Address(1).String(),
			},
			selfStake: false,
			expected:  big.NewInt(12854),
		},
		{
			name:   "NFT-I, auto-stake disabled",
			consts: consts,
			voteBucket: &VoteBucket{
				StakedDuration:            91 * 17280 * blockInterval,
				StakedDurationBlockNumber: 91 * 17280,
				AutoStake:                 false,
				StakedAmount:              big.NewInt(10000),
				ContractAddress:           identityset.Address(1).String(),
			},
			selfStake: true,
			expected:  big.NewInt(12474),
		},
		{
			name:   "NFT-II, auto-stake enabled",
			consts: consts,
			voteBucket: &VoteBucket{
				StakedDuration:            91 * 17280 * blockInterval,
				StakedDurationBlockNumber: 91 * 17280,
				AutoStake:                 true,
				StakedAmount:              big.NewInt(100000),
				ContractAddress:           identityset.Address(1).String(),
			},
			selfStake: false,
			expected:  big.NewInt(128543),
		},
		{
			name:   "NFT-II, auto-stake disabled",
			consts: consts,
			voteBucket: &VoteBucket{
				StakedDuration:            91 * 17280 * blockInterval,
				StakedDurationBlockNumber: 91 * 17280,
				AutoStake:                 false,
				StakedAmount:              big.NewInt(100000),
				ContractAddress:           identityset.Address(1).String(),
			},
			selfStake: true,
			expected:  big.NewInt(124741),
		},
		{
			name:   "NFT-III, auto-stake enabled",
			consts: consts,
			voteBucket: &VoteBucket{
				StakedDuration:            2 * 17280 * blockInterval,
				StakedDurationBlockNumber: 2 * 17280,
				AutoStake:                 true,
				StakedAmount:              big.NewInt(1000),
				ContractAddress:           identityset.Address(1).String(),
			},
			selfStake: false,
			expected:  big.NewInt(1076),
		},
		{
			name:   "NFT-III, auto-stake disabled",
			consts: consts,
			voteBucket: &VoteBucket{
				StakedDuration:            2 * 17280 * blockInterval,
				StakedDurationBlockNumber: 2 * 17280,
				AutoStake:                 false,
				StakedAmount:              big.NewInt(1000),
				ContractAddress:           identityset.Address(1).String(),
			},
			selfStake: true,
			expected:  big.NewInt(1038),
		},
	}

	// Run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := CalculateVoteWeight(tt.consts, tt.voteBucket, tt.selfStake)
			require.Equal(t, tt.expected, actual)
		})
	}
}
func TestIsUnstaked(t *testing.T) {
	r := require.New(t)

	// Test for native vote bucket
	vb := NewVoteBucket(identityset.Address(1), identityset.Address(2), big.NewInt(10), 1, time.Now(), false)
	r.False(vb.isUnstaked())
	vb.UnstakeStartTime = vb.StakeStartTime.Add(1 * time.Hour)
	r.True(vb.isUnstaked())

	// Test for nft vote bucket
	vb = NewVoteBucket(identityset.Address(1), identityset.Address(2), big.NewInt(10), 1, time.Now(), false)
	vb.ContractAddress = identityset.Address(1).String()
	vb.CreateBlockHeight = 1
	vb.StakeStartBlockHeight = 1
	vb.UnstakeStartBlockHeight = maxBlockNumber
	r.False(vb.isUnstaked())
	vb.UnstakeStartBlockHeight = 2
	r.True(vb.isUnstaked())
}
