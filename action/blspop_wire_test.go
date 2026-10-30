// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// The staking protocol keeps new and old nodes in agreement before the PoP
// fork with a single rule: reject any action carrying a non-empty blsPop. That
// rule is only sufficient because of two properties of this package, which the
// tests below pin down.
//
//   1. Decoding either V2 selector implies a non-empty blsPop, so "reject
//      non-empty blsPop" and "reject the V2 selectors" are the same rule. If a
//      V2 call could decode with an empty PoP it would slip past validation on
//      a new node and still fail Unfold on an old one.
//
//   2. An action with no PoP serialises exactly as it did before the field
//      existed, so it hashes identically on both nodes and its signature
//      verifies on both. If Proto() emitted an empty blsPop, every legacy
//      action would hash differently across the upgrade.

func popTestPubKey(t *testing.T) []byte {
	t.Helper()
	// A valid, canonical G1 point: the decoders run BLS12381PublicKeyFromBytes
	// before they ever look at blsPop, so a dummy byte string would fail for
	// the wrong reason.
	sk, err := crypto.GenerateBLS12381PrivateKey(identityset.PrivateKey(0).Bytes())
	require.NoError(t, err)
	return sk.PublicKey().Bytes()
}

func TestV2SelectorsRequireNonEmptyPoP(t *testing.T) {
	r := require.New(t)
	pk := popTestPubKey(t)
	op := common.BytesToAddress(identityset.Address(2).Bytes())
	rew := common.BytesToAddress(identityset.Address(3).Bytes())
	own := common.BytesToAddress(identityset.Address(1).Bytes())

	t.Run("candidateRegisterWithBLSAndPoP", func(t *testing.T) {
		for _, pop := range [][]byte{nil, {}} {
			data, err := _candidateRegisterWithBLSAndPoPMethod.Inputs.Pack(
				"cand", op, rew, own, uint32(1), true, pk, pop, []byte(nil))
			r.NoError(err)
			calldata := append(append([]byte{}, _candidateRegisterWithBLSAndPoPMethod.ID...), data...)

			_, err = NewCandidateRegisterFromABIBinary(calldata, big.NewInt(0))
			r.ErrorIs(err, errDecodeFailure)
			r.ErrorContains(err, "blsPop is empty")
		}
	})

	t.Run("candidateUpdateWithBLSAndPoP", func(t *testing.T) {
		for _, pop := range [][]byte{nil, {}} {
			data, err := _candidateUpdateWithBLSAndPoPMethod.Inputs.Pack("cand", op, rew, pk, pop)
			r.NoError(err)
			calldata := append(append([]byte{}, _candidateUpdateWithBLSAndPoPMethod.ID...), data...)

			_, err = NewCandidateUpdateFromABIBinary(calldata)
			r.ErrorIs(err, errDecodeFailure)
			r.ErrorContains(err, "blsPop is empty")
		}
	})
}

func TestEmptyPoPLeavesTheWireFormatUnchanged(t *testing.T) {
	r := require.New(t)
	pk := popTestPubKey(t)
	owner := identityset.Address(1).String()
	op := identityset.Address(2).String()
	rew := identityset.Address(3).String()
	amount := "1200000000000000000000000"
	pop := bytes.Repeat([]byte{0x7}, 96)

	t.Run("register", func(t *testing.T) {
		noPoP, err := NewCandidateRegisterWithBLS("cand", op, rew, owner, amount, 1, true, pk, nil, nil)
		r.NoError(err)
		emptyPoP, err := NewCandidateRegisterWithBLS("cand", op, rew, owner, amount, 1, true, pk, []byte{}, nil)
		r.NoError(err)
		withPoP, err := NewCandidateRegisterWithBLS("cand", op, rew, owner, amount, 1, true, pk, pop, nil)
		r.NoError(err)

		r.Empty(noPoP.Proto().GetCandidate().GetBlsPop(),
			"an absent PoP must not put the field on the wire")
		r.Equal(noPoP.Serialize(), emptyPoP.Serialize(),
			"nil and empty PoP must serialise identically")
		r.NotEqual(noPoP.Serialize(), withPoP.Serialize(),
			"a PoP must be part of the hash preimage, or it is not committed to")
	})

	t.Run("update", func(t *testing.T) {
		noPoP, err := NewCandidateUpdateWithBLS("cand", op, rew, pk, nil)
		r.NoError(err)
		emptyPoP, err := NewCandidateUpdateWithBLS("cand", op, rew, pk, []byte{})
		r.NoError(err)
		withPoP, err := NewCandidateUpdateWithBLS("cand", op, rew, pk, pop)
		r.NoError(err)

		r.Empty(noPoP.Proto().GetBlsPop())
		r.Equal(noPoP.Serialize(), emptyPoP.Serialize())
		r.NotEqual(noPoP.Serialize(), withPoP.Serialize())
	})

	// A wire message carrying a PoP with no key must not survive the round
	// trip. LoadProto already nests the PoP under the key; Proto() has to do
	// the same, or an action decoded from such a message would re-serialise
	// into something different from what it was decoded from -- and, since
	// envelopeHash marshals the rebuilt message, hash to a different value on
	// a node that keeps the field than on one that never had it.
	t.Run("orphan PoP does not survive the wire round trip", func(t *testing.T) {
		orphan := &iotextypes.CandidateBasicInfo{
			Name:            "cand",
			OperatorAddress: op,
			RewardAddress:   rew,
			BlsPop:          pop,
		}
		var cu CandidateUpdate
		r.NoError(cu.LoadProto(orphan))
		r.Empty(cu.BLSPop(), "LoadProto must drop a PoP with no key")
		r.Empty(cu.Proto().GetBlsPop(), "Proto must not put it back")

		clean := &iotextypes.CandidateBasicInfo{
			Name:            "cand",
			OperatorAddress: op,
			RewardAddress:   rew,
		}
		var ref CandidateUpdate
		r.NoError(ref.LoadProto(clean))
		r.Equal(ref.Serialize(), cu.Serialize(),
			"an orphan PoP must hash the same as no PoP at all")
	})
}
