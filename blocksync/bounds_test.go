// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blocksync

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotexrpc"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/nodeinfo"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// blockAtHeight builds a distinct, structurally valid block at the given height.
// nonce only varies the hash.
func blockAtHeight(t *testing.T, height uint64, nonce uint64) *block.Block {
	t.Helper()
	blk, err := block.NewTestingBuilder().
		SetHeight(height).
		SetVersion(uint32(nonce)).
		SignAndBuild(identityset.PrivateKey(0))
	require.NoError(t, err)
	return &blk
}

// However many distinct peers flood a height, the buffered-block count for it
// stays bounded by maxBlocksPerHeight. Blocks are buffered before any validation
// and deduped only by full block hash, so without the cap a flood of cheaply
// mutated variants would grow the buffer without bound while the tip stalls.
func TestBufferBoundsBlocksPerHeight(t *testing.T) {
	r := require.New(t)

	b := newBlockBuffer(200, 20)
	const tip, height = 10, 12

	for i := range maxBlocksPerHeight * 4 {
		pid := "peer" + strconv.Itoa(i)
		b.AddBlock(tip, newPeerBlock(pid, blockAtHeight(t, height, uint64(i))))
	}
	r.Len(b.blockQueues[height].blocks, maxBlocksPerHeight)
}

// No coalition of peers may crowd out the authentic block. Even once the height
// is full of unvalidated candidates, a block delivered by a peer that is not yet
// over-represented displaces an over-represented one instead of being dropped,
// so the honest block always secures a slot while memory stays bounded.
func TestBufferCoalitionCannotExcludeHonestBlock(t *testing.T) {
	r := require.New(t)

	b := newBlockBuffer(200, 20)
	const tip, height = 10, 12

	// four peers fill all maxBlocksPerHeight (=8) slots, two each
	nonce := uint64(0)
	for p := 0; p < maxBlocksPerHeight/maxBlocksPerPeerPerHeight; p++ {
		pid := "busy" + strconv.Itoa(p)
		for range maxBlocksPerPeerPerHeight {
			ok, _ := b.AddBlock(tip, newPeerBlock(pid, blockAtHeight(t, height, nonce)))
			r.True(ok)
			nonce++
		}
	}
	r.Len(b.blockQueues[height].blocks, maxBlocksPerHeight)

	// the authentic block, delivered later by a different peer, is admitted
	honest := blockAtHeight(t, height, 999)
	ok, _ := b.AddBlock(tip, newPeerBlock("honest", honest))
	r.True(ok)
	r.Len(b.blockQueues[height].blocks, maxBlocksPerHeight) // still bounded

	// and it is actually present in the queue
	found := false
	for _, blk := range b.blockQueues[height].blocks {
		if blk.pid == "honest" {
			found = true
		}
	}
	r.True(found)
}

// A single peer is still held to its per-peer share and cannot take the height.
func TestBufferCapsBlocksPerPeer(t *testing.T) {
	r := require.New(t)

	b := newBlockBuffer(200, 20)
	const tip, height = 10, 12

	accepted := 0
	for i := range maxBlocksPerHeight * 4 {
		if ok, _ := b.AddBlock(tip, newPeerBlock("busy", blockAtHeight(t, height, uint64(i)))); ok {
			accepted++
		}
	}
	r.Equal(maxBlocksPerPeerPerHeight, accepted)
}

// Heights at or below the tip can never be popped again, so they must not
// accumulate in the buffer map.
func TestBufferDropsHeightsAtOrBelowTip(t *testing.T) {
	r := require.New(t)

	b := newBlockBuffer(200, 20)
	for h := uint64(11); h <= 20; h++ {
		ok, _ := b.AddBlock(10, newPeerBlock("p", blockAtHeight(t, h, 0)))
		r.True(ok)
	}
	r.Len(b.blockQueues, 10)

	// tip advances past most of them
	ok, _ := b.AddBlock(19, newPeerBlock("p", blockAtHeight(t, 21, 0)))
	r.True(ok)
	r.Len(b.blockQueues, 2) // heights 20 and 21 only
	for h := range b.blockQueues {
		r.Greater(h, uint64(19))
	}
}

// A duplicate block must still be rejected, and a full height must not evict
// what is already there.
func TestBufferDedupStillApplies(t *testing.T) {
	r := require.New(t)

	b := newBlockBuffer(200, 20)
	blk := blockAtHeight(t, 12, 0)
	ok, _ := b.AddBlock(10, newPeerBlock("p", blk))
	r.True(ok)
	ok, _ = b.AddBlock(10, newPeerBlock("p", blk))
	r.False(ok)
	r.Len(b.blockQueues[12].blocks, 1)
}

// A single sync request must not be able to make this node read and unicast
// the whole chain from one blockSync worker.
func TestProcessSyncRequestClampsRange(t *testing.T) {
	r := require.New(t)

	var served []uint64
	bs := &blockSyncer{
		cfg:              Config{BufferSize: 200, ProcessSyncRequestTTL: time.Second},
		tipHeightHandler: func() uint64 { return 100000 },
		blockByHeightHandler: func(h uint64) (*block.Block, error) {
			served = append(served, h)
			return blockAtHeight(t, h, 0), nil
		},
		unicastOutbound: func(context.Context, peer.AddrInfo, proto.Message) error { return nil },
	}

	// a peer asks for the entire chain
	r.NoError(bs.ProcessSyncRequest(context.Background(), peer.AddrInfo{}, 1, 1<<62))
	r.Len(served, int(bs.cfg.BufferSize))
	r.Equal(uint64(1), served[0])
	r.Equal(bs.cfg.BufferSize, served[len(served)-1])

	// an honest, in-window request is served in full
	served = nil
	r.NoError(bs.ProcessSyncRequest(context.Background(), peer.AddrInfo{}, 50, 69))
	r.Len(served, 20)
}

// fakeChain commits only blocks it considers authentic, in height order. Any
// other block fails with reject, or with a peer-attributable cause if unset.
type fakeChain struct {
	tip    uint64
	valid  map[hash.Hash256]bool
	reject error
}

func (c *fakeChain) commit(blk *block.Block) error {
	if !c.valid[blk.HashBlock()] || blk.Height() != c.tip+1 {
		if c.reject != nil {
			return c.reject
		}
		return errors.Wrap(block.ErrTxRootMismatch, "invalid block")
	}
	c.tip++
	return nil
}

type syncHarness struct {
	bs       *blockSyncer
	chain    *fakeChain
	requests []*iotexrpc.BlockSync
	blocked  map[string]bool
}

func newSyncHarness(t *testing.T, tip uint64) *syncHarness {
	h := &syncHarness{
		chain:   &fakeChain{tip: tip, valid: map[hash.Hash256]bool{}},
		blocked: map[string]bool{},
	}
	h.bs = &blockSyncer{
		cfg:                Config{BufferSize: 200, IntervalSize: 20, MaxRepeat: 3, RepeatDecayStep: 1},
		buf:                newBlockBuffer(200, 20),
		tipHeightHandler:   func() uint64 { return h.chain.tip },
		commitBlockHandler: h.chain.commit,
		p2pNeighbor: func() ([]peer.AddrInfo, error) {
			return []peer.AddrInfo{{ID: "honest-server"}}, nil
		},
		unicastOutbound: func(_ context.Context, _ peer.AddrInfo, msg proto.Message) error {
			if req, ok := msg.(*iotexrpc.BlockSync); ok {
				h.requests = append(h.requests, req)
			}
			return nil
		},
		blockP2pPeer:    func(pid string) { h.blocked[pid] = true },
		nodeInfoManager: nodeinfo.NewInfoManager(&nodeinfo.DefaultConfig, nil, nil, time.Second),
	}
	return h
}

func (h *syncHarness) authentic(t *testing.T, height uint64) *block.Block {
	blk := blockAtHeight(t, height, 7777)
	h.chain.valid[blk.HashBlock()] = true
	return blk
}

func (h *syncHarness) requested(height uint64) bool {
	for _, r := range h.requests {
		if r.Start <= height && height <= r.End {
			return true
		}
	}
	return false
}

// The authentic block arrives first, then maxBlocksPerHeight
// distinct peers keep rotating unvalidated candidates at the same
// height. The authentic block must survive and the chain must reach it.
func TestBufferAuthenticBlockSurvivesPeerRotation(t *testing.T) {
	r := require.New(t)
	for round := 0; round < 10; round++ {
		h := newSyncHarness(t, 10)
		ctx := context.Background()

		r.NoError(h.bs.ProcessBlock(ctx, "honest", h.authentic(t, 12)))
		nonce := uint64(0)
		for rot := 0; rot < 4; rot++ {
			for p := 0; p < maxBlocksPerHeight; p++ {
				r.NoError(h.bs.ProcessBlock(ctx, "mal"+strconv.Itoa(p), blockAtHeight(t, 12, nonce)))
				nonce++
			}
		}
		r.NoError(h.bs.ProcessBlock(ctx, "honest", h.authentic(t, 11)))
		r.Equal(uint64(12), h.chain.tip, "round %d: tip stuck below the authentic block", round)
	}
}

// Recovery path: the height is full of invalid candidates before the authentic
// block arrives, so the authentic block cannot be buffered. Once the candidates
// fail to commit, the node must re-request the height instead of treating it as
// received, and the re-delivered authentic block must commit.
func TestBufferRecoversHeightAfterCandidatesFail(t *testing.T) {
	r := require.New(t)
	h := newSyncHarness(t, 10)
	ctx := context.Background()

	for p := 0; p < maxBlocksPerHeight; p++ {
		r.NoError(h.bs.ProcessBlock(ctx, "mal"+strconv.Itoa(p), blockAtHeight(t, 12, uint64(p))))
	}
	auth12 := h.authentic(t, 12)
	r.NoError(h.bs.ProcessBlock(ctx, "honest", auth12)) // may be refused while the height is full

	// the height must not be considered complete while candidates were refused
	r.NotEmpty(h.bs.buf.GetBlocksIntervalsToSync(h.chain.tip, 12))

	r.NoError(h.bs.ProcessBlock(ctx, "honest", h.authentic(t, 11)))
	r.Equal(uint64(11), h.chain.tip)
	// every invalid candidate failed and its peer was blocked ...
	for p := 0; p < maxBlocksPerHeight; p++ {
		r.True(h.blocked["mal"+strconv.Itoa(p)])
	}
	// ... and the height was re-requested rather than abandoned
	r.True(h.requested(12), "height 12 must be re-requested after its candidates failed")

	// the re-delivered authentic block commits
	r.NoError(h.bs.ProcessBlock(ctx, "honest-server", auth12))
	r.Equal(uint64(12), h.chain.tip)
}
