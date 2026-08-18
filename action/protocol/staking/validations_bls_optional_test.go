// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// ctxAtEra builds a validation context whose FeatureCtx corresponds to one of
// the three BLS eras: pre-Xingu (no BLS at all), Xingu (BLS mandatory) and
// Zanzibar (BLS optional but paired with its PoP).
func ctxAtEra(t *testing.T, era string) context.Context {
	t.Helper()
	g := genesis.TestDefault()
	switch era {
	case "pre-xingu":
		g.XinguBlockHeight = 100
		g.XinguBetaBlockHeight = 100
		g.YapBlockHeight = 100
		g.YapBetaBlockHeight = 100
		g.ZanzibarBlockHeight = 100
	case "xingu":
		g.XinguBlockHeight = 0
		g.ZanzibarBlockHeight = 100
	case "zanzibar":
		g.XinguBlockHeight = 0
		g.ZanzibarBlockHeight = 0
	default:
		t.Fatalf("unknown era %s", era)
	}
	ctx := genesis.WithGenesisContext(context.Background(), g)
	return protocol.WithFeatureCtx(protocol.WithBlockCtx(ctx, protocol.BlockCtx{BlockHeight: 1}))
}

func mustRegister(t *testing.T, withBLS bool, pop []byte) *action.CandidateRegister {
	t.Helper()
	owner := identityset.Address(1).String()
	op := identityset.Address(2).String()
	rew := identityset.Address(3).String()
	amount := "1200000000000000000000000"
	if !withBLS {
		// The legacy candidateRegister entry: amount travels in the legacy
		// field, no BLS fields at all.
		cr, err := action.NewCandidateRegister("cand", op, rew, owner, amount, 1, true, nil)
		require.NoError(t, err)
		return cr
	}
	pk := blsKeyFromSeed(t, "pair").PublicKey().Bytes()
	cr, err := action.NewCandidateRegisterWithBLS("cand", op, rew, owner, amount, 1, true, pk, pop, nil)
	require.NoError(t, err)
	return cr
}

// TestValidateBLSPairing covers both halves of the pairing rule directly. The
// PoP-without-key direction is unreachable through the ABI -- every
// candidateRegisterWithBLS* method rejects an empty blsPubKey at decode time --
// so it is exercised here rather than through a constructed action.
func TestValidateBLSPairing(t *testing.T) {
	r := require.New(t)
	pop := []byte("pop")
	r.NoError(validateBLSPairing(false, nil))
	r.NoError(validateBLSPairing(true, pop))
	// A key with no PoP passes validation and is rejected by the handler's
	// EnforceBLSPoP check instead -- see TestHandleCandidateRegister_PoPGate.
	r.NoError(validateBLSPairing(true, nil))
	r.ErrorContains(validateBLSPairing(false, pop), "must be accompanied by a public key")
}

func TestValidateCandidateRegisterBLSEras(t *testing.T) {
	r := require.New(t)
	p := &Protocol{}
	p.config.RegistrationConsts.MinSelfStake = big.NewInt(0)
	pop := []byte("some-pop-bytes-not-verified-here-------------------------------------------------------------")

	t.Run("xingu: BLS mandatory", func(t *testing.T) {
		ctx := ctxAtEra(t, "xingu")
		r.ErrorContains(p.validateCandidateRegister(ctx, mustRegister(t, false, nil)),
			"must include BLS public key")
		r.NoError(p.validateCandidateRegister(ctx, mustRegister(t, true, nil)))
	})

	t.Run("zanzibar: BLS optional", func(t *testing.T) {
		ctx := ctxAtEra(t, "zanzibar")
		// no BLS at all -- now accepted
		r.NoError(p.validateCandidateRegister(ctx, mustRegister(t, false, nil)))
		// BLS with PoP -- accepted
		r.NoError(p.validateCandidateRegister(ctx, mustRegister(t, true, pop)))
		// BLS without PoP passes validation; the handler rejects it with
		// ErrUnauthorizedOperator once EnforceBLSPoP is on.
		r.NoError(p.validateCandidateRegister(ctx, mustRegister(t, true, nil)))
	})
}

func TestValidateCandidateUpdateBLSEras(t *testing.T) {
	r := require.New(t)
	p := &Protocol{}
	pk := blsKeyFromSeed(t, "upd").PublicKey().Bytes()
	pop := []byte("some-pop-bytes-not-verified-here-------------------------------------------------------------")
	op := identityset.Address(2).String()
	rew := identityset.Address(3).String()

	plain, err := action.NewCandidateUpdate("cand", op, rew)
	r.NoError(err)
	withKeyNoPoP, err := action.NewCandidateUpdateWithBLS("cand", op, rew, pk, nil)
	r.NoError(err)
	withKeyAndPoP, err := action.NewCandidateUpdateWithBLS("cand", op, rew, pk, pop)
	r.NoError(err)

	t.Run("xingu: BLS mandatory", func(t *testing.T) {
		ctx := ctxAtEra(t, "xingu")
		r.ErrorContains(p.validateCandidateUpdate(ctx, plain), "must include BLS public key")
		r.NoError(p.validateCandidateUpdate(ctx, withKeyNoPoP))
	})

	t.Run("zanzibar: BLS optional, paired when present", func(t *testing.T) {
		ctx := ctxAtEra(t, "zanzibar")
		r.NoError(p.validateCandidateUpdate(ctx, plain))
		r.NoError(p.validateCandidateUpdate(ctx, withKeyAndPoP))
		// Key without PoP is the handler's business, not validation's.
		r.NoError(p.validateCandidateUpdate(ctx, withKeyNoPoP))
	})

	t.Run("pre-xingu: BLS forbidden", func(t *testing.T) {
		ctx := ctxAtEra(t, "pre-xingu")
		r.NoError(p.validateCandidateUpdate(ctx, plain))
		r.ErrorContains(p.validateCandidateUpdate(ctx, withKeyAndPoP), "cannot include BLS public key")
	})
}
