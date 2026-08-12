// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package distributedlog

import (
	"math/big"
	"testing"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// TestUnpackRoundTrip is the property that matters: whatever Pack emits,
// Unpack must return field-for-field. It is asserted on every field rather
// than with a struct compare so a failure names the field that drifted.
//
// The parallel arrays are the reason this test exists. Their meaning is
// entirely positional -- voters[i] pairs with recipients[i], amounts[i],
// compoundBucketIds[i] and compounded[i] -- so an encoder/decoder ordering
// mismatch produces a log that decodes cleanly and reports wrong payouts.
func TestUnpackRoundTrip(t *testing.T) {
	r := require.New(t)
	args := happyArgs()

	topics, data, err := Pack(args)
	r.NoError(err)

	got, err := Unpack(topics, data)
	r.NoError(err)

	r.Equal(args.Epoch, got.Epoch)
	r.Equal(args.Delegate.String(), got.Delegate.String())
	r.Equal(args.RewardAddr.String(), got.RewardAddr.String())
	r.Zero(args.EraCommission.Cmp(got.EraCommission))
	r.Zero(args.ChunkVoterReward.Cmp(got.ChunkVoterReward))
	r.Equal(args.SnapshotHash, got.SnapshotHash)

	r.Len(got.Voters, len(args.Voters))
	for i := range args.Voters {
		r.Equal(args.Voters[i].String(), got.Voters[i].String(), "voters[%d]", i)
		r.Equal(args.Recipients[i].String(), got.Recipients[i].String(), "recipients[%d]", i)
		r.Zero(args.Amounts[i].Cmp(got.Amounts[i]), "amounts[%d]", i)
		r.Equal(args.CompoundBucketIDs[i], got.CompoundBucketIDs[i], "compoundBucketIds[%d]", i)
		r.Equal(args.Compounded[i], got.Compounded[i], "compounded[%d]", i)
	}
}

// TestUnpackPreservesBucketZeroCompound guards the one encoding subtlety a
// consumer is most likely to get wrong. Native bucket index 0 is a real
// bucket, so compoundBucketIds[i] == 0 does NOT mean "not compounded" --
// compounded[i] is the only valid discriminator. happyArgs deliberately puts a
// genuine compound-into-bucket-0 at index 0 and a non-compounded voter, also
// carrying bucket id 0, at index 2.
func TestUnpackPreservesBucketZeroCompound(t *testing.T) {
	r := require.New(t)

	topics, data, err := Pack(happyArgs())
	r.NoError(err)
	got, err := Unpack(topics, data)
	r.NoError(err)

	r.Equal(uint64(0), got.CompoundBucketIDs[0])
	r.True(got.Compounded[0], "a real compound into bucket 0 must survive the round trip")
	r.Equal(uint64(0), got.CompoundBucketIDs[2])
	r.False(got.Compounded[2], "bucket id 0 alone must not read as compounded")
}

// TestUnpackEmptyVoterList covers the degenerate log Pack still accepts.
func TestUnpackEmptyVoterList(t *testing.T) {
	r := require.New(t)
	args := happyArgs()
	args.Voters = nil
	args.Recipients = nil
	args.Amounts = nil
	args.CompoundBucketIDs = nil
	args.Compounded = nil

	topics, data, err := Pack(args)
	r.NoError(err)
	got, err := Unpack(topics, data)
	r.NoError(err)
	r.Empty(got.Voters)
	r.Equal(args.Epoch, got.Epoch)
}

// TestUnpackRejectsForeignEvent pins the skip/alert split. A log from some
// other event must come back as ErrNotDelegateDistributed so an indexer
// scanning every log in a block can skip it silently, without that outcome
// being confusable with a corrupt log of our own.
func TestUnpackRejectsForeignEvent(t *testing.T) {
	r := require.New(t)
	_, data, err := Pack(happyArgs())
	r.NoError(err)

	t.Run("wrong topic0", func(t *testing.T) {
		topics := action.Topics{
			hash.Hash256b([]byte("SomeOtherEvent(uint256)")),
			hash.Hash256{},
			hash.Hash256{},
		}
		_, err := Unpack(topics, data)
		require.ErrorIs(t, err, ErrNotDelegateDistributed)
	})

	t.Run("wrong topic count", func(t *testing.T) {
		topics, _, err := Pack(happyArgs())
		require.NoError(t, err)
		_, err = Unpack(topics[:2], data)
		require.ErrorIs(t, err, ErrNotDelegateDistributed)
	})
}

// TestUnpackRejectsMalformedTopicPadding covers the silent-truncation hazard.
// Both indexed values are left-padded into 32 bytes; returning just the low
// bytes of a topic whose padding is dirty would turn a malformed log into a
// plausible small epoch or a valid-looking address.
func TestUnpackRejectsMalformedTopicPadding(t *testing.T) {
	r := require.New(t)

	t.Run("epoch", func(t *testing.T) {
		topics, data, err := Pack(happyArgs())
		require.NoError(t, err)
		topics[1][0] = 0x01
		_, err = Unpack(topics, data)
		require.ErrorIs(t, err, ErrMalformedLog)
	})

	t.Run("delegate", func(t *testing.T) {
		topics, data, err := Pack(happyArgs())
		require.NoError(t, err)
		topics[2][0] = 0x01
		_, err = Unpack(topics, data)
		require.ErrorIs(t, err, ErrMalformedLog)
	})

	// Sanity: the unmutated log decodes, so the assertions above are not
	// passing for some unrelated reason.
	topics, data, err := Pack(happyArgs())
	r.NoError(err)
	_, err = Unpack(topics, data)
	r.NoError(err)
}

// TestUnpackRejectsTruncatedData ensures a short payload is an error rather
// than a partially populated EventArgs.
func TestUnpackRejectsTruncatedData(t *testing.T) {
	r := require.New(t)
	topics, data, err := Pack(happyArgs())
	r.NoError(err)

	_, err = Unpack(topics, data[:len(data)/2])
	r.Error(err)
	r.NotErrorIs(err, ErrNotDelegateDistributed, "a truncated payload of our own event is corruption, not a foreign event")
	r.ErrorIs(err, ErrMalformedLog)
}

// TestTopic0MatchesPackedLog pins the filter an indexer uses against what Pack
// actually emits. If these ever diverge the indexer silently sees no events.
func TestTopic0MatchesPackedLog(t *testing.T) {
	r := require.New(t)

	topic0, err := Topic0()
	r.NoError(err)

	topics, _, err := Pack(happyArgs())
	r.NoError(err)
	r.Equal(topics[0], topic0)

	r.Equal(hash.Hash256b([]byte(EventSignature)), hash.Hash256(topic0),
		"Topic0 must be keccak256(EventSignature)")
}

// TestABIExportedIsParseable guards the exported ABI surface: a consumer that
// parses ABIJSON itself must find the same event this package encodes.
func TestABIExportedIsParseable(t *testing.T) {
	r := require.New(t)

	parsed, err := ABI()
	r.NoError(err)
	ev, ok := parsed.Events[EventName]
	r.True(ok, "EventName must resolve in the exported ABI")
	r.Equal(EventSignature, ev.Sig)
}

func TestABICallerCannotMutateEncoderCache(t *testing.T) {
	r := require.New(t)
	parsed, err := ABI()
	r.NoError(err)
	delete(parsed.Events, EventName)

	_, _, err = Pack(happyArgs())
	r.NoError(err)
	_, ok := parsed.Events[EventName]
	r.False(ok, "the caller's ABI remains independently mutable")
}

// TestUnpackDoesNotAliasPackInput is a defensive check that Unpack returns
// independent big.Ints -- a consumer accumulating into the returned values
// must not corrupt anything the encoder still holds.
func TestUnpackDoesNotAliasPackInput(t *testing.T) {
	r := require.New(t)
	args := happyArgs()
	original := new(big.Int).Set(args.Amounts[0])

	topics, data, err := Pack(args)
	r.NoError(err)
	got, err := Unpack(topics, data)
	r.NoError(err)

	got.Amounts[0].Add(got.Amounts[0], big.NewInt(1))
	r.Zero(original.Cmp(args.Amounts[0]), "mutating the decoded amount must not touch the input")
}

// TestUnpackUnknownDelegateAddressStillDecodes confirms Unpack does not care
// whether the delegate is a known identity -- it decodes whatever address the
// topic carries.
func TestUnpackUnknownDelegateAddressStillDecodes(t *testing.T) {
	r := require.New(t)
	args := happyArgs()
	args.Delegate = identityset.Address(20)

	topics, data, err := Pack(args)
	r.NoError(err)
	got, err := Unpack(topics, data)
	r.NoError(err)
	r.Equal(identityset.Address(20).String(), got.Delegate.String())
}
