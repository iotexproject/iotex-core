// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package e2etest

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/rewardingpb"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
)

// readPendingVoterRewardDelegates asks the rewarding protocol for the delegate
// list at height, through the archive state reader the API uses for a historical
// eth_call.
func readPendingVoterRewardDelegates(
	t *testing.T,
	test *e2etest,
	g genesis.Genesis,
	p *rewarding.Protocol,
	height uint64,
) [][]byte {
	t.Helper()
	r := require.New(t)
	// Same context the API assembles for a historical read: the erigon store
	// needs the blockchain context at that height to build its contract backend.
	ctx := protocol.WithRegistry(context.Background(), test.cs.Registry())
	ctx = genesis.WithGenesisContext(ctx, g)
	ctx, err := test.cs.Blockchain().ContextAtHeight(ctx, height)
	r.NoErrorf(err, "context at height %d", height)
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
		BlockHeight:    bcCtx.Tip.Height,
		BlockTimeStamp: bcCtx.Tip.Timestamp,
	})
	ctx = protocol.WithFeatureCtx(ctx)

	ws, err := test.cs.StateFactory().WorkingSetAtHeight(ctx, height)
	r.NoErrorf(err, "working set at height %d", height)
	defer ws.Close()

	data, _, err := p.ReadState(ctx, ws, []byte("PendingVoterRewardDelegates"))
	r.NoErrorf(err, "PendingVoterRewardDelegates at height %d", height)
	decoded := &rewardingpb.PendingVoterRewardDelegates{}
	r.NoError(proto.Unmarshal(data, decoded))
	return decoded.GetDelegateIdentifiers()
}

// TestIIP59PendingVoterRewardDelegatesArchiveRead pins that the delegate list is
// readable at a past height on an archive node.
//
// The archive reader serves history out of erigon, whose objects are addressed
// by contract slot rather than by an ordered key space, so it refuses the
// ordered range scan the enumeration is built on. Every other IIP-59 read is a
// point read and was unaffected; this one used to fail outright with "erigon
// store does not support ordered range scan".
//
// The claim under test is not just "does not error": the answer at a height must
// still be the answer that height had, so the test freezes an expectation while
// that height is the tip and re-reads it after the chain has moved past it.
func TestIIP59PendingVoterRewardDelegatesArchiveRead(t *testing.T) {
	r := require.New(t)
	tier := iip59PerfTiers["small"]
	cfg := newIIP59PerfCfg(r, tier)
	historyIndexPath, err := os.MkdirTemp("", "historyindex")
	r.NoError(err)
	cfg.Chain.HistoryIndexPath = historyIndexPath
	defer clearDBPaths(&cfg)

	test := newE2ETest(t, cfg, iip59PerfBuildOptions(t, tier, cfg.Genesis)...)
	defer test.teardown()
	registerEpochProtocols(r, test)
	registerIIP59EraFreezer(r, test, tier)

	rewardProto := rewarding.FindProtocol(test.cs.Registry())
	r.NotNil(rewardProto)
	bc := test.cs.Blockchain()
	ap := test.cs.ActionPool()

	blkTime := time.Unix(cfg.Genesis.Timestamp, 0)
	mint := func() uint64 {
		blkTime = blkTime.Add(time.Second)
		_, err := mintOne(bc, ap, blkTime)
		r.NoErrorf(err, "mint at height %d", bc.TipHeight())
		return bc.TipHeight()
	}

	// Block rewards credit a pending pool per producing delegate, so a handful
	// of blocks is enough to make the list non-empty.
	var (
		observedHeight uint64
		observed       [][]byte
	)
	for i := 0; i < 12; i++ {
		observedHeight = mint()
		observed = readPendingVoterRewardDelegates(t, test, cfg.Genesis, rewardProto, observedHeight)
		if len(observed) > 0 {
			break
		}
	}
	r.NotEmptyf(observed, "fixture must accrue at least one pending pool by height %d", observedHeight)

	// The state factory itself still answers the tip from the statedb, where the
	// ordered range scan works. Pinning the archive answer against it at the same
	// height is what makes this more than a smoke test.
	ctx := protocol.WithRegistry(context.Background(), test.cs.Registry())
	ctx = genesis.WithGenesisContext(ctx, cfg.Genesis)
	ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{BlockHeight: observedHeight})
	ctx = protocol.WithFeatureCtx(ctx)
	scanned, _, err := rewardProto.ReadState(
		ctx, test.cs.StateFactory(), []byte("PendingVoterRewardDelegates"))
	r.NoError(err)
	viaScan := &rewardingpb.PendingVoterRewardDelegates{}
	r.NoError(proto.Unmarshal(scanned, viaScan))
	r.Equal(viaScan.GetDelegateIdentifiers(), observed,
		"archive read at the tip must match the range scan over the same state")

	for i := 0; i < 10; i++ {
		mint()
	}
	r.Greater(bc.TipHeight(), observedHeight)

	historical := readPendingVoterRewardDelegates(t, test, cfg.Genesis, rewardProto, observedHeight)
	r.Equal(observed, historical,
		"reading height %d after the chain moved on must return what that height held", observedHeight)
}
