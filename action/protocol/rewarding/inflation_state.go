// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"context"
	"math/big"

	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/rewardingpb"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/systemcontracts"
)

// _inflKey is the rewarding-namespace key for the IIP-62 InflationState. Mirrors
// the style of the existing single-record keys in protocol.go.
var _inflKey = []byte("inf")

// inflationState is the in-memory mirror of rewardingpb.InflationState. Mirrors
// the fund / admin / rewardAccount serialization pattern in this package.
// inflationState holds only the IIP-62 fields that change at year boundaries.
// PostActivationMinted, EpochRemainderAccumulator, and YearMintRemainder used to live
// here but were all deterministic per-block values; they are now derived on demand
// (CumulativeMinted / EpochInflationSurplus / PerBlockMint) so this record is written
// once per year instead of every block.
type inflationState struct {
	outstandingSupplyAtYearStart *big.Int
	currentInflationBps          uint64
	currentYearIndex             uint64
}

func newInflationState() *inflationState {
	return &inflationState{
		outstandingSupplyAtYearStart: new(big.Int),
	}
}

// Serialize encodes the inflation state into bytes.
func (s inflationState) Serialize() ([]byte, error) {
	return proto.Marshal(s.toProto())
}

// Deserialize decodes bytes into the inflation state.
func (s *inflationState) Deserialize(data []byte) error {
	gen := rewardingpb.InflationState{}
	if err := proto.Unmarshal(data, &gen); err != nil {
		return err
	}
	return s.fromProto(&gen)
}

// Encode satisfies the systemcontracts.GenericValueContainer interface so the
// state can ride the erigon storage path used by Fund / Admin.
func (s *inflationState) Encode() (systemcontracts.GenericValue, error) {
	d, err := proto.Marshal(s.toProto())
	if err != nil {
		return systemcontracts.GenericValue{}, err
	}
	return systemcontracts.GenericValue{PrimaryData: d}, nil
}

// Decode is the inverse of Encode.
func (s *inflationState) Decode(v systemcontracts.GenericValue) error {
	gen := rewardingpb.InflationState{}
	if err := proto.Unmarshal(v.PrimaryData, &gen); err != nil {
		return err
	}
	return s.fromProto(&gen)
}

func (s *inflationState) toProto() *rewardingpb.InflationState {
	return &rewardingpb.InflationState{
		OutstandingSupplyAtYearStart: bigToStr(s.outstandingSupplyAtYearStart),
		CurrentInflationBps:          s.currentInflationBps,
		CurrentYearIndex:             s.currentYearIndex,
	}
}

func (s *inflationState) fromProto(gen *rewardingpb.InflationState) error {
	var err error
	if s.outstandingSupplyAtYearStart, err = strToBig(gen.OutstandingSupplyAtYearStart, "outstandingSupplyAtYearStart"); err != nil {
		return err
	}
	s.currentInflationBps = gen.CurrentInflationBps
	s.currentYearIndex = gen.CurrentYearIndex
	return nil
}

// initInflationState seeds the IIP-62 InflationState at activation height. Called
// from CreatePreStates exactly once. Validates genesis inflation params and the
// Machina DAO address; panics on misconfiguration since this is a deployment-time
// invariant, not a per-block condition.
func (p *Protocol) initInflationState(ctx context.Context, sm protocol.StateManager) error {
	g := genesis.MustExtractGenesisContext(ctx)
	cfg := g.Rewarding

	if err := validateInflationConfig(&cfg); err != nil {
		return errors.Wrap(err, "invalid IIP-62 inflation configuration")
	}

	supply := cfg.OutstandingSupplyAtActivationBig()
	if supply.Sign() <= 0 {
		return errors.Errorf(
			"OutstandingSupplyAtActivation must be positive, got %s", supply.String())
	}

	s := newInflationState()
	// Only the year-start snapshot, the curve rate, and the year index are persisted —
	// all change only at year boundaries. The live supply, cumulative mint, per-year
	// remainder, and epoch surplus are all derived on demand (see inflation.go) rather
	// than stored, so this record is written once per year instead of every block.
	s.outstandingSupplyAtYearStart.Set(supply)
	s.currentInflationBps = cfg.InflationRateY1Bps
	s.currentYearIndex = 1

	return p.putState(ctx, sm, _inflKey, s)
}

// validateInflationConfig enforces IIP-62 shape constraints on the Rewarding
// genesis fields. Run at activation time; misconfiguration here is unrecoverable.
func validateInflationConfig(cfg *genesis.Rewarding) error {
	if cfg.StakerShareBps+cfg.MachinaShareBps != bpsDenom {
		return errors.Errorf(
			"share splits must sum to %d, got %d+%d",
			bpsDenom, cfg.StakerShareBps, cfg.MachinaShareBps)
	}
	if cfg.InflationFloorBps > cfg.InflationRateY1Bps {
		return errors.Errorf(
			"InflationFloorBps (%d) must not exceed InflationRateY1Bps (%d)",
			cfg.InflationFloorBps, cfg.InflationRateY1Bps)
	}
	if cfg.InflationDecayDenominator == 0 {
		return errors.New("InflationDecayDenominator must be non-zero")
	}
	if cfg.InflationDecayNumerator > cfg.InflationDecayDenominator {
		return errors.Errorf(
			"decay must be ≤ 1 (numerator %d > denominator %d)",
			cfg.InflationDecayNumerator, cfg.InflationDecayDenominator)
	}
	if cfg.BlocksPerYear == 0 {
		return errors.New("BlocksPerYear must be non-zero")
	}
	if cfg.MachinaDaoAddress == "" {
		return errors.New("MachinaDaoAddress must be set in genesis before activation")
	}
	if _, err := address.FromString(cfg.MachinaDaoAddress); err != nil {
		return errors.Wrapf(err, "MachinaDaoAddress %q does not parse", cfg.MachinaDaoAddress)
	}
	if cfg.OutstandingSupplyAtActivation == "" {
		return errors.New("OutstandingSupplyAtActivation must be set in genesis before activation")
	}
	return nil
}

// mintAndAllocate runs the IIP-62 per-block productive-inflation step. Called from
// the top of GrantBlockReward (after the assertNoRewardYet guard). On any block
// before the activation height it is a no-op and returns a zero mStaker so the
// step-F clamp degenerates gracefully.
//
// Pipeline at activation and beyond:
//  1. Load InflationState (must have been seeded by initInflationState).
//  2. If we crossed into a new Year: recompute OutstandingSupplyAtYearStart via the
//     genesis recurrence (ComputeYearStartSupply), refresh CurrentInflationBps from the
//     curve, advance CurrentYearIndex, and persist — this is the ONLY block on which
//     InflationState changes, so it is the only block that writes _inflKey.
//  3. Compute the constant per-block mint for the current Year. On the Year's final
//     block, add the recomputed year-end remainder so the realized annual mint exactly
//     equals the §1.3 table (the remainder is derived, not stored).
//  4. Split via SplitMint (staker = mTotal·bps/denom truncated; Machina = mTotal − staker).
//  5. Credit the staker share to the Fund (mirrors Deposit() arithmetic but without
//     a caller-subtraction — this is protocol mint, not user deposit).
//  6. Credit the Machina share to the externally-managed MachinaDaoAddress account.
//
// PostActivationMinted and the epoch surplus are no longer accumulated here — they are
// derived on read (CumulativeMinted / EpochInflationSurplus), so non-boundary blocks
// leave InflationState untouched and skip the per-block write entirely.
//
// Returns mStaker (this block's staker-share mint) and the per-block transaction
// logs attributing the staker / Machina credits. The staker log uses Sender="" to
// signal "protocol mint, no source" (mirroring the convention that Recipient=""
// indicates burn). Caller should treat a zero mStaker / nil logs as "no productive
// inflation in effect".
//
// Log types: INFLATION_MINT_STAKER / INFLATION_MINT_MACHINA, added to iotex-proto
// alongside IIP-62. Sender="" signals "protocol mint, no source account".
func (p *Protocol) mintAndAllocate(ctx context.Context, sm protocol.StateManager) (*big.Int, []*action.TransactionLog, error) {
	g := genesis.MustExtractGenesisContext(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)
	if !g.IsToBeEnabled(blkCtx.BlockHeight) {
		return new(big.Int), nil, nil
	}
	cfg := g.Rewarding
	activation := g.ToBeEnabledBlockHeight

	s := newInflationState()
	if _, err := p.state(ctx, sm, _inflKey, s); err != nil {
		return nil, nil, errors.Wrap(err, "inflation state not seeded; initInflationState must run at activation")
	}

	year := YearIndex(activation, cfg.BlocksPerYear, blkCtx.BlockHeight)
	if year == 0 {
		return new(big.Int), nil, nil
	}
	// Year boundary crossing: refresh snapshot and curve rate, and persist. This is the
	// only path that mutates InflationState; non-boundary blocks leave it unchanged and
	// skip the write. The branch also fires after a reorg that crosses the boundary
	// because CurrentYearIndex is persisted, so re-execution is deterministic.
	boundary := year != s.currentYearIndex
	if boundary {
		// Recompute the year-start supply via the genesis recurrence. Equivalent to
		// "previous snapshot + that year's AnnualMint" (exact, thanks to the final-block
		// remainder flush), but robust without depending on the prior-year stored rate.
		s.outstandingSupplyAtYearStart = ComputeYearStartSupply(
			cfg.OutstandingSupplyAtActivationBig(),
			year,
			cfg.InflationRateY1Bps,
			cfg.InflationDecayNumerator,
			cfg.InflationDecayDenominator,
			cfg.InflationFloorBps,
			cfg.BlocksPerYear,
		)
		s.currentInflationBps = ComputeInflationBps(
			year,
			cfg.InflationRateY1Bps,
			cfg.InflationDecayNumerator,
			cfg.InflationDecayDenominator,
			cfg.InflationFloorBps,
		)
		s.currentYearIndex = year
	}

	perBlock, rem := PerBlockMint(s.outstandingSupplyAtYearStart, s.currentInflationBps, cfg.BlocksPerYear)
	mTotal := new(big.Int).Set(perBlock)
	// Add the year-end remainder on the Year's final block so the realized annual mint is
	// exact. The remainder is recomputed here, not stored/flushed.
	if IsYearFinalBlock(activation, cfg.BlocksPerYear, blkCtx.BlockHeight) {
		mTotal.Add(mTotal, rem)
	}
	if mTotal.Sign() == 0 {
		return new(big.Int), nil, nil
	}

	mStaker, mMachina := SplitMint(
		mTotal,
		cfg.StakerShareBps, cfg.MachinaShareBps,
	)

	var tLogs []*action.TransactionLog

	// Credit staker share: mirror Fund.Deposit() arithmetic without caller subtraction.
	// This is protocol mint, not a user deposit.
	f := fund{}
	if _, err := p.state(ctx, sm, _fundKey, &f); err != nil {
		return nil, nil, errors.Wrap(err, "failed to load rewarding fund")
	}
	if mStaker.Sign() > 0 {
		f.totalBalance = new(big.Int).Add(f.totalBalance, mStaker)
		f.unclaimedBalance = new(big.Int).Add(f.unclaimedBalance, mStaker)
		if err := p.putState(ctx, sm, _fundKey, &f); err != nil {
			return nil, nil, errors.Wrap(err, "failed to credit staker share to fund")
		}
		tLogs = append(tLogs, &action.TransactionLog{
			Type:      iotextypes.TransactionLogType_INFLATION_MINT_STAKER,
			Sender:    "", // empty Sender = protocol mint, no source account
			Recipient: address.RewardingPoolAddr,
			Amount:    new(big.Int).Set(mStaker),
		})
	}

	// Credit Machina share to the externally-managed recipient account. LoadOrCreate
	// only allocates a state record on first credit; the multisig owning this
	// address is created and governed out of band.
	//
	// This is a pure balance credit — no EVM call is made into the recipient. If the
	// Machina DAO is implemented as a smart contract, its `receive()`/`fallback()`
	// will NOT be invoked; the contract must therefore be designed to tolerate
	// passive balance accrual (a multisig / Gnosis-Safe-style account does this
	// natively; a custom DAO contract that relies on receive() to update internal
	// accounting would not see this credit).
	if mMachina.Sign() > 0 {
		machinaAddr := p.machinaAddr
		accountCreationOpts := []state.AccountCreationOption{}
		if protocol.MustGetFeatureCtx(ctx).CreateLegacyNonceAccount {
			accountCreationOpts = append(accountCreationOpts, state.LegacyNonceAccountTypeOption())
		}
		acc, err := accountutil.LoadOrCreateAccount(sm, machinaAddr, accountCreationOpts...)
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed to load Machina DAO account")
		}
		if err := acc.AddBalance(mMachina); err != nil {
			return nil, nil, errors.Wrap(err, "failed to add Machina DAO balance")
		}
		if err := accountutil.StoreAccount(sm, machinaAddr, acc); err != nil {
			return nil, nil, errors.Wrap(err, "failed to store Machina DAO account")
		}
		tLogs = append(tLogs, &action.TransactionLog{
			Type:      iotextypes.TransactionLogType_INFLATION_MINT_MACHINA,
			Sender:    "", // empty Sender = protocol mint, no source account
			Recipient: machinaAddr.String(),
			Amount:    new(big.Int).Set(mMachina),
		})
	}

	// PostActivationMinted is no longer accumulated, and the staker-vs-block-reward
	// excess is no longer banked: both are derived on read (CumulativeMinted /
	// EpochInflationSurplus). Persist only when the year boundary actually changed the
	// state, so non-boundary blocks add no _inflKey write to the (archive) history.
	if boundary {
		if err := p.putState(ctx, sm, _inflKey, s); err != nil {
			return nil, nil, errors.Wrap(err, "failed to persist inflation state")
		}
	}
	return mStaker, tLogs, nil
}

// cumulativeMintedAt loads the (boundary-only) InflationState and derives the cumulative
// productive mint from activation through the read height. The height returned by p.state
// is the state reader's height — the query height for both latest and historical (archive)
// reads — so the partial-current-year term is reconstructed correctly. Returns the state
// error (e.g. state.ErrStateNotExist before activation) on failure.
func (p *Protocol) cumulativeMintedAt(ctx context.Context, sm protocol.StateReader) (*big.Int, *genesis.Rewarding, uint64, error) {
	s := newInflationState()
	height, err := p.state(ctx, sm, _inflKey, s)
	if err != nil {
		return nil, nil, height, err
	}
	g := genesis.MustExtractGenesisContext(ctx)
	cfg := g.Rewarding
	minted := CumulativeMinted(
		cfg.OutstandingSupplyAtActivationBig(),
		s.outstandingSupplyAtYearStart,
		s.currentInflationBps,
		g.ToBeEnabledBlockHeight,
		cfg.BlocksPerYear,
		height,
	)
	return minted, &cfg, height, nil
}

// OutstandingSupply returns the current outstanding native-token supply tracked by the
// IIP-62 inflation state. It is not stored: by construction it equals
// OutstandingSupplyAtActivation + the cumulative mint through the read height, derived via
// CumulativeMinted. Returns state.ErrStateNotExist before activation.
func (p *Protocol) OutstandingSupply(ctx context.Context, sm protocol.StateReader) (*big.Int, uint64, error) {
	minted, cfg, height, err := p.cumulativeMintedAt(ctx, sm)
	if err != nil {
		return nil, height, err
	}
	return minted.Add(minted, cfg.OutstandingSupplyAtActivationBig()), height, nil
}

// PostActivationMinted returns the cumulative amount minted by productive inflation since
// activation, derived (not stored) via CumulativeMinted. Returns state.ErrStateNotExist
// before activation.
func (p *Protocol) PostActivationMinted(ctx context.Context, sm protocol.StateReader) (*big.Int, uint64, error) {
	minted, _, height, err := p.cumulativeMintedAt(ctx, sm)
	if err != nil {
		return nil, height, err
	}
	return minted, height, nil
}

// CurrentInflationBps returns the inflation rate (in basis points of outstanding
// supply per year) currently in effect. Returns state.ErrStateNotExist before
// activation.
func (p *Protocol) CurrentInflationBps(ctx context.Context, sm protocol.StateReader) (uint64, uint64, error) {
	s := newInflationState()
	height, err := p.state(ctx, sm, _inflKey, s)
	if err != nil {
		return 0, height, err
	}
	return s.currentInflationBps, height, nil
}

func bigToStr(v *big.Int) string {
	if v == nil {
		return "0"
	}
	return v.String()
}

func strToBig(s, field string) (*big.Int, error) {
	if s == "" {
		return new(big.Int), nil
	}
	v, ok := new(big.Int).SetString(s, 10)
	if !ok {
		return nil, errors.Errorf("failed to parse %s as big int: %q", field, s)
	}
	return v, nil
}
