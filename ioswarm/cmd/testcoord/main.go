// testcoord runs an IOSwarm coordinator with mock data for E2E testing.
// Usage: go run ./cmd/testcoord
//
// It generates fake pending transactions every 2 seconds and dispatches
// them to connected agents. Logs everything to stdout.
package main

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"flag"
	"fmt"
	"math/big"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/iotexproject/iotex-core/v2/ioswarm"
	pb "github.com/iotexproject/iotex-core/v2/ioswarm/proto"
	"go.uber.org/zap"
)

// mockActPool simulates iotex-core's actpool with fake transactions.
type mockActPool struct {
	mu        sync.Mutex
	txs       []*ioswarm.PendingTx
	height    atomic.Uint64
	txCount   atomic.Uint64
	taskLevel string
}

func (m *mockActPool) PendingActions() []*ioswarm.PendingTx {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]*ioswarm.PendingTx, len(m.txs))
	copy(cp, m.txs)
	return cp
}

func (m *mockActPool) BlockHeight() uint64 {
	return m.height.Load()
}

func (m *mockActPool) generateBatch(count int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	senders := []string{"io1alice", "io1bob", "io1carol", "io1dave", "io1eve"}
	receivers := []string{"io1frank", "io1grace", "io1heidi", "io1ivan", "io1judy"}

	m.txs = make([]*ioswarm.PendingTx, 0, count)
	for i := 0; i < count; i++ {
		n := m.txCount.Add(1)

		// L3 mode: 30% contract calls, 70% simple transfers
		if m.taskLevel == "L3" && len(testContracts) > 0 && int(n)%10 < 3 {
			tc := testContracts[int(n)%len(testContracts)]
			m.txs = append(m.txs, &ioswarm.PendingTx{
				Hash:     fmt.Sprintf("tx-%d", n),
				From:     senders[int(n)%len(senders)],
				To:       tc.Address,
				Nonce:    n,
				Amount:   "0",
				GasLimit: 100000,
				GasPrice: "1000000000",
				Data:     incrementSelector, // call increment()
				RawBytes: buildMockTxRaw(n),
			})
		} else {
			m.txs = append(m.txs, &ioswarm.PendingTx{
				Hash:     fmt.Sprintf("tx-%d", n),
				From:     senders[int(n)%len(senders)],
				To:       receivers[int(n)%len(receivers)],
				Nonce:    n,
				Amount:   "1000000000000000000",
				GasLimit: 21000,
				GasPrice: "1000000000",
				RawBytes: buildMockTxRaw(n),
			})
		}
	}

	m.height.Add(1)
}

// mockState simulates iotex-core's statedb with L3 support.
type mockState struct {
	codes   map[string][]byte            // address → bytecode
	storage map[string]map[string]string  // address → slot → value
}

func newMockState() *mockState {
	ms := &mockState{
		codes:   make(map[string][]byte),
		storage: make(map[string]map[string]string),
	}
	// Deploy test contracts
	for _, tc := range testContracts {
		ms.codes[tc.Address] = tc.Code
		ms.storage[tc.Address] = map[string]string{
			"0x0000000000000000000000000000000000000000000000000000000000000000": tc.InitSlot,
		}
	}
	return ms
}

func (m *mockState) AccountState(address string) (*pb.AccountSnapshot, error) {
	snap := &pb.AccountSnapshot{
		Address: address,
		Balance: "100000000000000000000", // 100 IOTX
		Nonce:   0,
	}
	// Mark contracts with CodeHash
	if _, ok := m.codes[address]; ok {
		snap.CodeHash = []byte{0x01} // non-empty = has code
	}
	return snap, nil
}

func (m *mockState) GetCode(address string) ([]byte, error) {
	return m.codes[address], nil
}

func (m *mockState) GetStorageAt(address, slot string) (string, error) {
	if slots, ok := m.storage[address]; ok {
		if val, ok := slots[slot]; ok {
			return val, nil
		}
	}
	return "0x0000000000000000000000000000000000000000000000000000000000000000", nil
}

func (m *mockState) SimulateAccessList(from, to string, data []byte, value string, gasLimit uint64) (map[string][]string, error) {
	return nil, nil
}

// secp256k1N is the order of the secp256k1 curve used by IoTeX/Ethereum signatures.
var secp256k1N, _ = new(big.Int).SetString("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141", 16)

// buildMockTxRaw creates a mock transaction with nonce in the first 8 bytes
// and valid secp256k1 signature components in the last 65 bytes.
func buildMockTxRaw(nonce uint64) []byte {
	payload := make([]byte, 80)
	// Encode nonce in first 8 bytes (big-endian)
	binary.BigEndian.PutUint64(payload[:8], nonce)
	rand.Read(payload[8:])
	r, _ := rand.Int(rand.Reader, secp256k1N)
	s, _ := rand.Int(rand.Reader, secp256k1N)
	raw := append(payload, r.FillBytes(make([]byte, 32))...)
	raw = append(raw, s.FillBytes(make([]byte, 32))...)
	raw = append(raw, byte(0))
	return raw
}

func main() {
	masterSecret := flag.String("master-secret", "", "HMAC master secret for agent auth (empty = no auth)")
	delegateAddr := flag.String("delegate-address", "", "delegate IOTX address for reward payout")
	port := flag.Int("port", 14689, "gRPC listen port")
	taskLevel := flag.String("task-level", "L2", "task level: L1, L2, L3")
	flag.Parse()

	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	cfg := ioswarm.Config{
		Enabled:        true,
		GRPCPort:       *port,
		SwarmAPIPort:   *port + 1,
		MaxAgents:      100,
		TaskLevel:      *taskLevel,
		ShadowMode:     true,
		PollIntervalMS:  2000, // dispatch every 2s
		MasterSecret:    *masterSecret,
		DelegateAddress: *delegateAddr,
		Reward: ioswarm.RewardConfig{
			DelegateCutPct:    10,
			EpochBlocks:       3, // 3 × 10s = 30s epochs for demo
			MinTasksForReward: 5,
			BonusAccuracyPct:  99.5,
			BonusMultiplier:   1.2,
		},
	}

	actPool := &mockActPool{taskLevel: *taskLevel}
	actPool.height.Store(1000)
	stateDB := newMockState()

	coord := ioswarm.NewCoordinator(cfg, actPool, stateDB, ioswarm.WithLogger(logger))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := coord.Start(ctx); err != nil {
		logger.Fatal("failed to start coordinator", zap.Error(err))
	}

	logger.Info("=== IOSwarm Test Coordinator ===",
		zap.Int("port", *port),
		zap.String("task_level", cfg.TaskLevel),
		zap.Bool("shadow_mode", cfg.ShadowMode),
		zap.Bool("auth", *masterSecret != ""))
	logger.Info("Waiting for agents to connect...")
	logger.Info(fmt.Sprintf("Run agent with: ./ioswarm-agent --coordinator=localhost:%d --level=%s", *port, *taskLevel))

	// Generate mock transactions periodically
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		batch := 0
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				batch++
				count := 3 + (batch % 8) // 3-10 txs per batch
				actPool.generateBatch(count)
				logger.Info("generated mock transactions",
					zap.Int("batch", batch),
					zap.Int("tx_count", count),
					zap.Uint64("block_height", actPool.BlockHeight()))
			}
		}
	}()

	// Shadow mode: simulate block execution every 5 seconds.
	// Treats all dispatched tasks as valid (mock scenario).
	// Also runs EVM shadow comparison for L3 tasks.
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		var lastTaskID uint32
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				currentID := coord.LastTaskID()
				if currentID <= lastTaskID {
					continue
				}
				actualResults := make(map[uint32]bool)
				for id := lastTaskID + 1; id <= currentID; id++ {
					actualResults[id] = true
				}
				blockHeight := actPool.BlockHeight()
				coord.OnBlockExecuted(blockHeight, actualResults)

				// EVM shadow comparison: compare agent gas/state results
				// against expected values for known task types
				if *taskLevel == "L3" {
					agentResults := coord.DrainRecentResults()
					if len(agentResults) > 0 {
						evmActual := make(map[uint32]*ioswarm.EVMActualResult)
						for _, r := range agentResults {
							if r.GasUsed > 0 {
								evmActual[r.TaskID] = &ioswarm.EVMActualResult{
									TaskID:       r.TaskID,
									GasUsed:      r.GasUsed,
									StateChanges: r.StateChanges,
									ExecError:    r.ExecError,
								}
							}
						}
						if len(evmActual) > 0 {
							coord.CompareEVMShadow(agentResults, evmActual, blockHeight)
						}
					}
				}

				lastTaskID = currentID
			}
		}
	}()

	// Wait for signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	logger.Info("shutting down...")
	cancel()
	coord.Stop()
}
