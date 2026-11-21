// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package chainservice

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iotexproject/iotex-proto/golang/iotexrpc"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/test/mock/mock_blockchain"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_blockdao"
)

// The observation height should stay within the filter's accepted window.
// It remains unvalidated input and must never drive a process restart.
func TestFilterOnlyRecordsAcceptableHeights(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)

	dao := mock_blockdao.NewMockBlockDAO(ctrl)
	dao.EXPECT().Height().Return(uint64(100), nil).AnyTimes()
	cs := &ChainService{blockdao: dao}

	blockAt := func(h uint64) *iotextypes.Block {
		return &iotextypes.Block{Header: &iotextypes.BlockHeader{
			Core: &iotextypes.BlockHeaderCore{Height: h},
		}}
	}
	const cap = 200

	// far-future: rejected, and must leave the watchdog input untouched
	r.False(cs.Filter(iotexrpc.MessageType_BLOCK, blockAt(1<<62), cap))
	r.Zero(atomic.LoadUint64(&cs.lastReceivedBlockHeight))

	// exactly at the window edge: rejected too
	r.False(cs.Filter(iotexrpc.MessageType_BLOCK, blockAt(100+cap), cap))
	r.Zero(atomic.LoadUint64(&cs.lastReceivedBlockHeight))

	// inside the window: accepted and recorded
	r.True(cs.Filter(iotexrpc.MessageType_BLOCK, blockAt(150), cap))
	r.EqualValues(150, atomic.LoadUint64(&cs.lastReceivedBlockHeight))

	// a lower height must not move it backwards
	r.True(cs.Filter(iotexrpc.MessageType_BLOCK, blockAt(120), cap))
	r.EqualValues(150, atomic.LoadUint64(&cs.lastReceivedBlockHeight))
}

// Exercise the production monitor without waiting for minute-long ticks. Even
// beyond the former three-tick panic threshold, unvalidated heights may only
// update observations. Cover both writers of lastReceivedBlockHeight.
func TestChainProgressMonitorDoesNotRestartOnUnvalidatedBlocks(t *testing.T) {
	for _, source := range []string{"Filter", "ReportFullness"} {
		t.Run(source, func(t *testing.T) {
			r := require.New(t)
			ctrl := gomock.NewController(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			chain := mock_blockchain.NewMockBlockchain(ctrl)
			dao := mock_blockdao.NewMockBlockDAO(ctrl)
			cs := &ChainService{chain: chain, blockdao: dao}
			dao.EXPECT().Height().Return(uint64(100), nil).AnyTimes()
			stalledTicks := func() float64 {
				metric := &dto.Metric{}
				r.NoError(_stalledTipMtc.Write(metric))
				return metric.GetGauge().GetValue()
			}
			samples := []struct {
				tip, received uint64
				stalled       float64
			}{
				{100, 100, 0}, // establish a baseline
				{100, 101, 1},
				{100, 102, 2},
				{100, 103, 3}, // former panic threshold
				{100, 104, 4},
				{100, 104, 0}, // no new inbound height resets the observation
				{100, 105, 1},
				{101, 106, 0}, // real tip progress resets it too
			}
			ticks := make(chan time.Time, len(samples))
			for range samples {
				ticks <- time.Time{}
			}
			index := 0
			chain.EXPECT().TipHeight().DoAndReturn(func() uint64 {
				if index > 0 {
					r.Equal(samples[index-1].stalled, stalledTicks())
				}
				sample := samples[index]
				block := &iotextypes.Block{Header: &iotextypes.BlockHeader{
					Core: &iotextypes.BlockHeaderCore{Height: sample.received},
				}}
				if source == "Filter" {
					r.True(cs.Filter(iotexrpc.MessageType_BLOCK, block, 200))
				} else {
					cs.ReportFullness(ctx, iotexrpc.MessageType_BLOCK, block, 0.5)
				}
				index++
				if index == len(samples) {
					cancel()
				}
				return sample.tip
			}).Times(len(samples))
			r.NotPanics(func() { cs.monitorChainProgress(ctx, ticks) })
			r.Equal(len(samples), index)
			r.Equal(samples[len(samples)-1].stalled, stalledTicks())
		})
	}
}
