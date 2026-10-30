// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package autodeposit

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// fakeSlotReader stands in for evm.SlotReader in unit tests. Callers seed
// (voter, slotNum) → int256 tuples via setBucket / setRegistered; the
// GetSlot implementation computes the mapping key exactly as
// SlotBucketReader does and returns the seeded 32-byte value (zero if
// unset, matching Solidity's default-mapping behaviour).
type fakeSlotReader struct {
	slots map[[32]byte][32]byte
}

func newFakeSlotReader() *fakeSlotReader {
	return &fakeSlotReader{slots: map[[32]byte][32]byte{}}
}

func (f *fakeSlotReader) GetSlot(_ address.Address, key []byte) []byte {
	var k [32]byte
	copy(k[:], key)
	v := f.slots[k]
	out := make([]byte, 32)
	copy(out, v[:])
	return out
}

// setBucket seeds buckets[voter] with a signed int256 value. Values are
// stored big-endian and, for negatives, use two's-complement matching
// Solidity's on-chain encoding.
func (f *fakeSlotReader) setBucket(voter address.Address, value *big.Int) {
	f.setMappingSlot(voter, slotBuckets, value)
}

func (f *fakeSlotReader) setRegistered(voter address.Address, registered bool) {
	v := big.NewInt(0)
	if registered {
		v = big.NewInt(1)
	}
	f.setMappingSlot(voter, slotRegistrants, v)
}

func (f *fakeSlotReader) setMappingSlot(voter address.Address, slot uint8, value *big.Int) {
	voterEth := common.BytesToAddress(voter.Bytes())
	key := mappingSlotKey(voterEth, slot)
	var k [32]byte
	copy(k[:], key)

	// Two's-complement 256-bit encoding: for negatives, (2^256 + value)
	// mod 2^256 gives the on-chain byte layout Solidity uses.
	enc := new(big.Int).Set(value)
	if enc.Sign() < 0 {
		mod := new(big.Int).Lsh(big.NewInt(1), 256)
		enc.Add(mod, enc)
	}
	b := enc.Bytes()
	var v [32]byte
	copy(v[32-len(b):], b)
	f.slots[k] = v
}

func newSlotBucketReader(t *testing.T, r SlotReader) *SlotBucketReader {
	t.Helper()
	bridge, err := New(mainnetContract)
	require.NoError(t, err)
	br, err := bridge.NewSlotBucketReader(r)
	require.NoError(t, err)
	return br
}

func TestSlotBucketReader_Registered(t *testing.T) {
	r := require.New(t)

	voter := identityset.Address(1)
	store := newFakeSlotReader()
	store.setRegistered(voter, true)
	store.setBucket(voter, big.NewInt(42))

	br := newSlotBucketReader(t, store)
	bucketID, present, err := br.LookupBucket(voter)
	r.NoError(err)
	r.True(present)
	r.Equal(uint64(42), bucketID)
}

func TestSlotBucketReader_Unregistered(t *testing.T) {
	r := require.New(t)

	voter := identityset.Address(2)
	store := newFakeSlotReader() // registrants[voter] = 0

	br := newSlotBucketReader(t, store)
	bucketID, present, err := br.LookupBucket(voter)
	r.NoError(err)
	r.False(present)
	r.Zero(bucketID)
}

func TestSlotBucketReader_RegisteredButZeroBucket(t *testing.T) {
	// A voter marked registered with bucketId 0 is malformed on-chain state
	// (register() writes only positive IDs in practice). Fall back to credit
	// silently rather than halt the block — feedback-consensus-fallback-vs-halt.
	r := require.New(t)

	voter := identityset.Address(3)
	store := newFakeSlotReader()
	store.setRegistered(voter, true)
	store.setBucket(voter, big.NewInt(0))

	br := newSlotBucketReader(t, store)
	bucketID, present, err := br.LookupBucket(voter)
	r.NoError(err)
	r.False(present, "bucket ID 0 must route to credit")
	r.Zero(bucketID)
}

func TestSlotBucketReader_NegativeBucket(t *testing.T) {
	// Solidity int256 negative → high bit set → SetBytes reads as huge
	// positive that fails IsUint64. Silent fallback per contract.
	r := require.New(t)

	voter := identityset.Address(4)
	store := newFakeSlotReader()
	store.setRegistered(voter, true)
	store.setBucket(voter, big.NewInt(-1))

	br := newSlotBucketReader(t, store)
	bucketID, present, err := br.LookupBucket(voter)
	r.NoError(err)
	r.False(present, "negative bucket ID must degrade to credit, not error")
	r.Zero(bucketID)
}

func TestSlotBucketReader_ValueTooLargeForUint64(t *testing.T) {
	// A bucket ID that doesn't fit uint64 is malformed on-chain data. Same
	// degradation rule: silent fallback to credit, no error.
	r := require.New(t)

	voter := identityset.Address(5)
	store := newFakeSlotReader()
	store.setRegistered(voter, true)
	huge := new(big.Int).Lsh(big.NewInt(1), 70) // 2^70
	store.setBucket(voter, huge)

	br := newSlotBucketReader(t, store)
	bucketID, present, err := br.LookupBucket(voter)
	r.NoError(err)
	r.False(present, "oversized bucket ID must degrade to credit")
	r.Zero(bucketID)
}

func TestSlotBucketReader_NilVoterRejected(t *testing.T) {
	// A nil voter address is a caller bug — silently skipping would let a
	// voter slip past the drain without a routing decision. Must fail loud.
	r := require.New(t)

	br := newSlotBucketReader(t, newFakeSlotReader())
	_, _, err := br.LookupBucket(nil)
	r.Error(err)
}

func TestNewSlotBucketReader_NilReaderRejected(t *testing.T) {
	r := require.New(t)

	bridge, err := New(mainnetContract)
	r.NoError(err)
	_, err = bridge.NewSlotBucketReader(nil)
	r.Error(err)
}

func TestMappingSlotKey_MatchesSolidityFormula(t *testing.T) {
	// Guard against key-formula drift: the storage key for mapping(address =>
	// T) entry k must equal keccak256(pad32(k) || pad32(slotNumber)).
	r := require.New(t)

	voter := identityset.Address(1)
	voterEth := common.BytesToAddress(voter.Bytes())

	buf := make([]byte, 64)
	copy(buf[12:32], voterEth.Bytes())
	buf[63] = slotBuckets
	expected := crypto.Keccak256(buf)

	r.Equal(expected, mappingSlotKey(voterEth, slotBuckets))
}
