// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

// Package distributedlog encodes the IIP-59 §3.2 DelegateVoterRewardsDistributed
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

// Errors returned by Pack. Both indicate a wiring bug in the caller
// (PR 3'), not on-chain data, so they hard-fail rather than degrade —
// there is no per-item fallback available at the log-encode layer.
var (
	// ErrParallelArrayLengthMismatch is returned when the voters, recipients,
	// amounts, compound bucket ID, and compounded slices in EventArgs do not have
	// identical lengths. The rewarding protocol constructs these arrays in
	// lock-step, so a mismatch is a serialisation bug.
	ErrParallelArrayLengthMismatch = errors.New(
		"distributedlog: voters, recipients, amounts, compound bucket IDs, and compounded flags must have equal length")

	// ErrNilAddress is returned when Delegate, Voters[i], or Recipients[i]
	// is nil. Passing nil is a caller-side mistake and would
	// otherwise packet as the zero address, silently masking a lost voter.
	ErrNilAddress = errors.New("distributedlog: nil address")

	// ErrNilBigInt is returned when VoterAmount or
	// any Amounts[i] is nil.
	ErrNilBigInt = errors.New("distributedlog: nil *big.Int")

	// ErrNotDelegateVoterRewardsDistributed identifies logs for another event.
	ErrNotDelegateVoterRewardsDistributed = errors.New(
		"distributedlog: not a DelegateVoterRewardsDistributed log")

	// ErrMalformedLog identifies logs carrying this event's selector whose
	// topics or data do not match its ABI.
	ErrMalformedLog = errors.New(
		"distributedlog: malformed DelegateVoterRewardsDistributed log")
)

// abiOnce guards parseABI: abi.JSON is not free and the parsed ABI is
// immutable, so we cache it for the process lifetime.
var (
	abiOnce     sync.Once
	parsedABI   abi.ABI
	abiParseErr error
)

func loadABI() (abi.ABI, error) {
	abiOnce.Do(func() {
		parsedABI, abiParseErr = abi.JSON(strings.NewReader(ABIJSON))
	})
	return parsedABI, abiParseErr
}

// ABI parses and returns an independent copy of the event ABI. Returning a
// fresh value prevents callers from mutating the maps used by Pack and Unpack.
func ABI() (abi.ABI, error) {
	parsed, err := abi.JSON(strings.NewReader(ABIJSON))
	if err != nil {
		return abi.ABI{}, errors.Wrap(err, "distributedlog: parse ABI")
	}
	return parsed, nil
}

// Topic0 returns the selector carried in Topics[0].
func Topic0() (hash.Hash256, error) {
	parsed, err := loadABI()
	if err != nil {
		return hash.ZeroHash256, errors.Wrap(err, "distributedlog: parse ABI")
	}
	ev, ok := parsed.Events[EventName]
	if !ok {
		return hash.ZeroHash256, errors.Errorf("distributedlog: event %q not found in parsed ABI", EventName)
	}
	return hash.Hash256(ev.ID), nil
}

// EventArgs are the fields of one DelegateVoterRewardsDistributed log. Field order
// matches the Solidity signature so a struct literal reads left-to-right
// the same way as the on-chain event definition. Callers build one per
// delegate touched by a voter-major settlement chunk.
type EventArgs struct {
	Epoch             uint64            // indexed → Topics[1]
	Delegate          address.Address   // indexed → Topics[2]
	VoterAmount       *big.Int          // voter rewards paid in this chunk
	Voters            []address.Address // canonical sorted order per §3.4
	Recipients        []address.Address // actual direct recipient; voter for compound payout
	Amounts           []*big.Int        // parallel to Voters
	CompoundBucketIDs []uint64          // parallel to Voters; meaningful only where Compounded is true
	Compounded        []bool            // parallel to Voters; true → paid into CompoundBucketIDs[i]
}

// Pack encodes args as an EVM-shaped receipt log. The returned Topics
// and data satisfy the same layout an EVM-emitted DelegateVoterRewardsDistributed
// event would produce, so off-chain verifiers (PR #45) can decode with
// stock ethers.js / web3.py against this event ABI.
//
// Topics layout:
//
//	[0] keccak256(EventSignature)
//	[1] uint64 epoch, left-padded to 32 bytes
//	[2] address delegate, left-padded to 32 bytes
//
// Data layout: ABI-standard tuple of the remaining (non-indexed) inputs
// in declaration order — voterAmount, voters[], recipients[], amounts[], compoundBucketIds[],
// compounded[].
//
// compounded[i] is the only valid test for "was voter i's share compounded".
// compoundBucketIds[i] == 0 is NOT that test: native bucket index 0 is a real
// bucket and a voter can legitimately be compounded into it.
func Pack(args EventArgs) (action.Topics, []byte, error) {
	if args.Delegate == nil {
		return nil, nil, errors.Wrap(ErrNilAddress, "delegate")
	}
	if args.VoterAmount == nil {
		return nil, nil, errors.Wrap(ErrNilBigInt, "voterAmount")
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
	ev, ok := parsed.Events[EventName]
	if !ok {
		return nil, nil, errors.Errorf("distributedlog: event %q not found in parsed ABI", EventName)
	}
	data, err := ev.Inputs.NonIndexed().Pack(
		args.VoterAmount,
		voterAddrs,
		recipientAddrs,
		amounts,
		args.CompoundBucketIDs,
		args.Compounded,
	)
	if err != nil {
		return nil, nil, errors.Wrap(err, "distributedlog: pack DelegateVoterRewardsDistributed data")
	}

	topics := make(action.Topics, 3)
	topics[0] = hash.Hash256(ev.ID)
	topics[1] = encodeUint64Topic(args.Epoch)
	topics[2] = hash.BytesToHash256(args.Delegate.Bytes())
	return topics, data, nil
}

// Unpack decodes a DelegateVoterRewardsDistributed log. Callers must check the
// emitting protocol address separately. A foreign selector returns
// ErrNotDelegateVoterRewardsDistributed; a matching selector with invalid
// topics or data returns ErrMalformedLog.
func Unpack(topics action.Topics, data []byte) (*EventArgs, error) {
	parsed, err := loadABI()
	if err != nil {
		return nil, errors.Wrap(err, "distributedlog: parse ABI")
	}
	ev, ok := parsed.Events[EventName]
	if !ok {
		return nil, errors.Errorf("distributedlog: event %q not found in parsed ABI", EventName)
	}
	if len(topics) == 0 {
		return nil, errors.Wrap(ErrNotDelegateVoterRewardsDistributed, "log has no topics")
	}
	if topics[0] != hash.Hash256(ev.ID) {
		return nil, errors.Wrapf(ErrNotDelegateVoterRewardsDistributed, "topics[0]=%x", topics[0])
	}
	if len(topics) != 3 {
		return nil, errors.Wrapf(ErrMalformedLog, "got %d topics, want 3", len(topics))
	}

	epoch, err := decodeUint64Topic(topics[1])
	if err != nil {
		return nil, errors.Wrap(err, "distributedlog: decode epoch topic")
	}
	delegate, err := decodeAddressTopic(topics[2])
	if err != nil {
		return nil, errors.Wrap(err, "distributedlog: decode delegate topic")
	}

	values, err := ev.Inputs.NonIndexed().Unpack(data)
	if err != nil {
		return nil, errors.Wrapf(ErrMalformedLog,
			"unpack DelegateVoterRewardsDistributed data: %v", err)
	}
	if len(values) != 6 {
		return nil, errors.Wrapf(ErrMalformedLog, "got %d non-indexed values, want 6", len(values))
	}
	voterAmount, ok := values[0].(*big.Int)
	if !ok {
		return nil, errors.Wrapf(ErrMalformedLog, "voterAmount has type %T", values[0])
	}
	voters, err := fromEthAddresses(values[1], "voters")
	if err != nil {
		return nil, err
	}
	recipients, err := fromEthAddresses(values[2], "recipients")
	if err != nil {
		return nil, err
	}
	amounts, ok := values[3].([]*big.Int)
	if !ok {
		return nil, errors.Wrapf(ErrMalformedLog, "amounts has type %T", values[3])
	}
	compoundBucketIDs, ok := values[4].([]uint64)
	if !ok {
		return nil, errors.Wrapf(ErrMalformedLog, "compoundBucketIds has type %T", values[4])
	}
	compounded, ok := values[5].([]bool)
	if !ok {
		return nil, errors.Wrapf(ErrMalformedLog, "compounded has type %T", values[5])
	}
	if len(voters) != len(recipients) || len(voters) != len(amounts) ||
		len(voters) != len(compoundBucketIDs) || len(voters) != len(compounded) {
		return nil, errors.Wrapf(ErrParallelArrayLengthMismatch,
			"voters=%d recipients=%d amounts=%d compoundBucketIds=%d compounded=%d",
			len(voters), len(recipients), len(amounts), len(compoundBucketIDs), len(compounded))
	}

	return &EventArgs{
		Epoch:             epoch,
		Delegate:          delegate,
		VoterAmount:       voterAmount,
		Voters:            voters,
		Recipients:        recipients,
		Amounts:           amounts,
		CompoundBucketIDs: compoundBucketIDs,
		Compounded:        compounded,
	}, nil
}

func decodeUint64Topic(t hash.Hash256) (uint64, error) {
	for _, b := range t[:24] {
		if b != 0 {
			return 0, errors.Wrapf(ErrMalformedLog, "uint64 topic has non-zero padding: %x", t)
		}
	}
	return binary.BigEndian.Uint64(t[24:]), nil
}

func decodeAddressTopic(t hash.Hash256) (address.Address, error) {
	for _, b := range t[:12] {
		if b != 0 {
			return nil, errors.Wrapf(ErrMalformedLog, "address topic has non-zero padding: %x", t)
		}
	}
	addr, err := address.FromBytes(t[12:])
	if err != nil {
		return nil, errors.Wrap(ErrMalformedLog, err.Error())
	}
	return addr, nil
}

func fromEthAddresses(v any, field string) ([]address.Address, error) {
	ethAddrs, ok := v.([]common.Address)
	if !ok {
		return nil, errors.Wrapf(ErrMalformedLog, "%s has type %T", field, v)
	}
	out := make([]address.Address, len(ethAddrs))
	for i, a := range ethAddrs {
		addr, err := address.FromBytes(a.Bytes())
		if err != nil {
			return nil, errors.Wrapf(ErrMalformedLog, "decode %s[%d]: %v", field, i, err)
		}
		out[i] = addr
	}
	return out, nil
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
