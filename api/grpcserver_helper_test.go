// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package api

import (
	"math"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

func TestGasLimitAndUsed(t *testing.T) {
	r := require.New(t)

	tsf1, err := action.SignedTransfer(identityset.Address(1).String(), identityset.PrivateKey(0), 1, big.NewInt(1), nil, 100, big.NewInt(1))
	r.NoError(err)
	tsf2, err := action.SignedTransfer(identityset.Address(2).String(), identityset.PrivateKey(0), 2, big.NewInt(1), nil, 250, big.NewInt(1))
	r.NoError(err)

	acts := []*action.SealedEnvelope{tsf1, tsf2}
	receipts := []*action.Receipt{
		{GasConsumed: 40},
		{GasConsumed: 60},
	}
	gasLimit, gasUsed := gasLimitAndUsed(acts, receipts)
	r.Equal(uint64(350), gasLimit)
	r.Equal(uint64(100), gasUsed)

	// empty inputs yield zero
	gasLimit, gasUsed = gasLimitAndUsed(nil, nil)
	r.Zero(gasLimit)
	r.Zero(gasUsed)
}

func buildBlockWithActions(t *testing.T, n int) (*block.Block, []*action.Receipt) {
	t.Helper()
	acts := make([]*action.SealedEnvelope, 0, n)
	receipts := make([]*action.Receipt, 0, n)
	for i := 0; i < n; i++ {
		tsf, err := action.SignedTransfer(identityset.Address(1).String(), identityset.PrivateKey(0), uint64(i+1), big.NewInt(1), nil, 100, big.NewInt(1))
		require.NoError(t, err)
		acts = append(acts, tsf)
		receipts = append(receipts, &action.Receipt{BlockHeight: 1, GasConsumed: 10})
	}
	builder := block.NewTestingBuilder().
		SetHeight(1).
		SetVersion(1).
		SetTimeStamp(time.Now()).
		AddActions(acts...).
		SetReceipts(receipts)
	blk, err := builder.SignAndBuild(identityset.PrivateKey(0))
	require.NoError(t, err)
	return &blk, receipts
}

func TestActionsInBlock(t *testing.T) {
	r := require.New(t)

	t.Run("empty block returns nothing", func(t *testing.T) {
		emptyBlk, _ := buildBlockWithActions(t, 0)
		res, err := actionsInBlock(emptyBlk, nil, 0, 10)
		r.NoError(err)
		r.Empty(res)
	})

	blk, receipts := buildBlockWithActions(t, 5)

	t.Run("count zero is rejected", func(t *testing.T) {
		_, err := actionsInBlock(blk, receipts, 0, 0)
		r.Equal(codes.InvalidArgument, status.Code(err))
	})

	t.Run("start beyond range is rejected", func(t *testing.T) {
		_, err := actionsInBlock(blk, receipts, 5, 1)
		r.Equal(codes.InvalidArgument, status.Code(err))
	})

	t.Run("windowed slice", func(t *testing.T) {
		res, err := actionsInBlock(blk, receipts, 1, 2)
		r.NoError(err)
		r.Len(res, 2)
		r.Equal(uint32(1), res[0].Index)
		r.Equal(uint32(2), res[1].Index)
	})

	t.Run("count larger than remaining is clamped", func(t *testing.T) {
		res, err := actionsInBlock(blk, receipts, 3, 100)
		r.NoError(err)
		r.Len(res, 2)
	})

	t.Run("max count returns all from start", func(t *testing.T) {
		res, err := actionsInBlock(blk, receipts, 0, math.MaxUint64)
		r.NoError(err)
		r.Len(res, 5)
	})

	t.Run("fewer receipts than actions does not panic", func(t *testing.T) {
		// 5 actions but only the first 2 have receipts; the actions lacking a
		// receipt must be skipped instead of triggering an index-out-of-range.
		shortReceipts := receipts[:2]
		r.NotPanics(func() {
			res, err := actionsInBlock(blk, shortReceipts, 0, math.MaxUint64)
			r.NoError(err)
			// only the actions with a corresponding receipt are returned
			r.Len(res, 2)
			r.Equal(uint32(0), res[0].Index)
			r.Equal(uint32(1), res[1].Index)
		})
	})

	t.Run("no receipts at all yields empty result without panic", func(t *testing.T) {
		r.NotPanics(func() {
			res, err := actionsInBlock(blk, nil, 0, math.MaxUint64)
			r.NoError(err)
			r.Empty(res)
		})
	})
}
