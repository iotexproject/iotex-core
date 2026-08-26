// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"math"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// ctxAtEra builds a validation context for one of the three BLS eras:
// pre-Xingu (no BLS at all), Xingu (BLS mandatory) and post-fork (BLS
// optional).
func ctxAtEra(t *testing.T, era string) context.Context {
	t.Helper()
	g := genesis.TestDefault()
	switch era {
	case "pre-xingu":
		g.XinguBlockHeight = 100
		g.XinguBetaBlockHeight = 100
		g.YapBlockHeight = 100
		g.YapBetaBlockHeight = 100
		g.ZanzibarBlockHeight = math.MaxUint64
		g.ZanzibarBetaBlockHeight = math.MaxUint64
	case "xingu":
		g.XinguBlockHeight = 0
		g.ZanzibarBlockHeight = math.MaxUint64
		g.ZanzibarBetaBlockHeight = math.MaxUint64
	case "optional":
		g.XinguBlockHeight = 0
		g.ZanzibarBlockHeight = 0
		g.ZanzibarBetaBlockHeight = 0
	default:
		t.Fatalf("unknown era %s", era)
	}
	ctx := genesis.WithGenesisContext(context.Background(), g)
	return protocol.WithFeatureCtx(protocol.WithBlockCtx(ctx, protocol.BlockCtx{BlockHeight: 1}))
}

// mustRegister builds either the legacy shape (no BLS, amount in the legacy
// field) or the BLS shape (amount in value).
func mustRegister(t *testing.T, withBLS bool, pop []byte) *action.CandidateRegister {
	t.Helper()
	owner := identityset.Address(1).String()
	op := identityset.Address(2).String()
	rew := identityset.Address(3).String()
	amount := "1200000000000000000000000"
	if !withBLS {
		cr, err := action.NewCandidateRegister("cand", op, rew, owner, amount, 1, true, nil)
		require.NoError(t, err)
		return cr
	}
	pk := blsKeyFromSeed(t, "pair").PublicKey().Bytes()
	cr, err := action.NewCandidateRegisterWithBLS("cand", op, rew, owner, amount, 1, true, pk, pop, nil)
	require.NoError(t, err)
	return cr
}

// TestValidateBLSPairing covers the half of the pairing rule that validation
// owns. A PoP with no key is unreachable through the ABI -- every
// candidateRegisterWithBLS* method rejects an empty blsPubKey at decode time --
// so it is exercised directly. The mirror case, a key with no PoP, is the
// handler's business via EnforceBLSPoP; see TestHandleCandidateRegister_PoPGate.
func TestValidateBLSPairing(t *testing.T) {
	r := require.New(t)
	pop := []byte("pop")
	r.NoError(validateBLSPairing(false, nil))
	r.NoError(validateBLSPairing(true, pop))
	r.NoError(validateBLSPairing(true, nil))
	r.ErrorContains(validateBLSPairing(false, pop), "must be accompanied by a public key")
}

func TestValidateCandidateRegisterBLSEras(t *testing.T) {
	r := require.New(t)
	p := &Protocol{}
	p.config.RegistrationConsts.MinSelfStake = big.NewInt(0)
	pop := []byte("some-pop-bytes-not-verified-at-validation-time")

	t.Run("xingu: BLS mandatory", func(t *testing.T) {
		ctx := ctxAtEra(t, "xingu")
		r.ErrorContains(p.validateCandidateRegister(ctx, mustRegister(t, false, nil)),
			"must include BLS public key")
		r.NoError(p.validateCandidateRegister(ctx, mustRegister(t, true, nil)))
	})

	t.Run("post-fork: BLS optional", func(t *testing.T) {
		ctx := ctxAtEra(t, "optional")
		// No BLS at all -- now accepted, through the legacy entry.
		r.NoError(p.validateCandidateRegister(ctx, mustRegister(t, false, nil)))
		// BLS with PoP -- accepted.
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
	pop := []byte("some-pop-bytes-not-verified-at-validation-time")
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

	t.Run("post-fork: BLS optional", func(t *testing.T) {
		ctx := ctxAtEra(t, "optional")
		r.NoError(p.validateCandidateUpdate(ctx, plain))
		r.NoError(p.validateCandidateUpdate(ctx, withKeyAndPoP))
		r.NoError(p.validateCandidateUpdate(ctx, withKeyNoPoP))
	})

	t.Run("pre-xingu: BLS forbidden", func(t *testing.T) {
		ctx := ctxAtEra(t, "pre-xingu")
		r.NoError(p.validateCandidateUpdate(ctx, plain))
		// Asserted with the no-PoP shape so this stays a statement about the
		// key alone. A PoP-carrying action is rejected earlier and for a
		// different reason -- see TestValidateRejectsPoPBeforeFork.
		r.ErrorContains(p.validateCandidateUpdate(ctx, withKeyNoPoP), "cannot include BLS public key")
	})
}
