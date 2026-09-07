// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blocksync

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

// TestBlockSyncerSyncRequestServeBounded regression-guards the availability fix for
// blocksync.ProcessSyncRequest: a cheap connected peer must NOT be able to force the
// node to re-serialize and unicast its ENTIRE chain in a single {start:0,end:tip}
// request. Before the fix the request served start..tip (tip+1 blocks) with no range
// cap; now the served window is clamped to end/tip and bounded by
// Config.MaxBlocksPerSyncRequest (keeping the tail of the window).
func TestBlockSyncerSyncRequestServeBounded(t *testing.T) {
	require := require.New(t)

	const tip = uint64(300) // chain of 301 blocks (0..300), larger than the default cap
	cap_ := uint64(DefaultConfig.MaxBlocksPerSyncRequest)

	var (
		blockByHeightCalls int64 // total DB reads triggered by serving
		outboundCalls      int64 // total outbound unicast messages (blocks sent)
	)

	bs, err := NewBlockSyncer(
		DefaultConfig,
		func() uint64 { return tip }, // TipHeight
		func(_ uint64) (*block.Block, error) { // BlockByHeight: count every read; fixed lightweight block
			atomic.AddInt64(&blockByHeightCalls, 1)
			return block.NewBlockDeprecated(
				uint32(1),
				0,
				hash.Hash256{},
				testutil.TimestampNow(),
				identityset.PrivateKey(27).PublicKey(),
				nil,
			), nil
		},
		func(_ *block.Block) error { return nil },           // CommitBlock (unused here)
		func() ([]peer.AddrInfo, error) { return nil, nil }, // Neighbors (unused here)
		func(_ context.Context, _ peer.AddrInfo, msg proto.Message) error { // UniCastOutbound
			atomic.AddInt64(&outboundCalls, 1)
			return nil
		},
		func(_ string) {}, // BlockPeer (unused)
		nil,               // nodeInfoManager
	)
	require.NoError(err)

	peerInfo := peer.AddrInfo{ID: peer.ID("remote-cheap-peer")}

	// ---- Honest incremental request: serves exactly its own (tip..tip) range. ----
	atomic.StoreInt64(&blockByHeightCalls, 0)
	atomic.StoreInt64(&outboundCalls, 0)
	require.NoError(bs.ProcessSyncRequest(context.Background(), peerInfo, tip, tip))
	require.Equal(int64(1), atomic.LoadInt64(&blockByHeightCalls), "honest tip-range request must read 1 block")
	require.Equal(int64(1), atomic.LoadInt64(&outboundCalls), "honest tip-range request must send 1 block")

	// ---- Attack #1: {start:0, end:tip} must now be capped to the per-request limit. ----
	atomic.StoreInt64(&blockByHeightCalls, 0)
	atomic.StoreInt64(&outboundCalls, 0)
	require.NoError(bs.ProcessSyncRequest(context.Background(), peerInfo, 0, tip))
	served := atomic.LoadInt64(&outboundCalls)
	require.Equal(int64(cap_), served,
		"a full-chain request must serve at most MaxBlocksPerSyncRequest blocks (not the whole chain)")
	require.Equal(int64(cap_), atomic.LoadInt64(&blockByHeightCalls),
		"every served block triggers a DB read; the read count must equal the capped serve count")

	// ---- Attack #2: inverted / empty windows must cost nothing. ----
	atomic.StoreInt64(&blockByHeightCalls, 0)
	atomic.StoreInt64(&outboundCalls, 0)
	require.NoError(bs.ProcessSyncRequest(context.Background(), peerInfo, tip+1, tip)) // start > end
	require.Equal(int64(0), atomic.LoadInt64(&outboundCalls), "inverted window must serve nothing")
	// start beyond tip: after clamping end to tip the window is empty.
	require.NoError(bs.ProcessSyncRequest(context.Background(), peerInfo, tip+5, tip+10)) // entirely above tip
	require.Equal(int64(0), atomic.LoadInt64(&outboundCalls), "window above tip must serve nothing")

	// ---- Repeated cheap full-chain requests stay bounded per request (no per-request cost growth). ----
	atomic.StoreInt64(&blockByHeightCalls, 0)
	atomic.StoreInt64(&outboundCalls, 0)
	const repeat = 10
	for i := 0; i < repeat; i++ {
		require.NoError(bs.ProcessSyncRequest(context.Background(), peerInfo, 0, tip))
	}
	require.Equal(int64(repeat*int(cap_)), atomic.LoadInt64(&outboundCalls),
		"each identical full-chain request is independently capped; no unbounded per-request amplification")

	// NOTE: repeated re-requests are additionally rate-limited by the dispatcher's
	// ProcessSyncRequestInterval (default now positive) — tested in dispatcher/; this
	// unit test only locks in the per-request serve cap.
}
