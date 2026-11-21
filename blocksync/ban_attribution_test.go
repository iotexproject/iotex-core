// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blocksync

import (
	"context"
	"io"
	"strconv"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/blockchain"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/v2/blockchain/filedao"
)

// newBanRecorder returns a blockSyncer whose commit handler always fails with
// commitErr, and the list of peers it blocks.
func newBanRecorder(commitErr error) (*blockSyncer, *[]string) {
	banned := []string{}
	return &blockSyncer{
		commitBlockHandler: func(*block.Block) error { return commitErr },
		blockP2pPeer:       func(pid string) { banned = append(banned, pid) },
	}, &banned
}

// A peer is blocked only when the commit failure proves it served a bad block.
// Local faults, causes that local state or older rules can produce, and causes
// nobody has classified must not block the peer that served the block.
func TestCommitBlocksBlocksOnlyAttributablePeers(t *testing.T) {
	for _, c := range []struct {
		name     string
		err      error
		ban      bool
		rejected bool
	}{
		// peer-attributable: the peer is blocked
		{"tx root mismatch", block.ErrTxRootMismatch, true, true},
		{"wrapped tx root mismatch", errors.Wrap(block.ErrTxRootMismatch, "failed to validate block"), true, true},

		// local faults
		{"disk full", errors.New("write /data/chain.db: no space left on device"), false, true},
		{"wrapped io error", errors.Wrap(io.ErrUnexpectedEOF, "failed to read block index"), false, true},
		{"block store height race", filedao.ErrInvalidTipHeight, false, true},
		// unknown causes, e.g. a node on older rules
		{"unknown cause", errors.New("failed to validate block: unknown action type 42"), false, true},
		// sentinels that local state or rule skew can produce for a valid block
		{"wrong prev hash", errors.Wrap(blockchain.ErrInvalidBlock, "wrong prev hash"), false, true},
		{"delta state mismatch", errors.Wrap(block.ErrDeltaStateMismatch, "digest"), false, true},
		{"receipt root mismatch", errors.Wrap(block.ErrReceiptRootMismatch, "receipt root"), false, true},
		{"gas used mismatch", errors.Wrap(block.ErrGasUsedMismatch, "gas used"), false, true},
		{"blob gas used mismatch", errors.Wrap(block.ErrBlobGasUsedMismatch, "blob gas used"), false, true},
		{"action nonce", errors.Wrap(blockchain.ErrActionNonce, "nonce"), false, true},
		{"balance", errors.Wrap(blockchain.ErrBalance, "balance"), false, true},
		{"insufficient gas", errors.Wrap(blockchain.ErrInsufficientGas, "gas"), false, true},

		// benign: neither blocked nor counted as rejected
		{"remote height too low", blockdao.ErrRemoteHeightTooLow, false, false},
		{"paused", errors.Wrapf(blockchain.ErrPaused, "cannot commit block %d", 11), false, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := require.New(t)
			bs, banned := newBanRecorder(c.err)
			committed, rejected := bs.commitBlocks([]*peerBlock{newPeerBlock("peer", blockAtHeight(t, 11, 0))})
			r.False(committed)
			r.Equal(c.rejected, rejected)
			if c.ban {
				r.Equal([]string{"peer"}, *banned)
			} else {
				r.Empty(*banned)
			}
		})
	}
}

// Pop returns every candidate buffered at a height, so one local fault used to
// block every peer that served it, up to the whole peer set. It must block none.
func TestCommitBlocksLocalFaultFanOut(t *testing.T) {
	localFault := errors.New("write /data/chain.db: no space left on device")
	for _, n := range []int{1, 5, 10, 30} {
		t.Run(strconv.Itoa(n), func(t *testing.T) {
			r := require.New(t)
			blks := make([]*peerBlock, 0, n)
			for i := range n {
				blks = append(blks, newPeerBlock("peer"+strconv.Itoa(i), blockAtHeight(t, 11, uint64(i))))
			}

			bs, banned := newBanRecorder(localFault)
			committed, rejected := bs.commitBlocks(blks)
			r.False(committed)
			r.True(rejected)
			r.Empty(*banned, "a local fault must not block any of the %d peers", n)

			// control: candidates that are provably bad still block every peer
			bs, banned = newBanRecorder(block.ErrTxRootMismatch)
			bs.commitBlocks(blks)
			r.Len(*banned, n)
		})
	}
}

// The lossy-height recovery must not depend on blocking: when the candidates
// that filled a height fail for a cause that is not attributable to their
// peers, nobody is blocked, but the height is still re-requested and the
// authentic block commits.
func TestBufferRecoversHeightAfterUnattributedFailures(t *testing.T) {
	r := require.New(t)
	h := newSyncHarness(t, 10)
	h.chain.reject = errors.New("failed to validate block: unknown action type 42")
	ctx := context.Background()

	for p := 0; p < maxBlocksPerHeight; p++ {
		r.NoError(h.bs.ProcessBlock(ctx, "peer"+strconv.Itoa(p), blockAtHeight(t, 12, uint64(p))))
	}
	auth12 := h.authentic(t, 12)
	r.NoError(h.bs.ProcessBlock(ctx, "honest", auth12)) // may be refused while the height is full

	r.NoError(h.bs.ProcessBlock(ctx, "honest", h.authentic(t, 11)))
	r.Equal(uint64(11), h.chain.tip)
	r.Empty(h.blocked)
	r.True(h.requested(12), "height 12 must be re-requested after its candidates failed")

	r.NoError(h.bs.ProcessBlock(ctx, "honest-server", auth12))
	r.Equal(uint64(12), h.chain.tip)
}
