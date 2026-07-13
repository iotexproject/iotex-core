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
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/autodeposit"
)

// Errors returned by Pack. All three indicate a wiring bug in the caller
// (PR 3'), not on-chain data, so they hard-fail rather than degrade —
// there is no per-item fallback available at the log-encode layer.
var (
	// ErrParallelArrayLengthMismatch is returned when the voters, amounts,
	// and routings slices in EventArgs do not have identical lengths.
	// PR 3' constructs these three in lock-step, so a mismatch is a
	// serialisation bug.
	ErrParallelArrayLengthMismatch = errors.New(
		"distributedlog: voters, amounts, and routings must have equal length")

	// ErrNilAddress is returned when Delegate, RewardAddr, or any
	// Voters[i] is nil. Passing nil is a caller-side mistake and would
	// otherwise packet as the zero address, silently masking a lost voter.
	ErrNilAddress = errors.New("distributedlog: nil address")

	// ErrNilBigInt is returned when TotalCommission, TotalVoterPool, or
	// any Amounts[i] is nil.
	ErrNilBigInt = errors.New("distributedlog: nil *big.Int")
)

// snapshotDomainSeparator scopes SnapshotHash so its output cannot
// collide with a hash computed from the same byte layout in some other
// context (e.g., a Merkle proof over the same voter list). Value:
// keccak256("iip59.delegatedistributed.snapshot.v1"), evaluated once at
// init to keep the hot path allocation-free.
var snapshotDomainSeparator hash.Hash256

func init() {
	snapshotDomainSeparator = hash.Hash256b([]byte("iip59.delegatedistributed.snapshot.v1"))
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
	Epoch           uint64              // indexed → Topics[1]
	Delegate        address.Address     // indexed → Topics[2]
	RewardAddr      address.Address     // where commission was credited
	TotalCommission *big.Int            // aggregate delegate commission
	TotalVoterPool  *big.Int            // pool split across voters
	SnapshotHash    hash.Hash256        // frozen voter list digest (see SnapshotHash)
	Voters          []address.Address   // canonical sorted order per §3.4
	Amounts         []*big.Int          // parallel to Voters
	Routings        []autodeposit.Route // parallel to Voters; wire enum from PR 4.6
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
// snapshotHash, voters[], amounts[], routings[].
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
	if len(args.Voters) != len(args.Amounts) || len(args.Voters) != len(args.Routings) {
		return nil, nil, errors.Wrapf(ErrParallelArrayLengthMismatch,
			"voters=%d amounts=%d routings=%d",
			len(args.Voters), len(args.Amounts), len(args.Routings))
	}

	voterAddrs := make([]common.Address, len(args.Voters))
	for i, v := range args.Voters {
		if v == nil {
			return nil, nil, errors.Wrapf(ErrNilAddress, "voters[%d]", i)
		}
		voterAddrs[i] = common.BytesToAddress(v.Bytes())
	}
	amounts := make([]*big.Int, len(args.Amounts))
	for i, a := range args.Amounts {
		if a == nil {
			return nil, nil, errors.Wrapf(ErrNilBigInt, "amounts[%d]", i)
		}
		amounts[i] = a
	}
	routings := make([]uint8, len(args.Routings))
	for i, r := range args.Routings {
		routings[i] = uint8(r)
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
		amounts,
		routings,
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
func encodeUint64Topic(x uint64) hash.Hash256 {
	var buf [32]byte
	binary.BigEndian.PutUint64(buf[24:], x)
	return hash.BytesToHash256(buf[:])
}

// SnapshotHash produces the bytes32 digest of a delegate's frozen voter
// list. voters and weights are parallel slices in the same canonical
// (sorted-by-address) order that §3.4 requires the snapshot to store.
//
// The hash is domain-separated so it cannot collide with hashes computed
// from the same byte layout in another context. Layout hashed:
//
//	keccak256(
//	    domainSep ||
//	    be_uint64(len(voters)) ||
//	    for each i: voter[i].Bytes()(20B) || left_pad32(weights[i].Bytes())
//	)
//
// Empty list is well-defined and yields a fixed value (asserted by
// TestSnapshotHash_EmptyList); external verifiers pin the same bytes.
//
// If len(voters) != len(weights), the shorter slice determines the
// hashed prefix — this helper is a pure utility and does not validate
// its inputs. Callers of Pack pass the two through EventArgs.Voters /
// EventArgs.Amounts, where Pack does enforce the length invariant.
func SnapshotHash(voters []address.Address, weights []*big.Int) hash.Hash256 {
	n := len(voters)
	if n > len(weights) {
		n = len(weights)
	}
	buf := make([]byte, 0, 32+8+n*(20+32))
	buf = append(buf, snapshotDomainSeparator[:]...)
	var lenBuf [8]byte
	binary.BigEndian.PutUint64(lenBuf[:], uint64(n))
	buf = append(buf, lenBuf[:]...)
	for i := 0; i < n; i++ {
		if voters[i] == nil {
			buf = append(buf, make([]byte, 20)...)
		} else {
			buf = append(buf, voters[i].Bytes()...)
		}
		buf = append(buf, leftPad32(weights[i])...)
	}
	return hash.Hash256b(buf)
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
