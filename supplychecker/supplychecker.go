// Copyright (c) 2025 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

// Package supplychecker implements the L3 tier of IoTeX's total-supply
// conservation monitoring: an off-consensus, read-only observer that
// periodically verifies the IOTX total-supply non-increase invariant.
//
// Motivation (audit follow-up on the 2026-08 Harmony "empty block" incident):
// a protocol-level bug let an attacker mint IOTX out of thin air. IoTeX's own
// supply can never exceed the genesis endowment, so a node-side observer that
// recomputes the total circulating supply and compares it against that upper
// bound is a cheap early-warning for the same class of bug.
//
// This is the L3 (periodic reconciliation) tier, NOT a per-block check. Because
// it walks the entire Account namespace, it must never run every minute on every
// validator: a full-namespace scan holds the state-factory read lock and would
// stall block commits (liveness). It is therefore opt-in, driven by
// blockchain.SupplyCheckConfig (default disabled), and meant to run daily / per
// epoch on a dedicated auditor node.
//
// The observer is deliberately OFF-consensus and non-fatal:
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
//
// The Account namespace also holds legacy Poll/Rewarding states that pre-date
// the Greenland storage layout and are never deleted; feeding them to
// state.Account.Deserialize would panic and over-count bogus balances. This
// observer decodes them with a strict, non-panicking decoder (decodeAccount)
// that skips anything that is not a well-formed account, so the scan can neither
// crash the node nor produce a false positive from those legacy payloads.
package supplychecker

import (
	"bytes"
	"context"
	"math/big"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/account/accountpb"
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
	total, err := parseDecimal(gen.TotalBalance, "rewarding fund total balance")
	if err != nil {
		return err
	}
	unclaimed, err := parseDecimal(gen.UnclaimedBalance, "rewarding fund unclaimed balance")
	if err != nil {
		return err
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
	amount, err := parseDecimal(gen.Amount, "bucket pool total amount")
	if err != nil {
		return err
	}
	t.amount = amount
	t.count = gen.Count
	return nil
}

// parseDecimal converts a decimal-string field into a big.Int, reporting a
// stable error that names the on-chain field when the string is malformed.
func parseDecimal(s, field string) (*big.Int, error) {
	n, ok := new(big.Int).SetString(s, 10)
	if !ok {
		return nil, errors.Errorf("failed to parse %s", field)
	}
	return n, nil
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

// rawAccountState is a state.Deserializer that captures the raw serialized bytes
// of an entry in the Account namespace without decoding them through
// state.Account. state.Account.Deserialize is written for trusted input and
// panics on legacy Poll/Rewarding payloads (unknown account type / invalid
// balance, state/account.go) that were never removed from this namespace; this
// observer must never crash the node, so it defers to decodeAccount.
type rawAccountState []byte

// Deserialize implements state.Deserializer by capturing the raw bytes.
func (r *rawAccountState) Deserialize(data []byte) error {
	*r = append((*r)[:0], data...)
	return nil
}

// decodeAccount parses a serialized entry from the Account namespace WITHOUT
// panicking. The Account namespace historically also stored legacy Poll and
// Rewarding states (pre-Greenland) that are never deleted, and
// state.Account.Deserialize is written for trusted input and panics on them.
// This local decoder rejects anything that is not a genuine, canonically-encoded
// account:
//   - payloads carrying unknown proto fields (wire-collision artifacts of legacy
//     Fund / Admin / exempt / rewardAccount / CandidatesList, whose tags overlap
//     accountpb.Account and otherwise decode into bogus balances / types);
//   - an AccountType enum outside {DEFAULT, ZERO_NONCE} (e.g.
//     Admin.productivityThreshold colliding with field 7 would panic the real
//     decoder);
//   - a balance that is not a decimal integer;
//   - any non-canonical wire encoding (guards against field-layout drift).
//
// It returns (account, true) on success, or (nil, false) to skip the entry.
func decodeAccount(data []byte) (*accountpb.Account, bool) {
	if len(data) == 0 {
		return nil, false
	}
	acct := &accountpb.Account{}
	if err := proto.Unmarshal(data, acct); err != nil {
		return nil, false
	}
	// Reject unknown proto fields: they signal a different message type whose
	// field tags collided with accountpb.Account.
	if len(acct.ProtoReflect().GetUnknown()) > 0 {
		return nil, false
	}
	// The account-type enum only admits {0,1}. Anything else is a wire collision
	// with another message (e.g. rewardingpb.Admin.productivityThreshold -> field 7)
	// that would make state.Account.FromProto panic.
	switch acct.GetType() {
	case accountpb.AccountType_DEFAULT, accountpb.AccountType_ZERO_NONCE:
	default:
		return nil, false
	}
	// Balance must parse as a decimal integer (state.Account.FromProto panics on
	// a malformed balance, e.g. a 1e26 scientific-notation field from a legacy
	// Fund state).
	if acct.GetBalance() != "" {
		if _, ok := new(big.Int).SetString(acct.GetBalance(), 10); !ok {
			return nil, false
		}
	}
	// Canonical round-trip: a genuine account is the proto.Marshal of its own
	// canonical form, so re-marshaling the parsed message must reproduce the
	// input exactly. Any drift means the payload is not a plain account.
	canonical, err := proto.Marshal(acct)
	if err != nil || !bytes.Equal(canonical, data) {
		return nil, false
	}
	return acct, true
}

// sumPrimaryBalances iterates the Account namespace and returns the sum of every
// genuine primary (top-level) account balance, including EVM contract balances.
// Entries that are not well-formed accounts (legacy Poll/Rewarding states
// lingering in this namespace) are skipped, so the scan can never panic on them.
func sumPrimaryBalances(sr protocol.StateReader) (*big.Int, uint64, error) {
	total := big.NewInt(0)
	var accounts uint64
	_, iter, err := sr.States(protocol.NamespaceOption(state.AccountKVNamespace))
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to enumerate account states")
	}
	for {
		var raw rawAccountState
		_, err := iter.Next(&raw)
		if errors.Cause(err) == state.ErrOutOfBoundary {
			break
		}
		if errors.Cause(err) == state.ErrNilValue {
			// Deleted / nil-storage entry carries no balance; skip it.
			continue
		}
		if err != nil {
			return nil, 0, errors.Wrap(err, "failed to read account state")
		}
		acct, ok := decodeAccount([]byte(raw))
		if !ok {
			// Not a genuine account (legacy Poll/Rewarding state). Skipping it is
			// the correct behavior: it is not a primary account balance reservoir.
			continue
		}
		bal, _ := new(big.Int).SetString(acct.GetBalance(), 10)
		total.Add(total, bal)
		accounts++
	}
	return total, accounts, nil
}

// readFundTotal reads the rewarding fund total balance from the Rewarding namespace.
// A missing fund state is treated as a zero balance: this is the same conservative
// direction as every other under-count (it weakens the check but never produces a
// false alarm), and it keeps the observer usable on nodes whose current height
// predates the v2-storage layout that stores this state.
func readFundTotal(sr protocol.StateReader) (*big.Int, error) {
	key := append(state.RewardingKeyPrefix[:], _fundKey...)
	var f rewardingFund
	if _, err := sr.State(&f, protocol.NamespaceOption(state.RewardingNamespace), protocol.KeyOption(key)); err != nil {
		if errors.Cause(err) == state.ErrStateNotExist {
			return big.NewInt(0), nil
		}
		return nil, errors.Wrap(err, "failed to read rewarding fund")
	}
	return f.totalBalance, nil
}

// readBucketPoolTotal reads the staking bucket pool total staked amount. Like the
// rewarding fund, a missing pool state (pre-Greenland layout) is treated as zero.
func readBucketPoolTotal(sr protocol.StateReader) (*big.Int, error) {
	key := append([]byte{0x00}, _bucketPoolKeyTemplate[:]...)
	var t bucketPoolTotal
	if _, err := sr.State(&t, protocol.NamespaceOption(state.StakingNamespace), protocol.KeyOption(key)); err != nil {
		if errors.Cause(err) == state.ErrStateNotExist {
			return big.NewInt(0), nil
		}
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
	if res.Account, res.Accounts, err = sumPrimaryBalances(o.sr); err != nil {
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
	Account  *big.Int // sum of primary account balances
	Accounts uint64   // number of genuine accounts summed (skips legacy states)
	Fund     *big.Int // rewarding fund total balance
	Pool     *big.Int // staking bucket pool total staked amount
	Total    *big.Int // Account + Fund + Pool
	Cap      *big.Int // genesis cap
}

func raiFloat(v *big.Int) float64 {
	f, _ := new(big.Float).SetInt(v).Float64()
	return f
}
