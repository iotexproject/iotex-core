// Copyright (c) 2026 IoTeX Foundation
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
)

// IsBucketGuardAction identifies the native actions covered by IIP-65 §9.6.
func IsBucketGuardAction(act action.Action) bool {
	switch act.(type) {
	case *action.BindBucketGuard, *action.GuardedBucketOp, *action.VaultAction:
		return true
	}
	return false
}

// PreflightBucketGuard checks the current state and signatures for local
// admission and gas estimation only. It must never be called by block validation.
// The caller supplies the transaction's nonce, not the confirmed account nonce:
// queued transactions may legitimately have a higher nonce. Guard nonces remain
// checked against committed guard state.
func (p *Protocol) PreflightBucketGuard(ctx context.Context, elp action.Envelope, sr protocol.StateReader) error {
	if !IsBucketGuardAction(elp.Action()) || !protocol.MustGetFeatureCtx(ctx).EnableBucketGuard {
		return nil
	}
	if err := elp.Action().SanityCheck(); err != nil {
		return err
	}
	if err := p.Validate(ctx, elp, sr); err != nil {
		return err
	}
	csm, err := NewCandidateStateManagerWithContext(ctx, &guardReadOnlyState{StateReader: sr})
	if err != nil {
		return err
	}
	switch act := elp.Action().(type) {
	case *action.BindBucketGuard:
		_, _, err = p.checkBindBucketGuard(ctx, act, csm, true)
	case *action.GuardedBucketOp:
		_, _, err = p.checkGuardedBucketOp(ctx, act, csm, true)
	case *action.VaultAction:
		_, _, err = p.checkVaultAction(ctx, act, csm, true)
	}
	return err
}

// No write-capable state handle is available to a preflight. These methods
// fail closed if a future handler accidentally moves a write before its check.
type guardReadOnlyState struct{ protocol.StateReader }

func (*guardReadOnlyState) Snapshot() int    { return 0 }
func (*guardReadOnlyState) Revert(int) error { return errors.New("guard preflight is read-only") }
func (*guardReadOnlyState) PutState(interface{}, ...protocol.StateOption) (uint64, error) {
	return 0, errors.New("guard preflight is read-only")
}
func (*guardReadOnlyState) DelState(...protocol.StateOption) (uint64, error) {
	return 0, errors.New("guard preflight is read-only")
}
func (*guardReadOnlyState) WriteView(string, protocol.View) error {
	return errors.New("guard preflight is read-only")
}
