// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package autodeposit

import (
	"context"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// mainnetContract is the pinned mainnet AutoDeposit deployment. Tests use
// it verbatim so a rename in production would be caught here too.
const mainnetContract = "io108ckwzlzpkhva7cnfceajlu7wu6ql5kq95uat9"

// fakeStore is an in-memory stand-in for the AutoDeposit contract's
// buckets(address) storage. Each entry maps a voter to the int256 the
// contract would return from bucket(voter). Missing entries return zero,
// matching Solidity's default-mapping behaviour.
type fakeStore struct {
	values map[string]*big.Int
}

func newFakeStore() *fakeStore { return &fakeStore{values: map[string]*big.Int{}} }

func (s *fakeStore) set(voter address.Address, bucketID *big.Int) {
	s.values[storeKey(voter)] = new(big.Int).Set(bucketID)
}

func storeKey(voter address.Address) string {
	return common.BytesToAddress(voter.Bytes()).Hex()
}

// reader turns fakeStore into a ContractReader by decoding the incoming
// bucket(address) call, looking up the value, and packing it the same way
// go-ethereum's abi package does.
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
		require.Equal(t, fieldBucketFn, method.Name)
		require.Len(t, args, 1)
		voterEth := args[0].(common.Address)
		addr, err := address.FromBytes(voterEth.Bytes())
		if err != nil {
			return nil, err
		}
		value, ok := s.values[storeKey(addr)]
		if !ok {
			value = big.NewInt(0)
		}
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

func TestLookupBucket_Registered(t *testing.T) {
	r := require.New(t)
	b, err := New(mainnetContract)
	r.NoError(err)

	voter := identityset.Address(1)
	store := newFakeStore()
	store.set(voter, big.NewInt(42))

	bucketID, present, err := b.LookupBucket(context.Background(), store.reader(t), voter)
	r.NoError(err)
	r.True(present)
	r.Equal(uint64(42), bucketID)
}

func TestLookupBucket_Unregistered(t *testing.T) {
	r := require.New(t)
	b, err := New(mainnetContract)
	r.NoError(err)

	voter := identityset.Address(2)
	store := newFakeStore() // voter never registered

	bucketID, present, err := b.LookupBucket(context.Background(), store.reader(t), voter)
	r.NoError(err)
	r.False(present)
	r.Zero(bucketID)
}

func TestLookupBucket_ExplicitZero(t *testing.T) {
	// A voter who called register(0) is indistinguishable from an
	// unregistered voter — the spec's "non-zero" precondition intentionally
	// treats bucket ID 0 as ineligible, matching Hermes' historical
	// interpretation.
	r := require.New(t)
	b, err := New(mainnetContract)
	r.NoError(err)

	voter := identityset.Address(3)
	store := newFakeStore()
	store.set(voter, big.NewInt(0))

	bucketID, present, err := b.LookupBucket(context.Background(), store.reader(t), voter)
	r.NoError(err)
	r.False(present, "bucket ID 0 must route to credit")
	r.Zero(bucketID)
}

func TestLookupBucket_NegativeInt256(t *testing.T) {
	// int256 allows negative sentinels; malformed on-chain data must
	// degrade to unregistered rather than error out (per
	// feedback-consensus-fallback-vs-halt).
	r := require.New(t)
	b, err := New(mainnetContract)
	r.NoError(err)

	voter := identityset.Address(4)
	store := newFakeStore()
	store.set(voter, big.NewInt(-1))

	bucketID, present, err := b.LookupBucket(context.Background(), store.reader(t), voter)
	r.NoError(err)
	r.False(present, "negative bucket ID must degrade to credit, not error")
	r.Zero(bucketID)
}

func TestLookupBucket_ValueTooLargeForUint64(t *testing.T) {
	// A bucket ID that doesn't fit uint64 is malformed on-chain data. Same
	// degradation rule: silent fallback to credit, no error.
	r := require.New(t)
	b, err := New(mainnetContract)
	r.NoError(err)

	voter := identityset.Address(5)
	store := newFakeStore()
	huge := new(big.Int).Lsh(big.NewInt(1), 70) // 2^70
	store.set(voter, huge)

	bucketID, present, err := b.LookupBucket(context.Background(), store.reader(t), voter)
	r.NoError(err)
	r.False(present, "oversized bucket ID must degrade to credit")
	r.Zero(bucketID)
}

func TestLookupBucket_ReaderErrorPropagates(t *testing.T) {
	r := require.New(t)
	b, err := New(mainnetContract)
	r.NoError(err)

	voter := identityset.Address(6)
	failReader := ContractReaderFunc(func(context.Context, string, []byte) ([]byte, error) {
		return nil, errors.New("rpc down")
	})

	_, present, err := b.LookupBucket(context.Background(), failReader, voter)
	r.Error(err)
	r.False(present)
	r.Contains(err.Error(), "rpc down")
	r.Contains(err.Error(), voter.String(), "error must name the offending voter")
}

func TestLookupBucket_NilReaderRejected(t *testing.T) {
	r := require.New(t)
	b, err := New(mainnetContract)
	r.NoError(err)

	_, _, err = b.LookupBucket(context.Background(), nil, identityset.Address(1))
	r.Error(err)
}

func TestLookupBucket_NilVoterRejected(t *testing.T) {
	// A nil voter address is a caller bug — silently skipping would let a
	// voter slip past the drain without a routing decision. Must fail loud.
	r := require.New(t)
	b, err := New(mainnetContract)
	r.NoError(err)

	store := newFakeStore()
	_, _, err = b.LookupBucket(context.Background(), store.reader(t), nil)
	r.Error(err)
}

func TestLookupBucket_CallDataShape(t *testing.T) {
	// Guard against ABI drift: the encoded call data's first four bytes
	// must equal keccak256("bucket(address)")[:4], and the voter address
	// must occupy the trailing 20 bytes of the following 32-byte word.
	r := require.New(t)
	b, err := New(mainnetContract)
	r.NoError(err)

	voter := identityset.Address(1)
	captured := make([][]byte, 0, 1)
	capReader := ContractReaderFunc(func(_ context.Context, _ string, data []byte) ([]byte, error) {
		captured = append(captured, append([]byte(nil), data...))
		// Return a valid packed uint256(0) so LookupBucket completes.
		parsed, _ := abi.JSON(strings.NewReader(abiJSON))
		return parsed.Methods[fieldBucketFn].Outputs.Pack(big.NewInt(0))
	})

	_, _, err = b.LookupBucket(context.Background(), capReader, voter)
	r.NoError(err)
	r.Len(captured, 1)

	callData := captured[0]
	r.GreaterOrEqual(len(callData), 4+32)
	expectedSelector := crypto.Keccak256([]byte("bucket(address)"))[:4]
	r.Equal(expectedSelector, callData[:4], "selector must match bucket(address)")

	// Word after selector: 12 zero bytes + 20-byte address.
	word := callData[4 : 4+32]
	r.Equal(make([]byte, 12), word[:12], "address word must be left-padded with zeroes")
	r.Equal(voter.Bytes(), word[12:], "trailing 20 bytes must be the voter address")
}
