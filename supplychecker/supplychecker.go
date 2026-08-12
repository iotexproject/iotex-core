// Copyright (c) 2025 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

// Package supplychecker implements an off-consensus, read-only observer that
// periodically verifies the IOTX total-supply non-increase invariant.
//
// Motivation (audit follow-up on the 2026-08 Harmony "empty block" incident):
// a protocol-level bug let an attacker mint IOTX out of thin air. IoTeX's own
// supply can never exceed the genesis endowment, so a node-side observer that
// recomputes the total circulating supply and compares it against that upper
// bound is a cheap, sound early-warning for the same class of bug.
//
// The observer is deliberately OFF-consensus:
//   - it never rejects blocks nor stops consensus, so it cannot brick the chain;
//   - it only logs and exposes a Prometheus gauge when the invariant is violated;
//   - it does not change any state transition, so it needs no hardfork.
//
// Invariant:  R1 + R2 + R3 <= genesisTotal
//
//	R1 = sum of every primary account balance           (Account namespace)
//	R2 = rewarding fund.totalBalance                    (Rewarding namespace)
//	R3 = staking bucketPool.total.amount                (Staking namespace)
//
// genesisTotal = sum(genesis.InitBalanceMap) + genesis.Rewarding.InitBalanceStr.
//
// The invariant is an upper bound, not an exact equality, because the IOTX
// supply legitimately decreases over time (EIP-1559 base-fee burn, slashing).
// A state can therefore legally have total < genesisTotal, but never above it.
// Missing one of the three accounting reservoirs only makes the computed total
// smaller, which weakens the check but never produces a false alarm, so the
// observer is conservative (sound).
package supplychecker

import (
	"context"
	"math/big"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/rewardingpb"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/stakingpb"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/state"
)

var (
	_supplyMtc = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "iotex_supply_total_rau",
			Help: "Computed IOTX total supply in RAU, split by accounting reservoir",
		},
		[]string{"reservoir"},
	)
	_supplyViolationMtc = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "iotex_supply_above_cap",
			Help: "1 when the computed total supply exceeds the genesis cap (invariant violated), 0 otherwise",
		},
	)
)

func init() {
	prometheus.MustRegister(_supplyMtc)
	prometheus.MustRegister(_supplyViolationMtc)
}

// local deserializable mirrors of on-chain accounting structs. They intentionally
// re-declare the proto schema (not the unexported on-chain types) so the observer
// can decode the raw states stored under each namespace.
type (
	rewardingFund struct {
		totalBalance     *big.Int
		unclaimedBalance *big.Int
	}
	bucketPoolTotal struct {
		amount *big.Int
		count  uint64
	}
)

// Deserialize implements state.Deserializer for the rewarding fund state.
func (f *rewardingFund) Deserialize(data []byte) error {
	gen := rewardingpb.Fund{}
	if err := proto.Unmarshal(data, &gen); err != nil {
		return err
	}
	total, ok := new(big.Int).SetString(gen.TotalBalance, 10)
	if !ok {
		return errors.New("failed to parse rewarding fund total balance")
	}
	unclaimed, ok := new(big.Int).SetString(gen.UnclaimedBalance, 10)
	if !ok {
		return errors.New("failed to parse rewarding fund unclaimed balance")
	}
	f.totalBalance = total
	f.unclaimedBalance = unclaimed
	return nil
}

// Deserialize implements state.Deserializer for the staking bucket pool total.
func (t *bucketPoolTotal) Deserialize(data []byte) error {
	gen := stakingpb.TotalAmount{}
	if err := proto.Unmarshal(data, &gen); err != nil {
		return err
	}
	amount, ok := new(big.Int).SetString(gen.Amount, 10)
	if !ok {
		return errors.New("failed to parse bucket pool total amount")
	}
	t.amount = amount
	t.count = gen.Count
	return nil
}

// Conservation invariants governing the genesis-locked IOTX accounting. The
// three reservoirs must jointly not exceed the genesis endowment.
var (
	_fundKey                            = []byte("fnd")
	_bucketPoolKeyTemplate hash.Hash160 = hash.Hash160b([]byte("bucketPool"))
)

// Observer periodically recomputes the total IOTX supply and reports an
// invariant violation. It is purely observational and safe to run on a live node.
type Observer struct {
	sr     protocol.StateReader
	cap    *big.Int
	window time.Duration
}

// NewObserver returns a supply observer for the given state reader and genesis.
func NewObserver(sr protocol.StateReader, g genesis.Genesis, window time.Duration) *Observer {
	return &Observer{
		sr:     sr,
		cap:    genesisTotalSupply(g),
		window: window,
	}
}

// genesisTotalSupply derives the maximum possible IOTX supply in RAU from the
// genesis configuration: sum of initial account balances plus the rewarding fund
// seed. It is never allowed to increase after chain start.
func genesisTotalSupply(g genesis.Genesis) *big.Int {
	total := big.NewInt(0)
	for _, v := range g.InitBalanceMap {
		n, ok := new(big.Int).SetString(v, 10)
		if !ok {
			// The genesis is validated at startup before this observer runs, so
			// this is unreachable in practice; guard defensively anyway.
			continue
		}
		total.Add(total, n)
	}
	if f, ok := new(big.Int).SetString(g.Rewarding.InitBalanceStr, 10); ok {
		total.Add(total, f)
	}
	return total
}

// sumPrimaryBalances iterates the Account namespace and returns the sum of every
// primary (top-level) account balance, including EVM contract balances.
func sumPrimaryBalances(sr protocol.StateReader) (*big.Int, error) {
	total := big.NewInt(0)
	_, iter, err := sr.States(protocol.NamespaceOption(state.AccountKVNamespace))
	if err != nil {
		return nil, errors.Wrap(err, "failed to enumerate account states")
	}
	for {
		var acc state.Account
		_, err := iter.Next(&acc)
		if errors.Cause(err) == state.ErrOutOfBoundary {
			break
		}
		if errors.Cause(err) == state.ErrNilValue {
			// Deleted / nil-storage account carries no balance; skip it and keep
			// iterating the remaining accounts.
			continue
		}
		if err != nil {
			return nil, errors.Wrap(err, "failed to read account state")
		}
		total.Add(total, acc.Balance)
	}
	return total, nil
}

// readFundTotal reads the rewarding fund total balance from the Rewarding namespace.
func readFundTotal(sr protocol.StateReader) (*big.Int, error) {
	key := append(state.RewardingKeyPrefix[:], _fundKey...)
	var f rewardingFund
	if _, err := sr.State(&f, protocol.NamespaceOption(state.RewardingNamespace), protocol.KeyOption(key)); err != nil {
		return nil, errors.Wrap(err, "failed to read rewarding fund")
	}
	return f.totalBalance, nil
}

// readBucketPoolTotal reads the staking bucket pool total staked amount.
func readBucketPoolTotal(sr protocol.StateReader) (*big.Int, error) {
	key := append([]byte{0x00}, _bucketPoolKeyTemplate[:]...)
	var t bucketPoolTotal
	if _, err := sr.State(&t, protocol.NamespaceOption(state.StakingNamespace), protocol.KeyOption(key)); err != nil {
		return nil, errors.Wrap(err, "failed to read staking bucket pool")
	}
	return t.amount, nil
}

// Check recomputes the current total supply and verifies it does not exceed the
// genesis cap. It returns the observed reservoir sums (for reporting/tests) and
// an error only when the invariant cannot be evaluated (I/O failure). An
// over-cap state is *not* returned as a fatal error; it is logged and exported
// as a metric so the node keeps running and an operator can react, matching the
// off-consensus design.
func (o *Observer) Check(ctx context.Context) (*CheckResult, error) {
	res := &CheckResult{}
	var err error
	if res.Account, err = sumPrimaryBalances(o.sr); err != nil {
		return nil, err
	}
	if res.Fund, err = readFundTotal(o.sr); err != nil {
		return nil, err
	}
	if res.Pool, err = readBucketPoolTotal(o.sr); err != nil {
		return nil, err
	}
	res.Total = new(big.Int).Add(new(big.Int).Add(res.Account, res.Fund), res.Pool)
	res.Cap = new(big.Int).Set(o.cap)

	_supplyMtc.WithLabelValues("account").Set(raiFloat(res.Account))
	_supplyMtc.WithLabelValues("rewarding_fund").Set(raiFloat(res.Fund))
	_supplyMtc.WithLabelValues("staking_pool").Set(raiFloat(res.Pool))
	_supplyMtc.WithLabelValues("total").Set(raiFloat(res.Total))

	if res.Total.Cmp(o.cap) > 0 {
		_supplyViolationMtc.Set(1)
		log.L().Error("IOTX total supply exceeds genesis cap (possible unauthorized mint)",
			zap.String("totalRau", res.Total.String()),
			zap.String("capRau", o.cap.String()),
			zap.String("accountRau", res.Account.String()),
			zap.String("rewardingFundRau", res.Fund.String()),
			zap.String("stakingPoolRau", res.Pool.String()),
		)
		return res, nil
	}
	_supplyViolationMtc.Set(0)
	return res, nil
}

// Run loops Check on the configured interval until ctx is cancelled. Intended to
// be launched as a goroutine from the chain service lifecycle.
func (o *Observer) Run(ctx context.Context) {
	ticker := time.NewTicker(o.window)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := o.Check(ctx); err != nil {
				log.L().Error("failed to run IOTX supply check", zap.Error(err))
			}
		}
	}
}

// CheckResult reports the observed supply breakdown.
type CheckResult struct {
	Account *big.Int // sum of primary account balances
	Fund    *big.Int // rewarding fund total balance
	Pool    *big.Int // staking bucket pool total staked amount
	Total   *big.Int // Account + Fund + Pool
	Cap     *big.Int // genesis cap
}

func raiFloat(v *big.Int) float64 {
	f, _ := new(big.Float).SetInt(v).Float64()
	return f
}
