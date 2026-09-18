// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// A node that understands blsPop must reach the same verdict on every pre-fork
// block as one that does not. These tests cover the "gate off, non-empty PoP"
// cell that TestHandleCandidateRegister_PoPGate leaves open -- it exercises
// gate on x {valid, empty, invalid} and gate off x empty, which is every
// combination except the one that splits the chain.

func TestRejectPrePoPForkPop(t *testing.T) {
	r := require.New(t)
	pop := []byte("some-pop-bytes")

	off := protocol.FeatureCtx{EnforceBLSPoP: false}
	on := protocol.FeatureCtx{EnforceBLSPoP: true}

	// Only one cell rejects: the gate is off and a PoP is present.
	r.ErrorIs(rejectPrePoPForkPop(off, pop), action.ErrInvalidAct)
	r.ErrorContains(rejectPrePoPForkPop(off, pop), "not accepted before the PoP fork")

	r.NoError(rejectPrePoPForkPop(off, nil), "no PoP pre-fork is the legacy shape")
	r.NoError(rejectPrePoPForkPop(off, []byte{}), "empty and nil must behave alike")
	r.NoError(rejectPrePoPForkPop(on, pop), "a PoP is the point of the fork")
	r.NoError(rejectPrePoPForkPop(on, nil), "the handler, not validation, rejects a missing PoP")
}

// TestValidateRejectsPoPBeforeFork is the regression guard for the pre-fork
// chain split. Rejection has to happen for every pre-fork era, not just the
// one immediately before activation: the divergence is reachable at any height
// the attacker picks, through either the V2 ABI selectors or a native
// protobuf action carrying blsPop.
func TestValidateRejectsPoPBeforeFork(t *testing.T) {
	r := require.New(t)
	p := &Protocol{}
	p.config.RegistrationConsts.MinSelfStake = big.NewInt(0)

	pop := []byte("some-pop-bytes-not-verified-at-validation-time")
	pk := blsKeyFromSeed(t, "prefork-parity").PublicKey().Bytes()
	op := identityset.Address(2).String()
	rew := identityset.Address(3).String()

	updWithPoP, err := action.NewCandidateUpdateWithBLS("cand", op, rew, pk, pop)
	r.NoError(err)

	for _, era := range []string{"pre-xingu", "xingu"} {
		t.Run(era+": register with PoP rejected", func(t *testing.T) {
			ctx := ctxAtEra(t, era)
			err := p.validateCandidateRegister(ctx, mustRegister(t, true, pop))
			r.ErrorIs(err, action.ErrInvalidAct)
			r.ErrorContains(err, "not accepted before the PoP fork")
		})

		t.Run(era+": update with PoP rejected", func(t *testing.T) {
			ctx := ctxAtEra(t, era)
			err := p.validateCandidateUpdate(ctx, updWithPoP)
			r.ErrorIs(err, action.ErrInvalidAct)
			r.ErrorContains(err, "not accepted before the PoP fork")
		})
	}

	// The other side of the boundary: once the gate is on, the same actions
	// pass validation and the handler takes over enforcing the PoP itself.
	t.Run("post-fork: both accepted", func(t *testing.T) {
		ctx := ctxAtEra(t, "optional")
		r.NoError(p.validateCandidateRegister(ctx, mustRegister(t, true, pop)))
		r.NoError(p.validateCandidateUpdate(ctx, updWithPoP))
	})

	// The era switch reads act.WithBLS(), which looks at the public key and
	// never at the PoP, so the gate check has to sit outside it. Pin that:
	// an action whose key makes the switch take a branch that ignores blsPop
	// must still be rejected pre-fork.
	t.Run("pre-fork: rejection does not depend on the era branch", func(t *testing.T) {
		for _, era := range []string{"pre-xingu", "xingu", "optional"} {
			ctx := ctxAtEra(t, era)
			// "optional" is the post-fork era, where the gate is on and the
			// action is legitimate; the other two must reject it.
			err := p.validateCandidateUpdate(ctx, updWithPoP)
			if era == "optional" {
				r.NoError(err, "era %s", era)
				continue
			}
			r.ErrorContains(err, "not accepted before the PoP fork", "era %s", era)
		}
	})
}
