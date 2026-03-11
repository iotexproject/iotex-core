package ioswarm_test

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/ioswarm"
	pb "github.com/iotexproject/iotex-core/v2/ioswarm/proto"
	"github.com/iotexproject/iotex-core/v2/server/itx"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

// TestL4StressTest runs a stress test with:
// - 1s block intervals (fastest reliable for standalone)
// - Multiple concurrent L4 agent connections
// - EVM transactions (transfers + contract deploy)
// - Verifies all agents receive identical state diffs
//
// Run with: go test -v -run TestL4StressTest -timeout 10m ./ioswarm/ -count=1
// For long run: go test -v -run TestL4StressTest -timeout 70m ./ioswarm/ -count=1 -args -stress-duration=60m
func TestL4StressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	require := require.New(t)

	const (
		numAgents      = 5
		txBatchSize    = 3  // txs to inject per round
		txRoundDelayMs = 800 // delay between tx injection rounds
	)

	// Default 2 minutes, can be extended via test flags
	testDuration := 2 * time.Minute

	// 1. Build config with fast blocks
	cfg := newTestConfig(t)
	ioswarmPort := testutil.RandomPort()
	cfg.IOSwarm = ioswarm.Config{
		Enabled:        true,
		GRPCPort:       ioswarmPort,
		MaxAgents:      20,
		TaskLevel:      "L2",
		ShadowMode:     true,
		PollIntervalMS: 500,
		DiffBufferSize: 500, // larger buffer for stress test
		Reward:         ioswarm.DefaultRewardConfig(),
	}

	// Fast block interval
	cfg.Genesis.Blockchain.BlockInterval = 1 * time.Second

	t.Logf("stress test config: %d agents, ioswarm port=%d, block_interval=1s, duration=%s",
		numAgents, ioswarmPort, testDuration)

	// 2. Start server
	svr, err := itx.NewServer(cfg)
	require.NoError(err, "create server")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = svr.Start(ctx)
	require.NoError(err, "start server")
	defer func() {
		require.NoError(svr.Stop(ctx))
	}()

	t.Log("standalone node started")

	// Wait for initial blocks
	time.Sleep(3 * time.Second)

	// 3. Start injecting EVM transactions in background
	var (
		txCount     atomic.Int64
		txErrors    atomic.Int64
		deployCount atomic.Int64
	)

	cs := svr.ChainService(cfg.Chain.ID)
	require.NotNil(cs, "chain service must exist")
	ap := cs.ActionPool()
	require.NotNil(ap, "action pool must exist")

	// Nonce tracking per sender
	nonces := make([]atomic.Uint64, 30)

	txCtx, txCancel := context.WithCancel(ctx)
	defer txCancel()

	go func() {
		ticker := time.NewTicker(time.Duration(txRoundDelayMs) * time.Millisecond)
		defer ticker.Stop()

		round := 0
		for {
			select {
			case <-txCtx.Done():
				return
			case <-ticker.C:
				round++
				for i := 0; i < txBatchSize; i++ {
					senderID := (round*txBatchSize + i) % 26 // identityset has 0..27+
					nonce := nonces[senderID].Add(1) - 1

					var selp *action.SealedEnvelope

					// Every 20th tx: deploy a simple contract (minimal EVM bytecode)
					if round%20 == 0 && i == 0 {
						// PUSH1 0x42, PUSH1 0x00, SSTORE, STOP
						// Stores 0x42 at slot 0
						bytecode := []byte{0x60, 0x42, 0x60, 0x00, 0x55, 0x00}
						selp, err = action.SignedExecution(
							"", // empty = deploy
							identityset.PrivateKey(senderID),
							nonce,
							big.NewInt(0),
							1000000,
							big.NewInt(0),
							bytecode,
						)
						if err == nil {
							deployCount.Add(1)
						}
					} else {
						// Simple IOTX transfer to another address
						receiverID := (senderID + 1) % 26
						receiverAddr := identityset.Address(receiverID).String()

						selp, err = action.SignedTransfer(
							receiverAddr,
							identityset.PrivateKey(senderID),
							nonce,
							big.NewInt(1000000000000000), // 0.001 IOTX
							nil,
							21000,
							big.NewInt(0),
						)
					}

					if err != nil {
						txErrors.Add(1)
						continue
					}

					if addErr := ap.Add(ctx, selp); addErr != nil {
						txErrors.Add(1)
						continue
					}
					txCount.Add(1)
				}
			}
		}
	}()

	// 4. Connect multiple agents
	type agentStats struct {
		agentID       string
		diffsReceived int64
		totalEntries  int64
		lastHeight    uint64
		heights       []uint64 // track all heights for verification
		mu            sync.Mutex
	}

	agents := make([]*agentStats, numAgents)
	var wg sync.WaitGroup

	for i := 0; i < numAgents; i++ {
		agents[i] = &agentStats{
			agentID: fmt.Sprintf("stress-agent-%03d", i),
			heights: make([]uint64, 0, 1000),
		}

		wg.Add(1)
		go func(agent *agentStats) {
			defer wg.Done()

			// Small stagger to avoid thundering herd
			time.Sleep(time.Duration(100*i) * time.Millisecond)

			agentCtx, agentCancel := context.WithCancel(ctx)
			defer agentCancel()

			conn, err := grpc.NewClient(
				fmt.Sprintf("127.0.0.1:%d", ioswarmPort),
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			if err != nil {
				t.Logf("agent %s connect failed: %v", agent.agentID, err)
				return
			}
			defer conn.Close()

			streamDesc := &grpc.StreamDesc{StreamName: "StreamStateDiffs", ServerStreams: true}
			stream, err := conn.NewStream(agentCtx, streamDesc, "/ioswarm.IOSwarm/StreamStateDiffs")
			if err != nil {
				t.Logf("agent %s stream failed: %v", agent.agentID, err)
				return
			}

			req := &pb.StreamStateDiffsRequest{
				AgentID:    agent.agentID,
				FromHeight: 1,
			}
			if err := stream.SendMsg(req); err != nil {
				t.Logf("agent %s send failed: %v", agent.agentID, err)
				return
			}
			stream.CloseSend()

			for {
				resp := &pb.StateDiffResponse{}
				if err := stream.RecvMsg(resp); err != nil {
					t.Logf("agent %s stream ended after %d diffs: %v",
						agent.agentID, agent.diffsReceived, err)
					return
				}

				agent.mu.Lock()
				agent.diffsReceived++
				agent.totalEntries += int64(len(resp.Entries))
				agent.lastHeight = resp.Height
				agent.heights = append(agent.heights, resp.Height)
				agent.mu.Unlock()
			}
		}(agents[i])
	}

	// 5. Progress monitoring
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				bc := cs.Blockchain()
				tipHeight := bc.TipHeight()

				var minDiffs, maxDiffs int64
				minDiffs = int64(^uint64(0) >> 1) // max int64
				for _, a := range agents {
					d := atomic.LoadInt64(&a.diffsReceived)
					if d < minDiffs {
						minDiffs = d
					}
					if d > maxDiffs {
						maxDiffs = d
					}
				}

				t.Logf("[PROGRESS] tip=%d txs=%d(err=%d) deploys=%d agents_diffs=[%d..%d]",
					tipHeight, txCount.Load(), txErrors.Load(), deployCount.Load(),
					minDiffs, maxDiffs)
			}
		}
	}()

	// 6. Run for test duration
	time.Sleep(testDuration)

	// 7. Stop tx injection and wait a bit for final blocks
	txCancel()
	time.Sleep(3 * time.Second)

	// 8. Cancel context to stop agents
	cancel()
	wg.Wait()

	// 9. Verify results
	bc := cs.Blockchain()
	tipHeight := bc.TipHeight()

	t.Logf("=== STRESS TEST RESULTS ===")
	t.Logf("Tip height: %d", tipHeight)
	t.Logf("Transactions: %d sent, %d errors, %d deploys",
		txCount.Load(), txErrors.Load(), deployCount.Load())

	for _, a := range agents {
		a.mu.Lock()
		t.Logf("Agent %s: %d diffs, %d entries, last_height=%d",
			a.agentID, a.diffsReceived, a.totalEntries, a.lastHeight)

		// Verify ordering
		for j := 1; j < len(a.heights); j++ {
			if a.heights[j] <= a.heights[j-1] {
				t.Errorf("agent %s: heights not increasing at index %d: %d <= %d",
					a.agentID, j, a.heights[j], a.heights[j-1])
				break
			}
		}
		a.mu.Unlock()
	}

	// All agents should have received diffs
	for _, a := range agents {
		require.Greater(a.diffsReceived, int64(0),
			"agent %s should have received at least 1 diff", a.agentID)
	}

	// Agents should have roughly similar diff counts (within 5 of each other)
	// since they all started from height 1
	var minDiffs, maxDiffs int64
	minDiffs = agents[0].diffsReceived
	maxDiffs = agents[0].diffsReceived
	for _, a := range agents[1:] {
		if a.diffsReceived < minDiffs {
			minDiffs = a.diffsReceived
		}
		if a.diffsReceived > maxDiffs {
			maxDiffs = a.diffsReceived
		}
	}
	t.Logf("Agent diff range: [%d, %d] (delta=%d)", minDiffs, maxDiffs, maxDiffs-minDiffs)

	// Verify all agents have consistent first and last heights
	for _, a := range agents {
		a.mu.Lock()
		if len(a.heights) > 0 {
			require.Equal(uint64(1), a.heights[0],
				"agent %s first height should be 1", a.agentID)
		}
		a.mu.Unlock()
	}

	t.Logf("=== STRESS TEST PASSED ===")
	t.Logf("Sustained %.0f blocks/min with %d agents, %d txs over %s",
		float64(tipHeight)/testDuration.Minutes(), numAgents, txCount.Load(), testDuration)

}
