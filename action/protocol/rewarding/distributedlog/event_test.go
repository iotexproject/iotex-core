// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package distributedlog

import (
	"encoding/binary"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// happyArgs returns a plausible three-voter EventArgs with mixed
// destinations. Each test mutates one field to isolate the property under
// test.
func happyArgs() EventArgs {
	delegate := identityset.Address(1)
	reward := identityset.Address(2)
	voters := []address.Address{
		identityset.Address(3),
		identityset.Address(4),
		identityset.Address(5),
	}
	return EventArgs{
		Epoch:           4200,
		Delegate:        delegate,
		RewardAddr:      reward,
		TotalCommission: big.NewInt(1_000_000),
		TotalVoterPool:  big.NewInt(9_000_000),
		SnapshotHash:    hash.Hash256b([]byte("snapshot@epoch4200")),
		Voters:          voters,
		Amounts: []*big.Int{
			big.NewInt(3_000_000),
			big.NewInt(3_000_000),
			big.NewInt(3_000_000),
		},
		CompoundBucketIDs: []uint64{0, 42, 99},
	}
}

func TestPack_HappyPath(t *testing.T) {
	r := require.New(t)
	args := happyArgs()

	topics, data, err := Pack(args)
	r.NoError(err)
	r.Len(topics, 3, "one selector + two indexed args")

	// Topics[0] must equal keccak256(eventSignature); the golden test
	// below pins the exact bytes, but sanity-check the derivation here.
	r.Equal(hash.Hash256(crypto.Keccak256Hash([]byte(eventSignature))), topics[0])

	// Topics[1] is the 32-byte left-padded epoch.
	var epochWord [32]byte
	binary.BigEndian.PutUint64(epochWord[24:], args.Epoch)
	r.Equal(hash.BytesToHash256(epochWord[:]), topics[1])

	// Topics[2] is the delegate's 20-byte address left-padded to 32.
	r.Equal(hash.BytesToHash256(args.Delegate.Bytes()), topics[2])

	// Data must be a valid ABI-encoded tuple — parse it back and compare.
	parsed, err := abi.JSON(strings.NewReader(abiJSON))
	r.NoError(err)
	unpacked, err := parsed.Events[eventName].Inputs.NonIndexed().Unpack(data)
	r.NoError(err)
	r.Len(unpacked, 7)
	r.Equal(common.BytesToAddress(args.RewardAddr.Bytes()), unpacked[0])
	r.Equal(0, args.TotalCommission.Cmp(unpacked[1].(*big.Int)))
	r.Equal(0, args.TotalVoterPool.Cmp(unpacked[2].(*big.Int)))
	r.Equal([32]byte(args.SnapshotHash), unpacked[3])
}

func TestPack_ZeroVoters(t *testing.T) {
	// A delegate with no voters still emits a log — a batched log per
	// delegate is the observability contract, even when the batch is
	// empty. Guard against a premature len(voters)==0 short-circuit.
	r := require.New(t)
	args := happyArgs()
	args.Voters = nil
	args.Amounts = nil
	args.CompoundBucketIDs = nil

	topics, data, err := Pack(args)
	r.NoError(err)
	r.Len(topics, 3)

	parsed, err := abi.JSON(strings.NewReader(abiJSON))
	r.NoError(err)
	unpacked, err := parsed.Events[eventName].Inputs.NonIndexed().Unpack(data)
	r.NoError(err)
	r.Len(unpacked[4].([]common.Address), 0, "voters must decode as empty array")
	r.Len(unpacked[5].([]*big.Int), 0, "amounts must decode as empty array")
	r.Len(unpacked[6].([]uint64), 0, "compound bucket IDs must decode as empty array")
}

func TestPack_ParallelLengthMismatch_Amounts(t *testing.T) {
	r := require.New(t)
	args := happyArgs()
	args.Amounts = args.Amounts[:2] // one short

	_, _, err := Pack(args)
	r.ErrorIs(err, ErrParallelArrayLengthMismatch)
}

func TestPack_ParallelLengthMismatch_CompoundBucketIDs(t *testing.T) {
	r := require.New(t)
	args := happyArgs()
	args.CompoundBucketIDs = args.CompoundBucketIDs[:1] // two short

	_, _, err := Pack(args)
	r.ErrorIs(err, ErrParallelArrayLengthMismatch)
}

func TestPack_NilDelegate(t *testing.T) {
	r := require.New(t)
	args := happyArgs()
	args.Delegate = nil

	_, _, err := Pack(args)
	r.ErrorIs(err, ErrNilAddress)
}

func TestPack_NilRewardAddr(t *testing.T) {
	r := require.New(t)
	args := happyArgs()
	args.RewardAddr = nil

	_, _, err := Pack(args)
	r.ErrorIs(err, ErrNilAddress)
}

func TestPack_NilTotalCommission(t *testing.T) {
	r := require.New(t)
	args := happyArgs()
	args.TotalCommission = nil

	_, _, err := Pack(args)
	r.ErrorIs(err, ErrNilBigInt)
}

func TestPack_NilTotalVoterPool(t *testing.T) {
	r := require.New(t)
	args := happyArgs()
	args.TotalVoterPool = nil

	_, _, err := Pack(args)
	r.ErrorIs(err, ErrNilBigInt)
}

func TestPack_NilPerVoterAmount(t *testing.T) {
	// Nil per-voter amount would silently pack as zero, dropping a voter
	// from the split off-chain reconstruction while still crediting them
	// on-chain elsewhere — must fail loudly.
	r := require.New(t)
	args := happyArgs()
	args.Amounts[1] = nil

	_, _, err := Pack(args)
	r.ErrorIs(err, ErrNilBigInt)
	r.Contains(err.Error(), "amounts[1]", "error must name offending index")
}

func TestPack_NilPerVoterAddress(t *testing.T) {
	r := require.New(t)
	args := happyArgs()
	args.Voters[0] = nil

	_, _, err := Pack(args)
	r.ErrorIs(err, ErrNilAddress)
	r.Contains(err.Error(), "voters[0]", "error must name offending index")
}

func TestPack_SelectorPinned(t *testing.T) {
	// Pin the exact 32-byte selector. If this test breaks, either the
	// event signature drifted (spec change → requires a new selector and
	// coordinated verifier update) or a whitespace typo crept into
	// eventSignature.
	r := require.New(t)
	args := happyArgs()
	topics, _, err := Pack(args)
	r.NoError(err)

	expected := crypto.Keccak256Hash([]byte(
		"DelegateDistributed(uint64,address,address,uint256,uint256,bytes32,address[],uint256[],uint64[])",
	))
	r.Equal(hash.Hash256(expected), topics[0])
}

func TestPack_RoundTrip(t *testing.T) {
	// Full byte-level round trip: encode via Pack, decode via the same
	// ABI, verify every field equals the input. This is the contract
	// PR #45's off-chain verifier depends on.
	r := require.New(t)
	args := happyArgs()

	topics, data, err := Pack(args)
	r.NoError(err)

	parsed, err := abi.JSON(strings.NewReader(abiJSON))
	r.NoError(err)

	// Topics[1] round-trip → epoch.
	var epochOut uint64
	epochWord := topics[1]
	epochOut = binary.BigEndian.Uint64(epochWord[24:])
	r.Equal(args.Epoch, epochOut)

	// Topics[2] round-trip → delegate.
	delegateOut := topics[2]
	r.Equal(args.Delegate.Bytes(), delegateOut[12:])

	// Data round-trip: every non-indexed field.
	unpacked, err := parsed.Events[eventName].Inputs.NonIndexed().Unpack(data)
	r.NoError(err)
	r.Equal(common.BytesToAddress(args.RewardAddr.Bytes()), unpacked[0])
	r.Equal(0, args.TotalCommission.Cmp(unpacked[1].(*big.Int)))
	r.Equal(0, args.TotalVoterPool.Cmp(unpacked[2].(*big.Int)))
	r.Equal([32]byte(args.SnapshotHash), unpacked[3])

	votersOut := unpacked[4].([]common.Address)
	r.Len(votersOut, len(args.Voters))
	for i, v := range args.Voters {
		r.Equal(common.BytesToAddress(v.Bytes()), votersOut[i])
	}
	amountsOut := unpacked[5].([]*big.Int)
	r.Len(amountsOut, len(args.Amounts))
	for i, a := range args.Amounts {
		r.Equal(0, a.Cmp(amountsOut[i]))
	}
	bucketIDsOut := unpacked[6].([]uint64)
	r.Len(bucketIDsOut, len(args.CompoundBucketIDs))
	for i, bucketID := range args.CompoundBucketIDs {
		r.Equal(bucketID, bucketIDsOut[i])
	}
}

func TestSnapshotHash_Determinism(t *testing.T) {
	// Same input twice → same hash; reversed order → different hash.
	// Canonical ordering is load-bearing per §3.4.
	r := require.New(t)
	voters := []address.Address{
		identityset.Address(3),
		identityset.Address(4),
		identityset.Address(5),
	}
	weights := []*big.Int{big.NewInt(100), big.NewInt(200), big.NewInt(300)}

	h1 := SnapshotHash(voters, weights)
	h2 := SnapshotHash(voters, weights)
	r.Equal(h1, h2, "same input must hash identically")

	reversedVoters := []address.Address{voters[2], voters[1], voters[0]}
	reversedWeights := []*big.Int{weights[2], weights[1], weights[0]}
	h3 := SnapshotHash(reversedVoters, reversedWeights)
	r.NotEqual(h1, h3, "reversed order must hash differently")
}

func TestSnapshotHash_EmptyList(t *testing.T) {
	// A zero-voter delegate still needs a well-defined snapshot hash.
	// This asserts the exact bytes so external verifiers can pin them.
	r := require.New(t)

	empty := SnapshotHash(nil, nil)

	// Domain separator || big-endian uint64(0) — no per-voter payload.
	var expectedBuf []byte
	expectedBuf = append(expectedBuf, snapshotDomainSeparator[:]...)
	expectedBuf = append(expectedBuf, make([]byte, 8)...)
	expected := hash.Hash256b(expectedBuf)
	r.Equal(expected, empty)
}

func TestSnapshotHash_MismatchedLengthTruncates(t *testing.T) {
	// SnapshotHash is a pure utility; length invariants live in Pack.
	// If len(voters) > len(weights), the extra voters are ignored (the
	// shorter slice bounds the loop). Guards against surprises for
	// callers who bypass Pack.
	r := require.New(t)
	voters := []address.Address{
		identityset.Address(3),
		identityset.Address(4),
	}
	weights := []*big.Int{big.NewInt(100)} // one short

	h1 := SnapshotHash(voters, weights)
	h2 := SnapshotHash(voters[:1], weights)
	r.Equal(h1, h2, "extra voters must be ignored, not silently zero-weighted")
}
