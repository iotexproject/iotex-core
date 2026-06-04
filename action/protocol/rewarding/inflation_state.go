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
type inflationState struct {
	outstandingSupply            *big.Int
	outstandingSupplyAtYearStart *big.Int
	postActivationMinted         *big.Int
	currentInflationBps          uint64
	currentYearIndex             uint64
	dustStaker                   *big.Int
	dustMachina                  *big.Int
	yearMintRemainder            *big.Int
	epochRemainderAccumulator    *big.Int
}

func newInflationState() *inflationState {
	return &inflationState{
		outstandingSupply:            new(big.Int),
		outstandingSupplyAtYearStart: new(big.Int),
		postActivationMinted:         new(big.Int),
		dustStaker:                   new(big.Int),
		dustMachina:                  new(big.Int),
		yearMintRemainder:            new(big.Int),
		epochRemainderAccumulator:    new(big.Int),
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
		OutstandingSupply:            bigToStr(s.outstandingSupply),
		OutstandingSupplyAtYearStart: bigToStr(s.outstandingSupplyAtYearStart),
		PostActivationMinted:         bigToStr(s.postActivationMinted),
		CurrentInflationBps:          s.currentInflationBps,
		CurrentYearIndex:             s.currentYearIndex,
		DustStaker:                   bigToStr(s.dustStaker),
		DustMachina:                  bigToStr(s.dustMachina),
		YearMintRemainder:            bigToStr(s.yearMintRemainder),
		EpochRemainderAccumulator:    bigToStr(s.epochRemainderAccumulator),
	}
}

func (s *inflationState) fromProto(gen *rewardingpb.InflationState) error {
	var err error
	if s.outstandingSupply, err = strToBig(gen.OutstandingSupply, "outstandingSupply"); err != nil {
		return err
	}
	if s.outstandingSupplyAtYearStart, err = strToBig(gen.OutstandingSupplyAtYearStart, "outstandingSupplyAtYearStart"); err != nil {
		return err
	}
	if s.postActivationMinted, err = strToBig(gen.PostActivationMinted, "postActivationMinted"); err != nil {
		return err
	}
	if s.dustStaker, err = strToBig(gen.DustStaker, "dustStaker"); err != nil {
		return err
	}
	if s.dustMachina, err = strToBig(gen.DustMachina, "dustMachina"); err != nil {
		return err
	}
	if s.yearMintRemainder, err = strToBig(gen.YearMintRemainder, "yearMintRemainder"); err != nil {
		return err
	}
	if s.epochRemainderAccumulator, err = strToBig(gen.EpochRemainderAccumulator, "epochRemainderAccumulator"); err != nil {
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
	s.outstandingSupply.Set(supply)
	s.outstandingSupplyAtYearStart.Set(supply)
	s.currentInflationBps = cfg.InflationRateY1Bps
	s.currentYearIndex = 1
	// Pre-stage Y1's year-end remainder here — mintAndAllocate's boundary branch
	// only fires when year != currentYearIndex (Y2+), so without this seed the Y1
	// final-block flush would add zero and Y1 mint would fall short by up to
	// blocksPerYear-1 Rau.
	_, rem := PerBlockMint(s.outstandingSupplyAtYearStart, s.currentInflationBps, cfg.BlocksPerYear)
	s.yearMintRemainder.Set(rem)

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
//  2. If we crossed into a new Year: snapshot OutstandingSupplyAtYearStart from the
//     current OutstandingSupply, recompute CurrentInflationBps from the curve, and
//     reset YearMintRemainder to (annualMint mod blocksPerYear) for the new Year.
//  3. Compute the constant per-block mint for the current Year. On the Year's final
//     block, add YearMintRemainder so the realized annual mint exactly equals the
//     §1.3 table.
//  4. Split via SplitMint, carrying sub-bpsDenom dust between blocks.
//  5. Credit the staker share to the Fund (mirrors Deposit() arithmetic but without
//     a caller-subtraction — this is protocol mint, not user deposit).
//  6. Credit the Machina share to the externally-managed MachinaDaoAddress account.
//  7. Bump OutstandingSupply / PostActivationMinted; bank the staker-vs-block-reward
//     excess into EpochRemainderAccumulator (consumed by step G in GrantEpochReward).
//  8. Persist.
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
	// Year boundary crossing: refresh snapshot and curve rate. This branch also fires
	// after a reorg that crosses the boundary because CurrentYearIndex is persisted.
	if year != s.currentYearIndex {
		s.outstandingSupplyAtYearStart.Set(s.outstandingSupply)
		s.currentInflationBps = ComputeInflationBps(
			year,
			cfg.InflationRateY1Bps,
			cfg.InflationDecayNumerator,
			cfg.InflationDecayDenominator,
			cfg.InflationFloorBps,
		)
		s.currentYearIndex = year
		// Pre-stage the year-end remainder so the year's final block can flush it.
		_, rem := PerBlockMint(s.outstandingSupplyAtYearStart, s.currentInflationBps, cfg.BlocksPerYear)
		s.yearMintRemainder.Set(rem)
	}

	perBlock, _ := PerBlockMint(s.outstandingSupplyAtYearStart, s.currentInflationBps, cfg.BlocksPerYear)
	mTotal := new(big.Int).Set(perBlock)
	// Flush yearMintRemainder on the Year's final block so realized annual mint is exact.
	if IsYearFinalBlock(activation, cfg.BlocksPerYear, blkCtx.BlockHeight) {
		mTotal.Add(mTotal, s.yearMintRemainder)
		s.yearMintRemainder.SetUint64(0)
	}
	if mTotal.Sign() == 0 {
		return new(big.Int), nil, nil
	}

	mStaker, mMachina, dStaker, dMachina := SplitMint(
		mTotal,
		cfg.StakerShareBps, cfg.MachinaShareBps,
		s.dustStaker, s.dustMachina,
	)
	s.dustStaker.Set(dStaker)
	s.dustMachina.Set(dMachina)

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

	s.outstandingSupply.Add(s.outstandingSupply, mTotal)
	s.postActivationMinted.Add(s.postActivationMinted, mTotal)
	// Bank the staker-share-minus-block-reward excess into the epoch accumulator.
	// effective_block_reward = min(a.blockReward, mStaker); excess = mStaker − that.
	// Must read a.blockReward (admin state) — the SAME source calculateTotalRewardAndTip
	// uses for the actual grant — so the epoch accumulator stays exactly consistent
	// with what GrantBlockReward pays out. Post-Wake, a.blockReward == WakeBlockReward.
	a := admin{}
	if _, err := p.state(ctx, sm, _adminKey, &a); err != nil {
		return nil, nil, errors.Wrap(err, "failed to load rewarding admin for block-reward clamp")
	}
	effectiveBlock := new(big.Int).Set(a.blockReward)
	if mStaker.Cmp(effectiveBlock) < 0 {
		effectiveBlock.Set(mStaker)
	}
	excess := new(big.Int).Sub(mStaker, effectiveBlock)
	if excess.Sign() > 0 {
		s.epochRemainderAccumulator.Add(s.epochRemainderAccumulator, excess)
	}

	if err := p.putState(ctx, sm, _inflKey, s); err != nil {
		return nil, nil, errors.Wrap(err, "failed to persist inflation state")
	}
	return mStaker, tLogs, nil
}

// OutstandingSupply returns the current outstanding native-token supply tracked
// by the IIP-62 inflation state. Returns state.ErrStateNotExist before activation.
func (p *Protocol) OutstandingSupply(ctx context.Context, sm protocol.StateReader) (*big.Int, uint64, error) {
	s := newInflationState()
	height, err := p.state(ctx, sm, _inflKey, s)
	if err != nil {
		return nil, height, err
	}
	return s.outstandingSupply, height, nil
}

// PostActivationMinted returns the cumulative amount minted by productive
// inflation since activation. Returns state.ErrStateNotExist before activation.
func (p *Protocol) PostActivationMinted(ctx context.Context, sm protocol.StateReader) (*big.Int, uint64, error) {
	s := newInflationState()
	height, err := p.state(ctx, sm, _inflKey, s)
	if err != nil {
		return nil, height, err
	}
	return s.postActivationMinted, height, nil
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
