// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"crypto/sha256"
	"math"
	"testing"
	"time"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/mohae/deepcopy"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// blsKeyFromSeed generates a deterministic BLS keypair for tests.
func blsKeyFromSeed(t *testing.T, seed string) *crypto.BLS12381PrivateKey {
	t.Helper()
	h := sha256.Sum256([]byte(seed))
	sk, err := crypto.GenerateBLS12381PrivateKey(h[:])
	require.NoError(t, err)
	return sk
}

// genesisWithPoPGate returns a TestDefault genesis tuned for the PoP
// gate tests: XinguBlockHeight is forced to 0 so the BLS-register
// codepath is reachable (CandidateBLSPublicKey feature requires it),
// and ToBeEnabledBlockHeight controls whether EnforceBLSPoP is on.
func genesisWithPoPGate(gate bool) genesis.Genesis {
	g := deepcopy.Copy(genesis.TestDefault()).(genesis.Genesis)
	g.TsunamiBlockHeight = 0
	g.XinguBlockHeight = 0
	if gate {
		g.ToBeEnabledBlockHeight = 0
	} else {
		g.ToBeEnabledBlockHeight = math.MaxUint64
	}
	return g
}

// buildHandlerCtx wires the same context the handler expects, with the
// genesis configured for the PoP gate the caller wants.
func buildHandlerCtx(caller address.Address, gate bool, nonce uint64) context.Context {
	ctx := protocol.WithActionCtx(context.Background(), protocol.ActionCtx{
		Caller:       caller,
		GasPrice:     testGasPrice,
		IntrinsicGas: 10000,
		Nonce:        nonce,
	})
	ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
		BlockHeight:    1,
		BlockTimeStamp: time.Now(),
		GasLimit:       1000000,
	})
	ctx = protocol.WithBlockchainCtx(ctx, protocol.BlockchainCtx{Tip: protocol.TipInfo{}})
	ctx = genesis.WithGenesisContext(ctx, genesisWithPoPGate(gate))
	ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
	return ctx
}

// runRegisterWithBLS runs handleCandidateRegister (via Protocol.Handle)
// for a candidate registration that carries the supplied BLS material.
// Returns the receipt so the caller can assert on Status.
func runRegisterWithBLS(t *testing.T, p *Protocol, sm protocol.StateManager,
	caller, owner address.Address, nonce uint64, name string,
	blsPubKey, blsPop []byte, gate bool) *action.Receipt {
	t.Helper()
	require := require.New(t)
	require.NoError(setupAccount(sm, caller, 100_000_000))

	cr, err := action.NewCandidateRegisterWithBLS(
		name,
		identityset.Address(28).String(),
		identityset.Address(29).String(),
		owner.String(),
		"1200000000000000000000000",
		uint32(10000),
		false,
		blsPubKey, blsPop, nil,
	)
	require.NoError(err)
	elp := builder.SetNonce(nonce).SetGasLimit(1_000_000).
		SetGasPrice(testGasPrice).SetAction(cr).Build()

	ctx := buildHandlerCtx(caller, gate, nonce)
	require.NoError(p.Validate(ctx, elp, sm))
	r, err := p.Handle(ctx, elp, sm)
	require.NoError(err)
	return r
}

// TestHandleCandidateRegister_PoPGate covers the post-fork PoP
// enforcement at the register handler. The unit tests for SignBLSPop /
// VerifyBLSPop establish the cryptographic property; this test wires
// it through the handler and confirms the gate semantics:
//
//   - gate ON + valid PoP        → ReceiptStatus_Success
//   - gate ON + empty PoP        → ReceiptStatus_ErrUnauthorizedOperator
//   - gate ON + invalid PoP      → ReceiptStatus_ErrUnauthorizedOperator
//   - gate OFF + empty PoP       → ReceiptStatus_Success (pre-fork
//     behaviour preserved — BLS registration works without PoP)
func TestHandleCandidateRegister_PoPGate(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	owner := identityset.Address(30)
	caller := identityset.Address(27) // initAll uses 27 as the caller fixture

	sk := blsKeyFromSeed(t, "register-gate")
	pk := sk.PublicKey().Bytes()

	t.Run("gate on, valid PoP → Success", func(t *testing.T) {
		sm, p, _, _ := initAll(t, ctrl)
		pop, err := SignBLSPop(sk, owner)
		require.NoError(err)
		r := runRegisterWithBLS(t, p, sm, caller, owner, 1, "popok", pk, pop, true)
		require.NotNil(r)
		require.EqualValues(iotextypes.ReceiptStatus_Success, r.Status,
			"handler should accept a valid PoP under EnforceBLSPoP")

		// confirm the BLS pubkey actually landed in state
		csm, err := NewCandidateStateManager(sm)
		require.NoError(err)
		cand := csm.GetByOwner(owner)
		require.NotNil(cand)
		require.Equal(pk, cand.BLSPubKey)
	})

	t.Run("gate on, empty PoP → ErrUnauthorizedOperator", func(t *testing.T) {
		sm, p, _, _ := initAll(t, ctrl)
		r := runRegisterWithBLS(t, p, sm, caller, owner, 1, "nopop", pk, nil, true)
		require.NotNil(r)
		require.EqualValues(iotextypes.ReceiptStatus_ErrUnauthorizedOperator, r.Status,
			"handler must reject register with empty PoP once the gate is on")
	})

	t.Run("gate on, invalid PoP → ErrUnauthorizedOperator", func(t *testing.T) {
		sm, p, _, _ := initAll(t, ctrl)
		// PoP shape-correct but signed by a different key — verifier
		// rejects because Verify(pk, ...) doesn't accept a sig produced
		// under sk_attacker.
		attackerSK := blsKeyFromSeed(t, "attacker")
		forged, err := attackerSK.Sign(SignBLSPopMustRoot(t, owner))
		require.NoError(err)
		r := runRegisterWithBLS(t, p, sm, caller, owner, 1, "badpop", pk, forged, true)
		require.NotNil(r)
		require.EqualValues(iotextypes.ReceiptStatus_ErrUnauthorizedOperator, r.Status,
			"handler must reject register with PoP that doesn't verify under blsPubKey")
	})

	t.Run("gate off, empty PoP → Success (pre-fork compat)", func(t *testing.T) {
		sm, p, _, _ := initAll(t, ctrl)
		r := runRegisterWithBLS(t, p, sm, caller, owner, 1, "preforkok", pk, nil, false)
		require.NotNil(r)
		require.EqualValues(iotextypes.ReceiptStatus_Success, r.Status,
			"gate off → PoP is optional, pre-fork behaviour preserved")
	})
}

// TestHandleCandidateRegister_BLSPubKeyUniqueness locks in the second
// post-fork invariant: two delegates cannot share a BLS pubkey, since
// IIP-52 FastAggregateVerify dedupes the pubkey set but the signer
// bitmap doesn't. Without the GetByBLSPubKey check the second
// registration would be silently accepted, breaking quorum counting.
func TestHandleCandidateRegister_BLSPubKeyUniqueness(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	sm, p, _, _ := initAll(t, ctrl)

	sk := blsKeyFromSeed(t, "shared")
	pk := sk.PublicKey().Bytes()

	// First registration — owner A — succeeds.
	ownerA := identityset.Address(30)
	popA, err := SignBLSPop(sk, ownerA)
	require.NoError(err)
	r := runRegisterWithBLS(t, p, sm, identityset.Address(27), ownerA, 1, "canda", pk, popA, true)
	require.NotNil(r)
	require.EqualValues(iotextypes.ReceiptStatus_Success, r.Status)

	// Second registration — owner B, same pubkey, valid PoP signed
	// under ownerB so PoP itself passes — must be rejected by the
	// uniqueness check.
	ownerB := identityset.Address(31)
	popB, err := SignBLSPop(sk, ownerB)
	require.NoError(err)
	r = runRegisterWithBLS(t, p, sm, identityset.Address(28), ownerB, 1, "candb", pk, popB, true)
	require.NotNil(r)
	require.EqualValues(iotextypes.ReceiptStatus_ErrCandidateConflict, r.Status,
		"a second candidate must not be allowed to register the same BLS pubkey")
}

// SignBLSPopMustRoot builds the signing root for a candidate without
// invoking VerifyBLSPop — used by the invalid-PoP case where we want
// to sign with the wrong key.
func SignBLSPopMustRoot(t *testing.T, candidateID address.Address) []byte {
	t.Helper()
	root := BLSPopSigningRoot(candidateID)
	require.NotNil(t, root)
	return root
}

// TestHandleCandidateUpdate_PoPGate covers the update path's PoP gate.
// Tested through handleCandidateUpdate (the owner-as-caller branch).
// The Operator-as-caller branch is structurally identical and shares
// the same VerifyBLSPop + GetByBLSPubKey calls — covered by Unit and
// by the rogue-key regression in bls_pop_test.go.
func TestHandleCandidateUpdate_PoPGate(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	owner := identityset.Address(30)
	caller := identityset.Address(27)

	// Set up state with an existing candidate owned by `owner`.
	setupCand := func() (protocol.StateManager, *Protocol) {
		sm, p, _, _ := initAll(t, ctrl)
		sk0 := blsKeyFromSeed(t, "original")
		pk0 := sk0.PublicKey().Bytes()
		pop0, err := SignBLSPop(sk0, owner)
		require.NoError(err)
		r := runRegisterWithBLS(t, p, sm, caller, owner, 1, "mycand", pk0, pop0, true)
		require.EqualValues(iotextypes.ReceiptStatus_Success, r.Status)
		return sm, p
	}

	runUpdate := func(sm protocol.StateManager, p *Protocol, callerAddr address.Address,
		newPK, newPoP []byte, nonce uint64, gate bool) *action.Receipt {
		require.NoError(setupAccount(sm, callerAddr, 100_000_000))
		cu, err := action.NewCandidateUpdateWithBLS(
			"mycand",
			identityset.Address(28).String(),
			identityset.Address(29).String(),
			newPK, newPoP,
		)
		require.NoError(err)
		elp := builder.SetNonce(nonce).SetGasLimit(1_000_000).
			SetGasPrice(testGasPrice).SetAction(cu).Build()
		ctx := buildHandlerCtx(callerAddr, gate, nonce)
		require.NoError(p.Validate(ctx, elp, sm))
		r, err := p.Handle(ctx, elp, sm)
		require.NoError(err)
		return r
	}

	t.Run("gate on, valid PoP → rotation succeeds, new BLSPubKey in state", func(t *testing.T) {
		sm, p := setupCand()
		csm0, err := NewCandidateStateManager(sm)
		require.NoError(err)
		c0 := csm0.GetByOwner(owner)
		require.NotNil(c0)

		newSK := blsKeyFromSeed(t, "rotation-target")
		newPK := newSK.PublicKey().Bytes()
		// PoP is signed under the candidate's IDENTIFIER (not the
		// current owner) — the property locked in by
		// TestBLSPop_StableAcrossOwnershipTransfer in bls_pop_test.go.
		newPoP, err := SignBLSPop(newSK, c0.GetIdentifier())
		require.NoError(err)

		r := runUpdate(sm, p, owner, newPK, newPoP, 1, true)
		require.NotNil(r)
		require.EqualValues(iotextypes.ReceiptStatus_Success, r.Status,
			"update with valid PoP under c.GetIdentifier() must succeed")

		csm, err := NewCandidateStateManager(sm)
		require.NoError(err)
		c := csm.GetByOwner(owner)
		require.NotNil(c)
		require.Equal(newPK, c.BLSPubKey, "BLSPubKey must be rotated in state")
	})

	t.Run("gate on, empty PoP → ErrUnauthorizedOperator", func(t *testing.T) {
		sm, p := setupCand()
		newSK := blsKeyFromSeed(t, "rotation-target-nopop")
		newPK := newSK.PublicKey().Bytes()
		r := runUpdate(sm, p, owner, newPK, nil, 1, true)
		require.NotNil(r)
		require.EqualValues(iotextypes.ReceiptStatus_ErrUnauthorizedOperator, r.Status,
			"empty PoP on rotation must be rejected under EnforceBLSPoP")
	})

	t.Run("gate on, PoP bound to wrong candidateID → reject", func(t *testing.T) {
		sm, p := setupCand()
		newSK := blsKeyFromSeed(t, "rotation-target-wrong-id")
		newPK := newSK.PublicKey().Bytes()
		// PoP signed under the WRONG candidateID — the cross-candidate
		// replay defence at the binding layer.
		wrongID := identityset.Address(33)
		wrongPoP, err := SignBLSPop(newSK, wrongID)
		require.NoError(err)
		r := runUpdate(sm, p, owner, newPK, wrongPoP, 1, true)
		require.NotNil(r)
		require.EqualValues(iotextypes.ReceiptStatus_ErrUnauthorizedOperator, r.Status,
			"PoP signed under the wrong candidateID must be rejected")
	})

	t.Run("gate off, empty PoP → rotation succeeds (pre-fork compat)", func(t *testing.T) {
		// For the gate-off variant we register without PoP, then
		// rotate without PoP, both under gate-off — confirming the
		// existing behaviour stays intact for pre-fork blocks.
		sm, p, _, _ := initAll(t, ctrl)
		require.NoError(setupAccount(sm, caller, 100_000_000))
		sk0 := blsKeyFromSeed(t, "prefork-original")
		pk0 := sk0.PublicKey().Bytes()
		r := runRegisterWithBLS(t, p, sm, caller, owner, 1, "mycand", pk0, nil, false)
		require.EqualValues(iotextypes.ReceiptStatus_Success, r.Status)

		newSK := blsKeyFromSeed(t, "prefork-rotation")
		newPK := newSK.PublicKey().Bytes()
		r = runUpdate(sm, p, owner, newPK, nil, 1, false)
		require.NotNil(r)
		require.EqualValues(iotextypes.ReceiptStatus_Success, r.Status,
			"gate off → update without PoP must continue to work")
	})
}
