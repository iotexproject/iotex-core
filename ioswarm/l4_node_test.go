package ioswarm_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/config"
	"github.com/iotexproject/iotex-core/v2/ioswarm"
	pb "github.com/iotexproject/iotex-core/v2/ioswarm/proto"
	"github.com/iotexproject/iotex-core/v2/server/itx"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

// TestL4RealNodeStateDiffs starts a real standalone iotex-core node with IOSwarm enabled,
// lets it produce blocks, and verifies that an L4 agent receives state diffs via gRPC.
//
// This is the ultimate E2E test for the L4 state diff pipeline:
//
//	stateDB.PutBlock() → diffCallback → coordinator.ReceiveStateDiff()
//	  → broadcaster.Publish() → gRPC StreamStateDiffs → agent
func TestL4RealNodeStateDiffs(t *testing.T) {
	require := require.New(t)

	// 1. Build config for standalone node with IOSwarm enabled
	cfg := newTestConfig(t)
	ioswarmPort := testutil.RandomPort()
	cfg.IOSwarm = ioswarm.Config{
		Enabled:        true,
		GRPCPort:       ioswarmPort,
		MaxAgents:      10,
		TaskLevel:      "L2",
		ShadowMode:     true,
		PollIntervalMS: 500,
		DiffBufferSize: 100,
		Reward:         ioswarm.DefaultRewardConfig(),
	}

	t.Logf("starting standalone node with IOSwarm on port %d", ioswarmPort)

	// 2. Create and start server
	svr, err := itx.NewServer(cfg)
	require.NoError(err, "failed to create server")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = svr.Start(ctx)
	require.NoError(err, "failed to start server")
	defer func() {
		require.NoError(svr.Stop(ctx))
	}()

	t.Log("node started, waiting for blocks to be produced...")

	// 3. Wait for a few blocks to be produced
	// Standalone node with TestDefault genesis uses 5s block interval,
	// but it starts producing immediately. Wait 8 seconds for ~2 blocks.
	time.Sleep(8 * time.Second)

	// 4. Connect as L4 agent and open StreamStateDiffs
	agentCtx, agentCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer agentCancel()

	conn, err := grpc.NewClient(
		fmt.Sprintf("127.0.0.1:%d", ioswarmPort),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(err, "agent connect failed")
	defer conn.Close()

	streamDesc := &grpc.StreamDesc{StreamName: "StreamStateDiffs", ServerStreams: true}
	stream, err := conn.NewStream(agentCtx, streamDesc, "/ioswarm.IOSwarm/StreamStateDiffs")
	require.NoError(err, "open stream failed")

	req := &pb.StreamStateDiffsRequest{
		AgentID:    "test-l4-real-node",
		FromHeight: 1,
	}
	err = stream.SendMsg(req)
	require.NoError(err, "send request failed")
	stream.CloseSend()

	// 5. Receive diffs — we should get at least 1 (catch-up from blocks already produced)
	var received []*pb.StateDiffResponse
	totalEntries := 0

	// Try to receive up to 5 diffs (or until timeout)
	for i := 0; i < 5; i++ {
		resp := &pb.StateDiffResponse{}
		if err := stream.RecvMsg(resp); err != nil {
			t.Logf("stream ended after %d diffs: %v", len(received), err)
			break
		}
		received = append(received, resp)
		totalEntries += len(resp.Entries)
		t.Logf("received diff: height=%d entries=%d digest_len=%d",
			resp.Height, len(resp.Entries), len(resp.DigestBytes))
	}

	// 6. Verify we got at least 1 diff
	require.NotEmpty(received, "expected at least 1 state diff from real node")

	// Verify diff properties
	for i, diff := range received {
		require.Greater(diff.Height, uint64(0), "diff %d: height must be > 0", i)
		require.NotEmpty(diff.Entries, "diff %d: entries must not be empty", i)

		// Each entry must have valid namespace
		for j, entry := range diff.Entries {
			require.NotEmpty(entry.Namespace, "diff %d entry %d: namespace must not be empty", i, j)
			require.NotEmpty(entry.Key, "diff %d entry %d: key must not be empty", i, j)
		}
	}

	// Verify ordering
	for i := 1; i < len(received); i++ {
		require.Greater(received[i].Height, received[i-1].Height,
			"heights must be increasing: diff[%d]=%d, diff[%d]=%d",
			i-1, received[i-1].Height, i, received[i].Height)
	}

	t.Logf("SUCCESS: received %d real state diffs with %d total entries from standalone node",
		len(received), totalEntries)
}

// newTestConfig creates a minimal test config for standalone node.
// Follows the exact pattern from server/itx/server_test.go.
func newTestConfig(t *testing.T) config.Config {
	t.Helper()
	require := require.New(t)

	dbPath, err := testutil.PathOfTempFile("chain.db")
	require.NoError(err)
	triePath, err := testutil.PathOfTempFile("trie.db")
	require.NoError(err)
	blobPath, err := testutil.PathOfTempFile("blob.db")
	require.NoError(err)
	contractIndexPath, err := testutil.PathOfTempFile("contractindxer.db")
	require.NoError(err)

	t.Cleanup(func() {
		testutil.CleanupPath(dbPath)
		testutil.CleanupPath(triePath)
		testutil.CleanupPath(blobPath)
		testutil.CleanupPath(contractIndexPath)
	})

	cfg := config.Default
	cfg.Genesis = genesis.TestDefault()
	cfg.API.GRPCPort = testutil.RandomPort()
	cfg.API.HTTPPort = testutil.RandomPort()
	cfg.API.WebSocketPort = testutil.RandomPort()
	cfg.Chain.ChainDBPath = dbPath
	cfg.Chain.TrieDBPath = triePath
	cfg.Chain.BlobStoreDBPath = blobPath
	cfg.Chain.TrieDBPatchFile = ""
	cfg.Chain.ContractStakingIndexDBPath = contractIndexPath
	if cfg.ActPool.Store != nil {
		cfg.ActPool.Store.Datadir = t.TempDir()
	}

	return cfg
}
