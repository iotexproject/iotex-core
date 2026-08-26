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
	voters := []address.Address{
		identityset.Address(3),
		identityset.Address(4),
		identityset.Address(5),
	}
	return EventArgs{
		Epoch:       4200,
		Delegate:    delegate,
		VoterAmount: big.NewInt(9_000_000),
		Voters:      voters,
		Recipients: []address.Address{
			voters[0],
			voters[1],
			identityset.Address(6),
		},
		Amounts: []*big.Int{
			big.NewInt(3_000_000),
			big.NewInt(3_000_000),
			big.NewInt(3_000_000),
		},
		// voters[0] is a real compound into native bucket 0 -- the case the
		// zero-as-sentinel encoding could not express. Compounded[0] is what
		// makes it distinguishable from voters[2]'s direct credit.
		CompoundBucketIDs: []uint64{0, 42, 0},
		Compounded:        []bool{true, true, false},
	}
}

func TestPack_HappyPath(t *testing.T) {
	r := require.New(t)
	args := happyArgs()

	topics, data, err := Pack(args)
	r.NoError(err)
	r.Len(topics, 3, "one selector + two indexed args")

	// Topics[0] must equal keccak256(EventSignature); the golden test
	// below pins the exact bytes, but sanity-check the derivation here.
	r.Equal(hash.Hash256(crypto.Keccak256Hash([]byte(EventSignature))), topics[0])

	// Topics[1] is the 32-byte left-padded epoch.
	var epochWord [32]byte
	binary.BigEndian.PutUint64(epochWord[24:], args.Epoch)
	r.Equal(hash.BytesToHash256(epochWord[:]), topics[1])

	// Topics[2] is the delegate's 20-byte address left-padded to 32.
	r.Equal(hash.BytesToHash256(args.Delegate.Bytes()), topics[2])

	// Data must be a valid ABI-encoded tuple — parse it back and compare.
	parsed, err := abi.JSON(strings.NewReader(ABIJSON))
	r.NoError(err)
	unpacked, err := parsed.Events[EventName].Inputs.NonIndexed().Unpack(data)
	r.NoError(err)
	r.Len(unpacked, 6)
	r.Equal(0, args.VoterAmount.Cmp(unpacked[0].(*big.Int)))
}

func TestPack_ZeroVoters(t *testing.T) {
	// A delegate with no voters still emits a log — a batched log per
	// delegate is the observability contract, even when the batch is
	// empty. Guard against a premature len(voters)==0 short-circuit.
	r := require.New(t)
	args := happyArgs()
	args.Voters = nil
	args.Recipients = nil
	args.Amounts = nil
	args.CompoundBucketIDs = nil
	args.Compounded = nil

	topics, data, err := Pack(args)
	r.NoError(err)
	r.Len(topics, 3)

	parsed, err := abi.JSON(strings.NewReader(ABIJSON))
	r.NoError(err)
	unpacked, err := parsed.Events[EventName].Inputs.NonIndexed().Unpack(data)
	r.NoError(err)
	r.Len(unpacked[1].([]common.Address), 0, "voters must decode as empty array")
	r.Len(unpacked[2].([]common.Address), 0, "recipients must decode as empty array")
	r.Len(unpacked[3].([]*big.Int), 0, "amounts must decode as empty array")
	r.Len(unpacked[4].([]uint64), 0, "compound bucket IDs must decode as empty array")
	r.Len(unpacked[5].([]bool), 0, "compounded flags must decode as empty array")
}

func TestPack_ParallelLengthMismatch_Recipients(t *testing.T) {
	r := require.New(t)
	args := happyArgs()
	args.Recipients = args.Recipients[:2]

	_, _, err := Pack(args)
	r.ErrorIs(err, ErrParallelArrayLengthMismatch)
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

func TestPack_ParallelLengthMismatch_Compounded(t *testing.T) {
	r := require.New(t)
	args := happyArgs()
	args.Compounded = args.Compounded[:2] // one short

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

func TestPack_NilVoterAmount(t *testing.T) {
	r := require.New(t)
	args := happyArgs()
	args.VoterAmount = nil

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

func TestPack_NilRecipientAddress(t *testing.T) {
	r := require.New(t)
	args := happyArgs()
	args.Recipients[0] = nil

	_, _, err := Pack(args)
	r.ErrorIs(err, ErrNilAddress)
	r.Contains(err.Error(), "recipients[0]", "error must name offending index")
}

func TestPack_SelectorPinned(t *testing.T) {
	// Pin the exact 32-byte selector. If this test breaks, either the
	// event signature drifted (spec change → requires a new selector and
	// coordinated verifier update) or a whitespace typo crept into
	// EventSignature.
	r := require.New(t)
	args := happyArgs()
	topics, _, err := Pack(args)
	r.NoError(err)

	expected := crypto.Keccak256Hash([]byte(
		"DelegateVoterRewardsDistributed(uint64,address,uint256,address[],address[],uint256[],uint64[],bool[])",
	))
	r.Equal(hash.Hash256(expected), topics[0])
}

// TestPack_BucketZeroIsDistinguishable is the R7 regression: a voter
// compounded into native bucket 0 and a voter paid directly both carry
// compoundBucketIds[i] == 0, so the bucket ID alone cannot tell them apart.
// compounded[i] must.
func TestPack_BucketZeroIsDistinguishable(t *testing.T) {
	r := require.New(t)
	args := happyArgs()
	topics, data, err := Pack(args)
	r.NoError(err)
	r.Len(topics, 3)

	parsed, err := abi.JSON(strings.NewReader(ABIJSON))
	r.NoError(err)
	unpacked, err := parsed.Events[EventName].Inputs.NonIndexed().Unpack(data)
	r.NoError(err)

	bucketIDs := unpacked[4].([]uint64)
	compounded := unpacked[5].([]bool)
	r.Equal([]uint64{0, 42, 0}, bucketIDs)
	r.Equal([]bool{true, true, false}, compounded)
	// Voter 0 and voter 2 are indistinguishable by bucket ID...
	r.Equal(bucketIDs[0], bucketIDs[2])
	// ...and distinguishable only by the compounded flag.
	r.NotEqual(compounded[0], compounded[2])
}

func TestPack_RoundTrip(t *testing.T) {
	// Full byte-level round trip: encode via Pack, decode via the same
	// ABI, verify every field equals the input. This is the contract
	// PR #45's off-chain verifier depends on.
	r := require.New(t)
	args := happyArgs()

	topics, data, err := Pack(args)
	r.NoError(err)

	parsed, err := abi.JSON(strings.NewReader(ABIJSON))
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
	unpacked, err := parsed.Events[EventName].Inputs.NonIndexed().Unpack(data)
	r.NoError(err)
	r.Equal(0, args.VoterAmount.Cmp(unpacked[0].(*big.Int)))

	votersOut := unpacked[1].([]common.Address)
	r.Len(votersOut, len(args.Voters))
	for i, v := range args.Voters {
		r.Equal(common.BytesToAddress(v.Bytes()), votersOut[i])
	}
	recipientsOut := unpacked[2].([]common.Address)
	r.Len(recipientsOut, len(args.Recipients))
	for i, recipient := range args.Recipients {
		r.Equal(common.BytesToAddress(recipient.Bytes()), recipientsOut[i])
	}
	amountsOut := unpacked[3].([]*big.Int)
	r.Len(amountsOut, len(args.Amounts))
	for i, a := range args.Amounts {
		r.Equal(0, a.Cmp(amountsOut[i]))
	}
	bucketIDsOut := unpacked[4].([]uint64)
	r.Len(bucketIDsOut, len(args.CompoundBucketIDs))
	for i, bucketID := range args.CompoundBucketIDs {
		r.Equal(bucketID, bucketIDsOut[i])
	}
	compoundedOut := unpacked[5].([]bool)
	r.Equal(args.Compounded, compoundedOut)
}
