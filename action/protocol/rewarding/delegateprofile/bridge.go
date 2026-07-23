// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

// Package delegateprofile bridges the on-chain DelegateProfile contract into
// the IIP-59 protocol-native voter reward path.
//
// PutPollResult calls the bridge once per epoch, passing the freshly selected
// candidate list. The bridge issues two view calls per delegate against the
// DelegateProfile contract (getProfileByField for "blockRewardPortion" and
// "epochRewardPortion") and returns the per-delegate commission split in
// basis points. Downstream (PR 2') freezes those values into the poll
// snapshot; rewarding (PR 3'/4') consumes them at epoch close.
package delegateprofile

import (
	"context"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// Field names used by the existing DelegateProfile contract. These are the
// same string keys the legacy Hermes service reads. Do not rename without a
// coordinated contract update — the keys are the on-chain lookup, not
// something we control.
const (
	fieldBlockRewardPortion = "blockRewardPortion"
	fieldEpochRewardPortion = "epochRewardPortion"

	// maxBasisPoints is the ceiling for a commission (or voter-take) rate:
	// 10000 basis points = 100.00%.
	maxBasisPoints uint64 = 10000
)

var (
	// ErrRateOutOfRange is returned when the on-chain field decodes to a value
	// outside [0, 10000]. Callers should treat this as a hard error at
	// PutPollResult — a malformed profile must not be silently ignored, since
	// silently defaulting to 0% would flip the delegate's whole reward stream
	// to the wrong party.
	ErrRateOutOfRange = errors.New("delegateprofile: portion out of range [0, 10000]")

	// ErrEmptyContractAddress is returned when the bridge is constructed
	// without a target contract.
	ErrEmptyContractAddress = errors.New("delegateprofile: empty contract address")
)

// ContractReader is the read-only view call primitive the bridge depends on.
// PutPollResult supplies an adapter around evm.SimulateExecution; tests supply
// an in-memory fake. The bridge does not depend on protocol.StateManager
// directly so it stays trivially unit-testable.
type ContractReader interface {
	Read(ctx context.Context, contract string, callData []byte) ([]byte, error)
}

// ContractReaderFunc lets a plain function satisfy ContractReader.
type ContractReaderFunc func(ctx context.Context, contract string, callData []byte) ([]byte, error)

// Read implements ContractReader.
func (f ContractReaderFunc) Read(ctx context.Context, contract string, callData []byte) ([]byte, error) {
	return f(ctx, contract, callData)
}

// CommissionRates carries the per-delegate result of a single bridge lookup.
//
// Registered flags whether the DelegateProfile contract has both portion
// fields set for this delegate. If false, both commission rates default to
// zero, so the post-fork rewarding path sends the full amount to voters.
type CommissionRates struct {
	// BlockCommissionBasisPoints is the delegate's take of the block-reward
	// stream, in basis points [0, 10000]. Derived as 10000 - voterTake_bp,
	// where voterTake_bp is the raw uint64 stored on-chain under
	// "blockRewardPortion".
	BlockCommissionBasisPoints uint64

	// EpochCommissionBasisPoints is the delegate's take of the epoch-reward
	// stream, in basis points [0, 10000]. Derived from "epochRewardPortion".
	EpochCommissionBasisPoints uint64

	// Registered is true iff both portion fields returned non-empty bytes.
	// A partial profile (one field set, the other empty) is treated as
	// Registered=false and therefore uses the all-to-voters default.
	Registered bool
}

// Bridge is the reusable read-only wrapper around a specific DelegateProfile
// contract deployment. Construct once at protocol init with the network's
// fixed contract address; call Snapshot once per PutPollResult.
type Bridge struct {
	abi      abi.ABI
	contract string
}

// New constructs a Bridge targeting contract. contract must be a valid IoTeX
// bech32 address; the caller is responsible for pinning it to the
// network-appropriate mainnet/testnet value.
func New(contract string) (*Bridge, error) {
	if contract == "" {
		return nil, ErrEmptyContractAddress
	}
	if _, err := address.FromString(contract); err != nil {
		return nil, errors.Wrap(err, "delegateprofile: invalid contract address")
	}
	parsed, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		return nil, errors.Wrap(err, "delegateprofile: failed to parse ABI")
	}
	return &Bridge{abi: parsed, contract: contract}, nil
}

// Contract returns the target contract address, mostly for logging.
func (b *Bridge) Contract() string { return b.contract }

// Snapshot reads commission portions for each delegate and returns a map
// keyed by delegate identity string (bech32). The map always contains one
// entry per input delegate.
//
// Iteration order over `delegates` is preserved: the bridge performs 2N
// sequential read calls in delegate-order. This keeps per-epoch behaviour
// deterministic and mirrors the caller's PutPollResult ordering.
//
// A per-delegate read error (RPC failure, ABI mismatch, out-of-range value)
// degrades that delegate to `Registered=false` with zero rates — downstream
// rewarding then uses the all-to-voters default. The error is logged
// but not returned. Rationale: PutPollResult runs at every epoch boundary
// on every validator, and returning an error would deterministically halt
// the chain if a single delegate's on-chain profile becomes malformed.
// Deterministic reward-path fallback is preferable to deterministic block
// production failure — same on-chain state ⇒ same fallback ⇒ no fork.
//
// Only catastrophic caller-side inputs (nil reader, nil delegate address)
// return an error; those indicate a wiring bug, not a data issue, and must
// surface loudly.
func (b *Bridge) Snapshot(
	ctx context.Context,
	reader ContractReader,
	delegates []address.Address,
) (map[string]*CommissionRates, error) {
	if reader == nil {
		return nil, errors.New("delegateprofile: nil ContractReader")
	}
	out := make(map[string]*CommissionRates, len(delegates))
	for _, d := range delegates {
		if d == nil {
			return nil, errors.New("delegateprofile: nil delegate address in list")
		}
		rates, err := b.readOne(ctx, reader, d)
		if err != nil {
			log.L().Warn(
				"delegateprofile: read failed, using default voter reward split",
				zap.String("delegate", d.String()),
				zap.String("contract", b.contract),
				zap.Error(err),
			)
			out[d.String()] = &CommissionRates{Registered: false}
			continue
		}
		out[d.String()] = rates
	}
	return out, nil
}

// readOne fetches both portion fields for one delegate and inverts them into
// commission basis points.
func (b *Bridge) readOne(
	ctx context.Context,
	reader ContractReader,
	delegate address.Address,
) (*CommissionRates, error) {
	ethAddr := common.BytesToAddress(delegate.Bytes())

	blockVoterBp, blockOK, err := b.queryPortion(ctx, reader, ethAddr, fieldBlockRewardPortion)
	if err != nil {
		return nil, err
	}
	epochVoterBp, epochOK, err := b.queryPortion(ctx, reader, ethAddr, fieldEpochRewardPortion)
	if err != nil {
		return nil, err
	}
	if !blockOK || !epochOK {
		return &CommissionRates{Registered: false}, nil
	}
	return &CommissionRates{
		BlockCommissionBasisPoints: maxBasisPoints - blockVoterBp,
		EpochCommissionBasisPoints: maxBasisPoints - epochVoterBp,
		Registered:                 true,
	}, nil
}

// queryPortion issues a single getProfileByField call and decodes the return
// as (voter-take basis points, present).
//
// "present" is false when the on-chain field returns empty bytes — the
// DelegateProfile contract's default value for an unset field. In that case
// the returned uint64 is zero and callers MUST NOT treat it as 0%
// voter-take (which would mean 100% commission). Distinguishing "absent"
// from "explicit zero" is the entire reason we surface a bool rather than
// just a number.
func (b *Bridge) queryPortion(
	ctx context.Context,
	reader ContractReader,
	delegate common.Address,
	field string,
) (uint64, bool, error) {
	callData, err := b.abi.Pack("getProfileByField", delegate, field)
	if err != nil {
		return 0, false, errors.Wrapf(err, "pack getProfileByField(%s)", field)
	}
	raw, err := reader.Read(ctx, b.contract, callData)
	if err != nil {
		return 0, false, errors.Wrapf(err, "read getProfileByField(%s)", field)
	}
	out, err := b.abi.Unpack("getProfileByField", raw)
	if err != nil {
		return 0, false, errors.Wrapf(err, "unpack getProfileByField(%s)", field)
	}
	if len(out) != 1 {
		return 0, false, errors.Errorf("getProfileByField(%s): expected 1 return value, got %d", field, len(out))
	}
	valueBytes, ok := out[0].([]byte)
	if !ok {
		return 0, false, errors.Errorf("getProfileByField(%s): expected bytes return, got %T", field, out[0])
	}
	if len(valueBytes) == 0 {
		return 0, false, nil
	}
	// The DelegateProfile contract stores the portion as big-endian bytes of
	// a uint256, which is how updateProfile(name, value) writes it. Values
	// larger than uint64 are always malformed for this field; SetBytes → Uint64
	// truncates silently, so we range-check first.
	value := new(big.Int).SetBytes(valueBytes)
	if !value.IsUint64() {
		return 0, false, errors.Wrapf(ErrRateOutOfRange, "field=%s raw=%x", field, valueBytes)
	}
	bp := value.Uint64()
	if bp > maxBasisPoints {
		return 0, false, errors.Wrapf(ErrRateOutOfRange, "field=%s bp=%d", field, bp)
	}
	return bp, true, nil
}
