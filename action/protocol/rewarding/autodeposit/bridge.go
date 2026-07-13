// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

// Package autodeposit bridges the on-chain AutoDeposit contract into the
// IIP-59 protocol-native voter reward drain path.
//
// distributeToVoters (added in PR 3', still unwritten) calls the bridge
// once per voter share at epoch close and routes the share based on the
// returned bucket ID + eligibility check. The bridge is deliberately
// stateless and takes a ContractReader adapter so unit tests can stand in
// a fake without pulling the EVM simulator into scope.
//
// Unlike the DelegateProfile bridge (PR 4.5), AutoDeposit is read live at
// drain time, not frozen at PutPollResult — see IIP-59 §3.6.
package autodeposit

import (
	"context"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
)

// fieldBucketFn is the ABI method name of the AutoDeposit view getter that
// returns a voter's registered compound-target bucket ID.
const fieldBucketFn = "bucket"

// Route classifies where a voter's per-epoch share is delivered. The wire
// value matches IIP-59 §3.5's DelegateDistributed.routings[] encoding, so
// PR 4.7's log emitter and this bridge share a single source of truth.
type Route uint8

const (
	// RouteCredit credits the share to the voter's unclaimed balance for
	// pull-claim via the existing ClaimFromRewardingFund action.
	RouteCredit Route = 0
	// RouteCompound calls into the native staking AddDeposit path against
	// the voter's registered bucket.
	RouteCompound Route = 1
)

// Decision is the per-voter routing verdict produced from LookupBucket plus
// the caller-side eligibility check.
type Decision struct {
	Route    Route
	BucketID uint64 // valid iff Route == RouteCompound
}

// ContractReader is the read-only view-call primitive the bridge depends on.
// Production wires an adapter around evm.SimulateExecution; tests supply an
// in-memory fake. Kept package-local (rather than depending on the
// delegateprofile package's identical type) so this bridge can ship
// independently of PR 4.5.
type ContractReader interface {
	Read(ctx context.Context, contract string, callData []byte) ([]byte, error)
}

// ContractReaderFunc lets a plain function satisfy ContractReader.
type ContractReaderFunc func(ctx context.Context, contract string, callData []byte) ([]byte, error)

// Read implements ContractReader.
func (f ContractReaderFunc) Read(ctx context.Context, contract string, callData []byte) ([]byte, error) {
	return f(ctx, contract, callData)
}

// ErrEmptyContractAddress is returned when the bridge is constructed
// without a target contract.
var ErrEmptyContractAddress = errors.New("autodeposit: empty contract address")

// Bridge is the reusable read-only wrapper around a specific AutoDeposit
// contract deployment. Construct once at protocol init with the network's
// pinned contract address; call LookupBucket per voter at drain time.
type Bridge struct {
	abi      abi.ABI
	contract string
}

// New constructs a Bridge targeting contract. contract must be a valid
// IoTeX bech32 address; the caller is responsible for pinning it to the
// network-appropriate mainnet/testnet value.
func New(contract string) (*Bridge, error) {
	if contract == "" {
		return nil, ErrEmptyContractAddress
	}
	if _, err := address.FromString(contract); err != nil {
		return nil, errors.Wrap(err, "autodeposit: invalid contract address")
	}
	parsed, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		return nil, errors.Wrap(err, "autodeposit: failed to parse ABI")
	}
	return &Bridge{abi: parsed, contract: contract}, nil
}

// Contract returns the target contract address, mostly for logging.
func (b *Bridge) Contract() string { return b.contract }

// LookupBucket reads AutoDeposit.bucket(voter) and normalises the result
// to (uint64, present, error):
//
//   - (bucketID, true, nil): voter is registered with a strictly positive
//     bucket ID that fits in uint64.
//   - (0, false, nil):        voter is unregistered — the contract returned
//     zero, a negative int256, or a value larger than
//     2^63-1. All three are treated identically so PR 3'
//     routes them to RouteCredit without further branching.
//   - (0, false, err):        wiring failure (nil reader, nil voter, ABI
//     mismatch, or reader RPC error). Callers MUST log
//     and route to RouteCredit; see IIP-59 §3.6:
//     "a failed contract call falls through to
//     unclaimedBalance".
//
// The malformed-value silent fallback follows the consensus-path rule:
// one voter's malformed on-chain registration must not halt the block.
// Wiring bugs (nil reader / nil voter) stay hard errors — those indicate
// a caller-side mistake, not on-chain data.
func (b *Bridge) LookupBucket(
	ctx context.Context,
	reader ContractReader,
	voter address.Address,
) (uint64, bool, error) {
	if reader == nil {
		return 0, false, errors.New("autodeposit: nil ContractReader")
	}
	if voter == nil {
		return 0, false, errors.New("autodeposit: nil voter address")
	}

	voterEth := common.BytesToAddress(voter.Bytes())
	callData, err := b.abi.Pack(fieldBucketFn, voterEth)
	if err != nil {
		return 0, false, errors.Wrap(err, "autodeposit: pack bucket(address)")
	}
	raw, err := reader.Read(ctx, b.contract, callData)
	if err != nil {
		return 0, false, errors.Wrapf(err, "autodeposit: read bucket(%s)", voter.String())
	}
	out, err := b.abi.Unpack(fieldBucketFn, raw)
	if err != nil {
		return 0, false, errors.Wrap(err, "autodeposit: unpack bucket(address)")
	}
	if len(out) != 1 {
		return 0, false, errors.Errorf("autodeposit: bucket(address) expected 1 return value, got %d", len(out))
	}
	bigVal, ok := out[0].(*big.Int)
	if !ok {
		return 0, false, errors.Errorf("autodeposit: bucket(address) expected *big.Int, got %T", out[0])
	}
	// Non-positive means unregistered per IIP-59 §3.6 precondition 1.
	// Solidity's default-zero for unset mappings and negative sentinels
	// both land here.
	if bigVal.Sign() <= 0 {
		return 0, false, nil
	}
	// Values that don't fit uint64 are malformed on-chain data. Silent
	// fallback to unregistered so PR 3' still makes progress on other
	// voters — see feedback-consensus-fallback-vs-halt.
	if !bigVal.IsUint64() {
		return 0, false, nil
	}
	return bigVal.Uint64(), true, nil
}
