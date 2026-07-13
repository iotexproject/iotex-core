// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package delegateprofile

import (
	"context"
	"encoding/binary"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// mainnetContract is the pinned mainnet DelegateProfile deployment. Tests use
// it verbatim so that a rename in production would be caught here too.
const mainnetContract = "io1lfl4ppn2c3wcft04f0rk0jy9lyn4pcjcm7638u"

// fakeStore is a keyed lookup table simulating on-chain field storage. Each
// key is (delegate-eth-address, field-name); the value is the raw bytes that
// getProfileByField(_delegate, _field) would return to a caller ABI-decoded
// as `bytes`. Missing entries return empty bytes, matching contract behaviour
// for unset fields.
type fakeStore struct {
	values map[string][]byte
}

func newFakeStore() *fakeStore { return &fakeStore{values: map[string][]byte{}} }

func (s *fakeStore) set(delegate address.Address, field string, portionBP uint64) {
	// Contract encodes the field value as big-endian uint256 bytes. Any
	// leading zeroes are trimmed by the on-chain encoding; encode without
	// zero-padding to mimic that.
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, portionBP)
	trimmed := buf
	for len(trimmed) > 0 && trimmed[0] == 0 {
		trimmed = trimmed[1:]
	}
	s.values[storeKey(delegate, field)] = trimmed
}

func (s *fakeStore) setRaw(delegate address.Address, field string, raw []byte) {
	s.values[storeKey(delegate, field)] = raw
}

func storeKey(delegate address.Address, field string) string {
	return common.BytesToAddress(delegate.Bytes()).Hex() + "|" + field
}

// reader turns the fakeStore into a ContractReader by decoding call data
// against the same ABI the bridge uses, then packing the response the same
// way go-ethereum does.
func (s *fakeStore) reader(t *testing.T) ContractReader {
	t.Helper()
	parsed, err := abi.JSON(strings.NewReader(abiJSON))
	require.NoError(t, err)
	return ContractReaderFunc(func(_ context.Context, contract string, callData []byte) ([]byte, error) {
		if contract == "" {
			return nil, errors.New("empty contract")
		}
		if len(callData) < 4 {
			return nil, errors.New("truncated call data")
		}
		method, err := parsed.MethodById(callData[:4])
		if err != nil {
			return nil, err
		}
		args, err := method.Inputs.Unpack(callData[4:])
		if err != nil {
			return nil, err
		}
		require.Equal(t, "getProfileByField", method.Name)
		require.Len(t, args, 2)
		delegateEth := args[0].(common.Address)
		field := args[1].(string)
		addr, err := address.FromBytes(delegateEth.Bytes())
		if err != nil {
			return nil, err
		}
		value := s.values[storeKey(addr, field)]
		return method.Outputs.Pack(value)
	})
}

func TestNew(t *testing.T) {
	r := require.New(t)

	t.Run("empty contract rejected", func(t *testing.T) {
		_, err := New("")
		r.ErrorIs(err, ErrEmptyContractAddress)
	})

	t.Run("garbage address rejected", func(t *testing.T) {
		_, err := New("not-a-bech32-address")
		r.Error(err)
	})

	t.Run("valid address accepted", func(t *testing.T) {
		b, err := New(mainnetContract)
		r.NoError(err)
		r.Equal(mainnetContract, b.Contract())
	})
}

func TestSnapshot_RegisteredDelegate(t *testing.T) {
	r := require.New(t)
	b, err := New(mainnetContract)
	r.NoError(err)

	delegate := identityset.Address(1)
	store := newFakeStore()
	// voter-take 90.00% on block reward, 80.00% on epoch reward.
	store.set(delegate, fieldBlockRewardPortion, 9000)
	store.set(delegate, fieldEpochRewardPortion, 8000)

	out, err := b.Snapshot(context.Background(), store.reader(t), []address.Address{delegate})
	r.NoError(err)
	r.Len(out, 1)
	rates := out[delegate.String()]
	r.NotNil(rates)
	r.True(rates.Registered)
	r.Equal(uint64(1000), rates.BlockCommissionBasisPoints) // 10000 - 9000
	r.Equal(uint64(2000), rates.EpochCommissionBasisPoints) // 10000 - 8000
}

func TestSnapshot_UnregisteredDelegate(t *testing.T) {
	r := require.New(t)
	b, err := New(mainnetContract)
	r.NoError(err)

	delegate := identityset.Address(2)
	store := newFakeStore() // both fields empty

	out, err := b.Snapshot(context.Background(), store.reader(t), []address.Address{delegate})
	r.NoError(err)
	r.Len(out, 1)
	rates := out[delegate.String()]
	r.NotNil(rates)
	r.False(rates.Registered, "unregistered delegate must fall back to legacy path")
	r.Zero(rates.BlockCommissionBasisPoints)
	r.Zero(rates.EpochCommissionBasisPoints)
}

func TestSnapshot_PartialProfileIsUnregistered(t *testing.T) {
	// A partial profile (one field set, other missing) is deliberately treated
	// as unregistered — the IIP relies on the "either fully opted-in or fully
	// legacy" invariant so that a delegate cannot end up with only one reward
	// stream migrated mid-fork.
	r := require.New(t)
	b, err := New(mainnetContract)
	r.NoError(err)

	delegate := identityset.Address(3)
	store := newFakeStore()
	store.set(delegate, fieldBlockRewardPortion, 5000)
	// epoch portion left unset

	out, err := b.Snapshot(context.Background(), store.reader(t), []address.Address{delegate})
	r.NoError(err)
	rates := out[delegate.String()]
	r.False(rates.Registered)
	r.Zero(rates.BlockCommissionBasisPoints)
	r.Zero(rates.EpochCommissionBasisPoints)
}

func TestSnapshot_ExplicitZeroVoterTakeIs100PercentCommission(t *testing.T) {
	// A delegate who explicitly sets voter-take-0% keeps 100% for themselves.
	// This must be distinguishable from "field never set" (unregistered).
	// getProfileByField returns non-empty bytes for an explicit zero (the
	// caller stored a 0-byte or wrote uint(0) = single 0x00 byte).
	r := require.New(t)
	b, err := New(mainnetContract)
	r.NoError(err)

	delegate := identityset.Address(4)
	store := newFakeStore()
	// Explicit zero — non-empty byte slice representing uint(0).
	store.setRaw(delegate, fieldBlockRewardPortion, []byte{0x00})
	store.setRaw(delegate, fieldEpochRewardPortion, []byte{0x00})

	out, err := b.Snapshot(context.Background(), store.reader(t), []address.Address{delegate})
	r.NoError(err)
	rates := out[delegate.String()]
	r.True(rates.Registered, "explicit zero voter-take is a valid registered profile")
	r.Equal(uint64(10000), rates.BlockCommissionBasisPoints)
	r.Equal(uint64(10000), rates.EpochCommissionBasisPoints)
}

func TestSnapshot_OutOfRangeDegradesToUnregistered(t *testing.T) {
	// A malformed on-chain value must NOT halt the block. The bridge degrades
	// the affected delegate to Registered=false; rewarding routes it via the
	// legacy path. Same on-chain state ⇒ same fallback on every validator ⇒
	// no fork.
	r := require.New(t)
	b, err := New(mainnetContract)
	r.NoError(err)

	delegate := identityset.Address(5)
	store := newFakeStore()
	store.set(delegate, fieldBlockRewardPortion, 10001)
	store.set(delegate, fieldEpochRewardPortion, 5000)

	got, err := b.Snapshot(context.Background(), store.reader(t), []address.Address{delegate})
	r.NoError(err)
	r.Len(got, 1)
	rates := got[delegate.String()]
	r.NotNil(rates)
	r.False(rates.Registered)
	r.Zero(rates.BlockCommissionBasisPoints)
	r.Zero(rates.EpochCommissionBasisPoints)
}

func TestSnapshot_LargeBigIntDegradesToUnregistered(t *testing.T) {
	r := require.New(t)
	b, err := New(mainnetContract)
	r.NoError(err)

	delegate := identityset.Address(6)
	store := newFakeStore()
	huge := make([]byte, 33)
	huge[0] = 0x01
	store.setRaw(delegate, fieldBlockRewardPortion, huge)
	store.set(delegate, fieldEpochRewardPortion, 5000)

	got, err := b.Snapshot(context.Background(), store.reader(t), []address.Address{delegate})
	r.NoError(err)
	rates := got[delegate.String()]
	r.NotNil(rates)
	r.False(rates.Registered)
}

func TestSnapshot_ReaderErrorDegradesToUnregistered(t *testing.T) {
	// Per-delegate reader failure (RPC hiccup, EVM revert, ABI mismatch) must
	// degrade to Registered=false, not abort PutPollResult. Otherwise a single
	// bad delegate deterministically halts the chain at every epoch boundary.
	r := require.New(t)
	b, err := New(mainnetContract)
	r.NoError(err)

	delegate := identityset.Address(7)
	failReader := ContractReaderFunc(func(context.Context, string, []byte) ([]byte, error) {
		return nil, errors.New("rpc down")
	})

	got, err := b.Snapshot(context.Background(), failReader, []address.Address{delegate})
	r.NoError(err)
	rates := got[delegate.String()]
	r.NotNil(rates)
	r.False(rates.Registered)
}

func TestSnapshot_PerDelegateErrorIsolated(t *testing.T) {
	// A failing delegate must not poison the successful ones sharing the same
	// batch. Freezing the whole poll list on one bad profile is the failure
	// mode this test defends against.
	r := require.New(t)
	b, err := New(mainnetContract)
	r.NoError(err)

	good := identityset.Address(8)
	bad := identityset.Address(9)
	store := newFakeStore()
	store.set(good, fieldBlockRewardPortion, 9000) // → 1000 bp commission
	store.set(good, fieldEpochRewardPortion, 8000) // → 2000 bp commission
	store.set(bad, fieldBlockRewardPortion, 12345) // out of range
	store.set(bad, fieldEpochRewardPortion, 5000)

	got, err := b.Snapshot(context.Background(), store.reader(t), []address.Address{good, bad})
	r.NoError(err)
	r.Len(got, 2)

	gr := got[good.String()]
	r.NotNil(gr)
	r.True(gr.Registered)
	r.Equal(uint64(1000), gr.BlockCommissionBasisPoints)
	r.Equal(uint64(2000), gr.EpochCommissionBasisPoints)

	br := got[bad.String()]
	r.NotNil(br)
	r.False(br.Registered)
}

func TestSnapshot_NilReaderRejected(t *testing.T) {
	r := require.New(t)
	b, err := New(mainnetContract)
	r.NoError(err)

	_, err = b.Snapshot(context.Background(), nil, []address.Address{identityset.Address(1)})
	r.Error(err)
}

func TestSnapshot_NilDelegateRejected(t *testing.T) {
	// A nil entry in the delegate slice is a caller bug that must fail loudly
	// — silently skipping would let a delegate slip past PutPollResult
	// without a commission snapshot.
	r := require.New(t)
	b, err := New(mainnetContract)
	r.NoError(err)

	store := newFakeStore()
	_, err = b.Snapshot(context.Background(), store.reader(t), []address.Address{nil})
	r.Error(err)
}

func TestSnapshot_PreservesIterationOrder(t *testing.T) {
	// The map's key set must contain every delegate exactly once, and the
	// underlying 2N reads happen in delegate-order. This isn't asserted with
	// a slice — the return type is a map — but exercising a multi-delegate
	// case at least catches accidental early returns.
	r := require.New(t)
	b, err := New(mainnetContract)
	r.NoError(err)

	delegates := []address.Address{
		identityset.Address(1),
		identityset.Address(2),
		identityset.Address(3),
	}
	store := newFakeStore()
	for i, d := range delegates {
		store.set(d, fieldBlockRewardPortion, uint64(1000*(i+1)))
		store.set(d, fieldEpochRewardPortion, uint64(500*(i+1)))
	}

	out, err := b.Snapshot(context.Background(), store.reader(t), delegates)
	r.NoError(err)
	r.Len(out, len(delegates))
	for i, d := range delegates {
		rates := out[d.String()]
		r.NotNil(rates, "delegate %d missing", i)
		r.True(rates.Registered)
		r.Equal(maxBasisPoints-uint64(1000*(i+1)), rates.BlockCommissionBasisPoints)
		r.Equal(maxBasisPoints-uint64(500*(i+1)), rates.EpochCommissionBasisPoints)
	}
}
