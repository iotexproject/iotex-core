// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

// Package distributedlog encodes the IIP-59 §3.2 DelegateDistributed
// receipt event. distributeToVoters (PR 3') calls Pack once per delegate
// at epoch close and wraps the returned Topics+data into an *action.Log;
// this package deliberately does NOT construct action.Log itself so it
// can be unit-tested without block context (mirrors the seam chosen by
// action/protocol/rewarding/delegateprofile and .../autodeposit).
package distributedlog

import (
	"encoding/binary"
	"math/big"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action"
)

// Errors returned by Pack. All three indicate a wiring bug in the caller
// (PR 3'), not on-chain data, so they hard-fail rather than degrade —
// there is no per-item fallback available at the log-encode layer.
var (
	// ErrParallelArrayLengthMismatch is returned when the voters, recipients,
	// amounts, compound bucket ID, and compounded slices in EventArgs do not have
	// identical lengths. The rewarding protocol constructs these arrays in
	// lock-step, so a mismatch is a serialisation bug.
	ErrParallelArrayLengthMismatch = errors.New(
		"distributedlog: voters, recipients, amounts, compound bucket IDs, and compounded flags must have equal length")

	// ErrNilAddress is returned when Delegate, RewardAddr, or any
	// Voters[i] is nil. Passing nil is a caller-side mistake and would
	// otherwise packet as the zero address, silently masking a lost voter.
	ErrNilAddress = errors.New("distributedlog: nil address")

	// ErrNilBigInt is returned when TotalCommission, TotalVoterPool, or
	// any Amounts[i] is nil.
	ErrNilBigInt = errors.New("distributedlog: nil *big.Int")
)

// eraSnapshotDomainSeparator scopes EraSnapshotHash so its output cannot
// collide with a hash computed from the same byte layout in some other
// context. Value: keccak256("iip59.delegatedistributed.snapshot.v2"),
// evaluated once at init to keep the hot path allocation-free.
//
// v2, not v1: v1 scoped a digest over the frozen (voter, weight) list, a
// preimage of an entirely different shape. Bumping the separator keeps the two
// domains disjoint rather than relying on the layouts never colliding.
var eraSnapshotDomainSeparator hash.Hash256

func init() {
	eraSnapshotDomainSeparator = hash.Hash256b([]byte("iip59.delegatedistributed.snapshot.v2"))
}

// abiOnce guards parseABI: abi.JSON is not free and the parsed ABI is
// immutable, so we cache it for the process lifetime.
var (
	abiOnce     sync.Once
	parsedABI   abi.ABI
	abiParseErr error
)

func loadABI() (abi.ABI, error) {
	abiOnce.Do(func() {
		parsedABI, abiParseErr = abi.JSON(strings.NewReader(abiJSON))
	})
	return parsedABI, abiParseErr
}

// EventArgs are the fields of one DelegateDistributed log. Field order
// matches the Solidity signature so a struct literal reads left-to-right
// the same way as the on-chain event definition. Callers build one per
// delegate (top-N loop and orphan-drain loop use the same shape).
type EventArgs struct {
	Epoch             uint64            // indexed → Topics[1]
	Delegate          address.Address   // indexed → Topics[2]
	RewardAddr        address.Address   // where commission was credited
	TotalCommission   *big.Int          // aggregate delegate commission
	TotalVoterPool    *big.Int          // pool split across voters
	SnapshotHash      hash.Hash256      // frozen era parameter digest (see EraSnapshotHash)
	Voters            []address.Address // canonical sorted order per §3.4
	Recipients        []address.Address // actual direct recipient; voter for compound payout
	Amounts           []*big.Int        // parallel to Voters
	CompoundBucketIDs []uint64          // parallel to Voters; meaningful only where Compounded is true
	Compounded        []bool            // parallel to Voters; true → paid into CompoundBucketIDs[i]
}

// Pack encodes args as an EVM-shaped receipt log. The returned Topics
// and data satisfy the same layout an EVM-emitted DelegateDistributed
// event would produce, so off-chain verifiers (PR #45) can decode with
// stock ethers.js / web3.py against this event ABI.
//
// Topics layout:
//
//	[0] keccak256(eventSignature)
//	[1] uint64 epoch, left-padded to 32 bytes
//	[2] address delegate, left-padded to 32 bytes
//
// Data layout: ABI-standard tuple of the remaining (non-indexed) inputs
// in declaration order — rewardAddr, totalCommission, totalVoterPool,
// snapshotHash, voters[], recipients[], amounts[], compoundBucketIds[],
// compounded[].
//
// compounded[i] is the only valid test for "was voter i's share compounded".
// compoundBucketIds[i] == 0 is NOT that test: native bucket index 0 is a real
// bucket and a voter can legitimately be compounded into it.
func Pack(args EventArgs) (action.Topics, []byte, error) {
	if args.Delegate == nil {
		return nil, nil, errors.Wrap(ErrNilAddress, "delegate")
	}
	if args.RewardAddr == nil {
		return nil, nil, errors.Wrap(ErrNilAddress, "rewardAddr")
	}
	if args.TotalCommission == nil {
		return nil, nil, errors.Wrap(ErrNilBigInt, "totalCommission")
	}
	if args.TotalVoterPool == nil {
		return nil, nil, errors.Wrap(ErrNilBigInt, "totalVoterPool")
	}
	if len(args.Voters) != len(args.Recipients) || len(args.Voters) != len(args.Amounts) ||
		len(args.Voters) != len(args.CompoundBucketIDs) || len(args.Voters) != len(args.Compounded) {
		return nil, nil, errors.Wrapf(ErrParallelArrayLengthMismatch,
			"voters=%d recipients=%d amounts=%d compoundBucketIds=%d compounded=%d",
			len(args.Voters), len(args.Recipients), len(args.Amounts),
			len(args.CompoundBucketIDs), len(args.Compounded))
	}

	voterAddrs, err := toEthAddresses(args.Voters, "voters")
	if err != nil {
		return nil, nil, err
	}
	recipientAddrs, err := toEthAddresses(args.Recipients, "recipients")
	if err != nil {
		return nil, nil, err
	}
	amounts := make([]*big.Int, len(args.Amounts))
	for i, a := range args.Amounts {
		if a == nil {
			return nil, nil, errors.Wrapf(ErrNilBigInt, "amounts[%d]", i)
		}
		amounts[i] = a
	}
	parsed, err := loadABI()
	if err != nil {
		return nil, nil, errors.Wrap(err, "distributedlog: parse ABI")
	}
	ev, ok := parsed.Events[eventName]
	if !ok {
		return nil, nil, errors.Errorf("distributedlog: event %q not found in parsed ABI", eventName)
	}
	data, err := ev.Inputs.NonIndexed().Pack(
		common.BytesToAddress(args.RewardAddr.Bytes()),
		args.TotalCommission,
		args.TotalVoterPool,
		[32]byte(args.SnapshotHash),
		voterAddrs,
		recipientAddrs,
		amounts,
		args.CompoundBucketIDs,
		args.Compounded,
	)
	if err != nil {
		return nil, nil, errors.Wrap(err, "distributedlog: pack DelegateDistributed data")
	}

	topics := make(action.Topics, 3)
	topics[0] = hash.Hash256(ev.ID)
	topics[1] = encodeUint64Topic(args.Epoch)
	topics[2] = hash.BytesToHash256(args.Delegate.Bytes())
	return topics, data, nil
}

// encodeUint64Topic returns the 32-byte, left-padded big-endian
// representation of x — the ABI encoding for an indexed uint64.
// toEthAddresses converts a parallel-array argument to the 20-byte form the
// event ABI packs. field names the argument in the error so a nil entry is
// traceable to the caller's array.
func toEthAddresses(addrs []address.Address, field string) ([]common.Address, error) {
	out := make([]common.Address, len(addrs))
	for i, a := range addrs {
		if a == nil {
			return nil, errors.Wrapf(ErrNilAddress, "%s[%d]", field, i)
		}
		out[i] = common.BytesToAddress(a.Bytes())
	}
	return out, nil
}

func encodeUint64Topic(x uint64) hash.Hash256 {
	var buf [32]byte
	binary.BigEndian.PutUint64(buf[24:], x)
	return hash.BytesToHash256(buf[:])
}

// EraSnapshotParams are the frozen per-delegate era scalars EraSnapshotHash
// commits to. They are exactly the contents of a CandidatePollSnapshot plus
// the candidate identifier that keys it.
type EraSnapshotParams struct {
	Delegate                   address.Address
	FreezeHeight               uint64
	TotalWeight                *big.Int
	SelfStakeBucketIdx         uint64
	BlockCommissionBasisPoints uint64
	EpochCommissionBasisPoints uint64
	Registered                 bool
	OnchainRewardEnabled       bool
}

// EraSnapshotHash produces the bytes32 digest a DelegateDistributed log
// carries in its snapshotHash field.
//
// One settlement pays a delegate's voters across many blocks and emits one
// partial log per block, so an off-chain consumer reassembles a delegate's
// payout by grouping logs on (snapshotHash, delegate, epoch). The digest's job
// is therefore to be a stable per-delegate-per-era identifier that a consumer
// can also recompute from a `voterRewardDelegateSnapshot` read to confirm the batch it
// assembled belongs to the era it thinks it does.
//
// It commits to every scalar the era froze for the delegate. FreezeHeight is
// what makes it era-unique: two consecutive boundaries at which nothing about
// a delegate changed still produce different digests. It does not commit to a
// voter list, because there is no longer a frozen one -- voters are enumerated
// from the era's copy-on-write bucket window and their weights recomputed, and
// TotalWeight (the frozen candidate.Votes) is the aggregate that governs every
// share the logs report.
//
// Layout hashed:
//
//	keccak256(
//	    domainSep ||
//	    delegate.Bytes()(20B) ||
//	    be_uint64(freezeHeight) ||
//	    left_pad32(totalWeight) ||
//	    be_uint64(selfStakeBucketIdx) ||
//	    be_uint64(blockCommissionBasisPoints) ||
//	    be_uint64(epochCommissionBasisPoints) ||
//	    flags(1B: bit0=registered, bit1=onchainRewardEnabled)
//	)
//
// A nil Delegate hashes as 20 zero bytes rather than erroring; the freezer
// never passes one, and a digest is not a place to fail a block from.
func EraSnapshotHash(p EraSnapshotParams) hash.Hash256 {
	buf := make([]byte, 0, 32+20+8+32+8+8+8+1)
	buf = append(buf, eraSnapshotDomainSeparator[:]...)
	if p.Delegate == nil {
		buf = append(buf, make([]byte, 20)...)
	} else {
		buf = append(buf, p.Delegate.Bytes()...)
	}
	buf = appendUint64BE(buf, p.FreezeHeight)
	buf = append(buf, leftPad32(p.TotalWeight)...)
	buf = appendUint64BE(buf, p.SelfStakeBucketIdx)
	buf = appendUint64BE(buf, p.BlockCommissionBasisPoints)
	buf = appendUint64BE(buf, p.EpochCommissionBasisPoints)
	var flags byte
	if p.Registered {
		flags |= 1
	}
	if p.OnchainRewardEnabled {
		flags |= 2
	}
	return hash.Hash256b(append(buf, flags))
}

// appendUint64BE appends x in 8-byte big-endian form.
func appendUint64BE(buf []byte, x uint64) []byte {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], x)
	return append(buf, b[:]...)
}

// leftPad32 returns the big-endian, zero-left-padded 32-byte
// representation of x's absolute value. nil is treated as zero;
// negative values (not expected in this domain) are encoded by
// absolute value — the snapshot never contains negative weights.
func leftPad32(x *big.Int) []byte {
	var out [32]byte
	if x == nil {
		return out[:]
	}
	b := x.Bytes()
	copy(out[32-len(b):], b)
	return out[:]
}
