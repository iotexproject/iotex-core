// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package autodeposit

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
)

// AutoDeposit contract storage layout. Verified empirically in
// action/protocol/execution/protocol_iip59_bench_test.go against the
// mainnet runtime bytecode (a voter registered with bucketId=2
// disambiguates the two mappings). Layout is frozen for the lifetime of
// the deployment: AutoDepositRegister is Pausable, Ownable — no proxy, no
// upgradeTo, so a fresh redeploy is the only way these could shift, and a
// redeploy would require a new contract address.
//
// Slot 0 is Pausable._paused. Ownable.owner is never allocated because the
// mainnet deployment installs runtime bytecode directly without running
// the constructor.
const (
	slotBuckets     = uint8(1)
	slotRegistrants = uint8(2)
)

// SlotReader is the minimum surface SlotBucketReader needs from the EVM
// world-state adapter. *evm.SlotReader satisfies it in production; tests
// inject a fake to exercise SlotBucketReader in isolation. Kept as an
// interface here so this package does not pull the evm import — the
// dependency direction stays rewarding → evm, rewarding → autodeposit.
type SlotReader interface {
	GetSlot(contract address.Address, key []byte) []byte
}

// BucketReader is what the rewarding drain path calls once per voter.
// Both the direct-slot production impl (SlotBucketReader) and test fakes
// satisfy it. The (uint64, bool, error) contract matches what the retired
// Bridge.LookupBucket returned so the caller-side switching cost is zero.
type BucketReader interface {
	LookupBucket(voter address.Address) (bucketID uint64, present bool, err error)
}

// SlotBucketReader is the production BucketReader that bypasses the EVM's
// bucket(address) view by reading (registrants[voter], buckets[voter])
// directly from the contract's storage trie. See
// docs/iip-59-perf-report.md for the ~26× per-call speedup this yields at
// mainnet voter counts.
type SlotBucketReader struct {
	r        SlotReader
	contract address.Address
}

// NewSlotBucketReader wires a SlotReader to the bridge's target contract
// address. Bridge caches its contract as a bech32 string; the parse cost is
// paid once per drain (i.e. per NewSlotBucketReader call), not per voter.
func (b *Bridge) NewSlotBucketReader(r SlotReader) (*SlotBucketReader, error) {
	if r == nil {
		return nil, errors.New("autodeposit: nil SlotReader")
	}
	addr, err := address.FromString(b.contract)
	if err != nil {
		return nil, errors.Wrap(err, "autodeposit: parse bridge contract address")
	}
	return &SlotBucketReader{r: r, contract: addr}, nil
}

// LookupBucket returns the voter's registered bucket ID (or unregistered)
// by reading two storage slots directly. Semantics mirror the retired
// Bridge.LookupBucket exactly:
//
//   - (bucketID, true, nil): voter is registered with a strictly positive
//     bucket ID that fits in uint64.
//   - (0, false, nil):        registrants[voter] is zero (unregistered), OR
//     buckets[voter] is non-positive / too large for
//     uint64 (malformed on-chain data). Silent fallback
//     per feedback-consensus-fallback-vs-halt — one
//     malformed voter must not halt the block.
//   - (0, false, err):        wiring failure (nil voter). Wiring bugs stay
//     hard errors; on-chain data does not.
func (b *SlotBucketReader) LookupBucket(voter address.Address) (uint64, bool, error) {
	if voter == nil {
		return 0, false, errors.New("autodeposit: nil voter address")
	}
	voterEth := common.BytesToAddress(voter.Bytes())

	reg := b.r.GetSlot(b.contract, mappingSlotKey(voterEth, slotRegistrants))
	// Solidity stores a bool in the low byte of the 32-byte slot; slot 31
	// is the last byte in big-endian layout. All-zero slot means unset.
	if len(reg) == 0 || reg[len(reg)-1] == 0 {
		return 0, false, nil
	}

	buck := b.r.GetSlot(b.contract, mappingSlotKey(voterEth, slotBuckets))
	val := new(big.Int).SetBytes(buck)
	// SetBytes treats the raw storage as unsigned; a Solidity int256 with
	// the sign bit set surfaces here as a value that fails IsUint64. Both
	// that case and Sign()<=0 (which only fires on all-zero storage — the
	// registrants check above should have caught this already, but keeping
	// the check defends against inconsistent state) fall back to the
	// unregistered branch so PR 3' still makes progress on other voters.
	if val.Sign() <= 0 || !val.IsUint64() {
		return 0, false, nil
	}
	return val.Uint64(), true, nil
}

// mappingSlotKey returns keccak256(pad32(addr) || pad32(mappingSlot)) —
// the Solidity storage-key formula for a mapping(address => T) entry.
// addr occupies bytes [12:32] of the first 32-byte word; the mapping's
// declaration slot occupies the last byte of the second word (uint8 fits).
func mappingSlotKey(addr common.Address, mappingSlot uint8) []byte {
	buf := make([]byte, 64)
	copy(buf[12:32], addr.Bytes())
	buf[63] = mappingSlot
	return crypto.Keccak256(buf)
}
