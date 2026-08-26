// Copyright (c) 2025 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package supplychecker

import (
	"bytes"
	"math/big"
	"sync"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/state/factory"
)

// This file implements the L2 tier of IoTeX's total-supply conservation
// monitoring: an off-consensus, per-block observer that tracks the running total
// supply (R1 + R2 + R3) using the state-diff write queue and asserts that it
// never *increases*.
//
// Invariant (exact, per block, O(block write set)):
//
//	delta(N) = R1(N) - R1(N-1) + R2(N) - R2(N-1) + R3(N) - R3(N-1) <= 0
//
// where
//
//	R1 = sum of every primary account balance           (Account namespace)
//	R2 = rewarding fund.totalBalance                    (Rewarding namespace)
//	R3 = staking bucketPool.total.amount                (Staking namespace)
//
// IOTX has no post-genesis mint path: rewards are paid out of the pre-seeded
// rewarding fund (R2 -> R1), staking moves value between the account and staking
// reservoirs (R1 <-> R3), and the only net outflow is the EIP-1559 base-fee burn
// (and penalty slashing). Every lawful movement therefore conserves or decreases
// the total. A net *increase* in any single block can only come from a balance
// change that no action authorized -- i.e. an ex-nihilo mint -- and is detected
// here at 1-rau precision, without the slack that an upper-bound comparison
// against the genesis cap has (cumulative base-fee burn makes the true supply
// strictly less than the cap, so a small unauthorized mint is invisible to L3's
// loose bound).
//
// This is observability-only:
//   - it only reads the state-diff write queue already produced for other
//     consumers, never rejects blocks and never stops consensus, so it cannot
//     brick the chain;
//   - it does not change any state transition, so it needs no hardfork.
//
// The per-key deltas are computed from the PriorValue / Value pair captured on
// each WriteQueueEntry (see state/factory.CaptureWriteQueue).
//
// Deployment caveats:
//   - The R2 key (rewarding fund) is matched against the v2 (post-Greenland)
//     Rewarding-namespace layout. Pre-Greenland the fund lives in the Account
//     namespace as a legacy state, which this tracker (like the L3 observer)
//     deliberately ignores: it neither counts into R1 (decodeAccount rejects it)
//     nor R2, so on a pre-Greenland-height node reward payouts would appear as a
//     false positive (recipient +X with no visible fund -X). Modern nodes are far
//     past Greenland, so in practice this only matters for full archive replay.
//   - The absolute RunningTotal gauge is seeded at the genesis cap even when the
//     tracker starts on a live node mid-history, so the *gauge* reads high by the
//     base-fee burn accrued before startup. The per-block delta, which is what
//     flags an unauthorized mint, is exact regardless.

var (
	_supplyDeltaMtc = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "iotex_supply_per_block_delta_rau",
			Help: "Net change in IOTX total supply (R1+R2+R3) in RAU for the last committed block; >0 signals a possible unauthorized mint",
		},
	)
	_supplyRunningMtc = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "iotex_supply_running_total_rau",
			Help: "Running IOTX total supply (R1+R2+R3) in RAU maintained from per-block deltas",
		},
	)
)

func init() {
	prometheus.MustRegister(_supplyDeltaMtc)
	prometheus.MustRegister(_supplyRunningMtc)
}

var (
	// recognition keys for the rewarding fund and staking bucket pool states. The
	// rewarding fund lives in the v2 Rewarding namespace at
	// RewardingKeyPrefix+"fnd" (see rewarding.Protocol.putStateV2); the staking
	// bucket pool total is stored at 0x00+Hash160("bucketPool") (see staking.
	// bucket_pool.go). Matching the real on-chain keys is what lets the tracker
	// account R2/R3 deltas correctly from the actual write queue.
	_fundKeyL2       = append(state.RewardingKeyPrefix[:], []byte("fnd")...)
	_bucketPoolKeyL2 = append([]byte{0x00}, _bucketPoolKeyTemplate[:]...)
)

// entryAgg aggregates the write-queue entries for a single (namespace, key): the
// final post-block value and the pre-block (base-store) value.
type entryAgg struct {
	namespace string
	key       []byte
	prior     []byte
	cur       []byte
}

// SupplyTracker is the L2 per-block observer. It is wired to the state factory's
// StateDiffCallback and records the running total-supply plus the per-block
// delta, flagging and logging any single block whose net supply delta is
// positive (unauthorized mint signature). It is safe for concurrent use.
type SupplyTracker struct {
	mu           sync.RWMutex
	seed         *big.Int // genesis total supply (R1+R2+R3 at genesis)
	runningTotal *big.Int
	blocks       uint64
}

// NewSupplyTracker returns a per-block supply tracker seeded at the genesis total
// supply (R1+R2+R3 at genesis == genesisTotalSupply).
func NewSupplyTracker(seed *big.Int) *SupplyTracker {
	seed = new(big.Int).Set(seed)
	return &SupplyTracker{
		seed:         seed,
		runningTotal: big.NewInt(0).Set(seed),
	}
}

// NewSupplyTrackerFromGenesis returns a per-block supply tracker seeded at the
// genesis total supply (R1+R2+R3 at genesis), derived from the genesis config.
func NewSupplyTrackerFromGenesis(g genesis.Genesis) *SupplyTracker {
	return NewSupplyTracker(genesisTotalSupply(g))
}

// OnBlockCommitted is the StateDiffCallback entry point. It consumes the write
// queue of a just-committed block, updates the running total and reports a
// violation when the net supply delta is positive. It never returns an error and
// never blocks consensus; it only logs and updates metrics.
func (t *SupplyTracker) OnBlockCommitted(height uint64, entries []factory.WriteQueueEntry, _ []byte) {
	delta, err := blockSupplyDelta(entries)
	if err != nil {
		log.L().Error("failed to compute per-block supply delta",
			zap.Uint64("height", height), zap.Error(err))
		return
	}

	t.mu.Lock()
	t.runningTotal.Add(t.runningTotal, delta)
	t.blocks++
	positive := delta.Sign() > 0
	if positive {
		_supplyViolationMtc.Set(1)
		log.L().Error("IOTX total supply INCREASED in a single block (possible unauthorized mint)",
			zap.Uint64("height", height),
			zap.String("deltaRau", delta.String()),
			zap.String("runningTotalRau", t.runningTotal.String()),
		)
	} else {
		_supplyViolationMtc.Set(0)
	}
	running := new(big.Int).Set(t.runningTotal)
	t.mu.Unlock()

	_supplyDeltaMtc.Set(raiFloat(delta))
	_supplyRunningMtc.Set(raiFloat(running))
}

// RunningTotal returns the current running total supply in RAU.
func (t *SupplyTracker) RunningTotal() *big.Int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return new(big.Int).Set(t.runningTotal)
}

// Height returns the number of blocks observed.
func (t *SupplyTracker) Height() uint64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.blocks
}

// blockSupplyDelta computes the net change in R1+R2+R3 from a block's write
// queue. Entries are aggregated per (namespace, key): only the final post-block
// value and the pre-block value are used, so a key written multiple times in one
// block is netted exactly once. Non-account legacy payloads lingering in the
// Account namespace (and the per-block height key) are skipped because they
// decode to no primary balance, mirroring L3's sumPrimaryBalances.
func blockSupplyDelta(entries []factory.WriteQueueEntry) (*big.Int, error) {
	delta := big.NewInt(0)

	agg := map[string]*entryAgg{}
	// Aggregate to the last write per (namespace, key).
	var order []string
	for _, e := range entries {
		id := e.Namespace + "\x00" + string(e.Key)
		a, ok := agg[id]
		if !ok {
			a = &entryAgg{namespace: e.Namespace, key: append([]byte(nil), e.Key...),
				prior: append([]byte(nil), e.PriorValue...)}
			agg[id] = a
			order = append(order, id)
		}
		// For delete entries Value is nil; the net new value is zero.
		a.cur = nil
		if e.WriteType == 0 && e.Value != nil {
			a.cur = append([]byte(nil), e.Value...)
		}
	}

	for _, id := range order {
		a := agg[id]
		switch a.namespace {
		case state.AccountKVNamespace:
			if err := applyAccountDelta(delta, a.prior, a.cur); err != nil {
				return nil, err
			}
		case state.RewardingNamespace:
			if bytes.Equal(a.key, _fundKeyL2) {
				if err := applyFundDelta(delta, a.prior, a.cur); err != nil {
					return nil, err
				}
			}
		case state.StakingNamespace:
			if bytes.Equal(a.key, _bucketPoolKeyL2) {
				if err := applyPoolDelta(delta, a.prior, a.cur); err != nil {
					return nil, err
				}
			}
		}
	}
	return delta, nil
}

// accountBalance decodes a raw Account-namespace payload to its balance, or 0 if
// it is not a genuine primary account (legacy Poll/Rewarding states, the height
// key, deleted/nil, EVM contract accounts all decode via the strict decoder).
func accountBalance(data []byte) *big.Int {
	if acct, ok := decodeAccount(data); ok {
		if b, ok := new(big.Int).SetString(acct.GetBalance(), 10); ok {
			return b
		}
	}
	return big.NewInt(0)
}

func applyAccountDelta(delta *big.Int, prior, cur []byte) error {
	priorBal := accountBalance(prior)
	curBal := accountBalance(cur)
	delta.Add(delta, new(big.Int).Sub(curBal, priorBal))
	return nil
}

func applyFundDelta(delta *big.Int, prior, cur []byte) error {
	priorTotal, err := fundTotal(prior)
	if err != nil {
		return err
	}
	curTotal, err := fundTotal(cur)
	if err != nil {
		return err
	}
	delta.Add(delta, new(big.Int).Sub(curTotal, priorTotal))
	return nil
}

func applyPoolDelta(delta *big.Int, prior, cur []byte) error {
	priorTotal, err := poolTotal(prior)
	if err != nil {
		return err
	}
	curTotal, err := poolTotal(cur)
	if err != nil {
		return err
	}
	delta.Add(delta, new(big.Int).Sub(curTotal, priorTotal))
	return nil
}

// fundTotal decodes a rewarding-fund state's totalBalance, treating a nil/missing
// payload as zero.
func fundTotal(data []byte) (*big.Int, error) {
	if len(data) == 0 {
		return big.NewInt(0), nil
	}
	var f rewardingFund
	if err := f.Deserialize(data); err != nil {
		return nil, errors.Wrap(err, "failed to decode rewarding fund in supply delta")
	}
	return f.totalBalance, nil
}

// poolTotal decodes a staking bucket-pool total's amount, treating a nil/missing
// payload as zero.
func poolTotal(data []byte) (*big.Int, error) {
	if len(data) == 0 {
		return big.NewInt(0), nil
	}
	var t bucketPoolTotal
	if err := t.Deserialize(data); err != nil {
		return nil, errors.Wrap(err, "failed to decode bucket pool total in supply delta")
	}
	return t.amount, nil
}
