package bucketguard

import (
	"bytes"
	"crypto/sha512"
	"math"
	"math/big"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto"
)

// SeedStakingBucket supplies an existing eligible native bucket to this MVP.
// Production eligibility and PoS timers remain the staking handler's job.
func (e *VaultEngine) SeedStakingBucket(index uint64, owner []byte, principal *big.Int) error {
	if e.staking[index] != nil || !validAddress(owner) || !validAmount(principal) || principal.Sign() == 0 {
		return ErrMalformed
	}
	e.staking[index] = &VaultBucket{index, bytes.Clone(owner), principal.Bytes()}
	return nil
}

// StakingBucket returns a copy of the simulated Class B principal and owner.
func (e *VaultEngine) StakingBucket(index uint64) *VaultBucket { return e.staking[index].clone() }

// BindGuardBV2 binds or rebinds with ACCEPT_B_V2, preserving isolated counters
// and cooldown across a tombstone. Delay is immutable until the guard clears.
func (e *VaultEngine) BindGuardBV2(env OpEnv, index, delay uint64, ng *NewGuard) error {
	v := e.staking[index]
	if v == nil {
		return ErrClassMismatch
	}
	if err := ownerOnly(v.Owner, env); err != nil {
		return err
	}
	if delay != 0 && delay != UnbindDelayBlocks || ng == nil {
		return ErrMalformed
	}
	g := e.guard(ClassBStaking, index)
	if g != nil && len(g.PublicKey) > 0 {
		return ErrBucketGuarded
	}
	if g == nil {
		g = &BucketGuardV2{BucketClass: ClassBStaking, NextRequestID: 1}
	} else {
		g = g.clone()
	}
	if g.Generation == math.MaxUint64 {
		return ErrGuardMismatch
	}
	msg := EncodeAcceptBV2(e.domain, index, g.Generation+1, v.Owner, delay, ng.AlgID, ng.PublicKey, [64]byte{})
	if err := verifyV2(ng.AlgID, ng.PublicKey, AcceptContext, msg, ng.AcceptSig); err != nil {
		return err
	}
	g.AlgID = ng.AlgID
	g.PublicKey = bytes.Clone(ng.PublicKey)
	g.Generation++
	g.BoundHeight = env.Height
	g.UnbindDelay = delay
	e.guards[VaultStateKey(9, index)] = g
	return nil
}
func (e *VaultEngine) activeB(index uint64) (*VaultBucket, *BucketGuardV2, error) {
	v := e.staking[index]
	g := e.guard(ClassBStaking, index)
	if v == nil || g == nil || g.BucketClass != ClassBStaking {
		return nil, nil, ErrClassMismatch
	}
	if len(g.PublicKey) == 0 {
		return nil, nil, ErrBucketNotGuarded
	}
	return v, g, nil
}

// GuardUnbindInitiate never increments guardNonce.
func (e *VaultEngine) GuardUnbindInitiate(env OpEnv, index uint64) error {
	v, g, err := e.activeB(index)
	if err != nil {
		return err
	}
	if err = ownerOnly(v.Owner, env); err != nil {
		return err
	}
	if g.UnbindDelay == 0 {
		return ErrBucketGuarded
	}
	if env.Height < g.CancelCooldown {
		return ErrCancelCooldown
	}
	return initiate(g, env.Height)
}

// GuardUnbindCancel verifies authentication before the five exhaustive request
// branches. An idempotent retry cannot extend cooldown or consume guardNonce.
func (e *VaultEngine) GuardUnbindCancel(env OpEnv, m *GuardCancel) error {
	if m == nil {
		return ErrMalformed
	}
	_, g, err := e.activeB(m.BucketIndex)
	if err != nil {
		return err
	}
	if len(m.Sig) != pqcrypto.AlgSLHDSA192s.SignatureSize() {
		return ErrMalformed
	}
	if err = checkWindow(g, m.Generation, m.Deadline, m.AnchorHeight, m.AnchorHash, m.Sponsor, env); err != nil {
		return err
	}
	if err = verifyV2(g.AlgID, g.PublicKey, GuardCancelContext, EncodeGuardCancel(e.domain, m), m.Sig); err != nil {
		return err
	}
	switch {
	case m.RequestID >= g.NextRequestID:
		return ErrFutureRequestId
	case g.Pending == nil:
		return nil
	case g.Pending.RequestID != m.RequestID:
		return ErrStaleRequestId
	case env.Height >= g.Pending.ExpiresAt:
		return ErrUnbindElapsed
	default:
		if env.Height > math.MaxUint64-CancelCooldownBlocks {
			return ErrGuardMismatch
		}
		g.Pending = nil
		g.CancelCooldown = env.Height + CancelCooldownBlocks
		return nil
	}
}

// GuardUnbindFinalize restores ECDSA authority only; principal and PoS timers
// are untouched, so it cannot be an emergency staking withdrawal.
func (e *VaultEngine) GuardUnbindFinalize(env OpEnv, index uint64) error {
	_, g, err := e.activeB(index)
	if err != nil {
		return err
	}
	if g.Pending == nil || env.Height < g.Pending.ExpiresAt {
		return ErrUnbindNotElapsed
	}
	if g.Generation == math.MaxUint64 {
		return ErrGuardMismatch
	}
	clearV2(g, env.Height)
	return nil
}

// StakingDeposit simulates owner deposits and system IIP-59 compounding.
// A skipped compound returns (false,nil): the caller keeps rewards claimable.
func (e *VaultEngine) StakingDeposit(env OpEnv, index uint64, amount *big.Int, compound bool) (bool, error) {
	v := e.staking[index]
	if v == nil {
		return false, ErrClassMismatch
	}
	if !validAmount(amount) || amount.Sign() == 0 {
		return false, ErrMalformed
	}
	if !compound {
		if err := ownerOnly(v.Owner, env); err != nil {
			return false, err
		}
	}
	g := e.guard(ClassBStaking, index)
	if compound && g != nil && g.Pending != nil {
		return false, nil
	}
	n := new(big.Int).Add(new(big.Int).SetBytes(v.Amount), amount)
	if n.BitLen() > 256 {
		return false, ErrMalformed
	}
	v.Amount = n.Bytes()
	return true, nil
}

// StakingGuardedOp models Draft 5 guard authorization and pending write-sets.
// checkNative is mandatory, read-only, and must enforce native PoS preconditions
// (including maturity/withdrawal waits). This MVP never pays out Class B funds.
func (e *VaultEngine) StakingGuardedOp(env OpEnv, m *GuardedBucketOp, checkNative func() error) error {
	if m == nil || checkNative == nil {
		return ErrMalformed
	}
	v, g, err := e.activeB(m.BucketIndex)
	if err != nil {
		return err
	}
	if err = m.Validate(ParamsV1()); err != nil {
		return err
	}
	old := &Guard{AlgID: g.AlgID, PublicKey: g.PublicKey, Generation: g.Generation, Nonce: g.GuardNonce, BoundHeight: g.BoundHeight}
	if err = CheckOp(ParamsV1(), old, m, env); err != nil {
		return err
	}
	pool := VaultPool()
	if bytes.Equal(m.Recipient, pool[:]) || bytes.Equal(m.NewOwner, pool[:]) ||
		(m.ClearGuard && forbiddenVaultAddress(m.NewOwner)) {
		return ErrMalformed
	}
	if m.Op == OpTransfer && bytes.Equal(m.NewOwner, v.Owner) {
		return ErrMalformed
	}
	if err = checkNative(); err != nil {
		return err
	}
	ownerAddress, err := address.FromBytes(v.Owner)
	if err != nil {
		return err
	}
	call := EncodeCall(e.domain, ownerAddress, m)
	if err = verifyV2(g.AlgID, g.PublicKey, CallContext, call, m.Sig); err != nil {
		return err
	}
	owner := v.Owner
	if m.Op == OpTransfer {
		owner = m.NewOwner
	}
	if m.NewGuard != nil {
		ng := m.NewGuard
		msg := EncodeAcceptBV2(e.domain, m.BucketIndex, g.Generation+1, owner, g.UnbindDelay, ng.AlgID, ng.PublicKey, sha512.Sum512(call))
		if err = verifyV2(ng.AlgID, ng.PublicKey, AcceptContext, msg, ng.AcceptSig); err != nil {
			return err
		}
	}
	if (m.Op == OpTransfer || m.Op == OpWithdraw) && g.Generation == math.MaxUint64 {
		return ErrGuardMismatch
	}
	g.GuardNonce++
	switch m.Op {
	case OpRotate, OpTransfer, OpWithdraw:
		g.Pending = nil
		g.Generation++
		g.BoundHeight = env.Height
	}
	if m.NewGuard != nil {
		g.AlgID = m.NewGuard.AlgID
		g.PublicKey = bytes.Clone(m.NewGuard.PublicKey)
	}
	if m.ClearGuard || m.Op == OpWithdraw {
		g.AlgID = 0
		g.PublicKey = nil
	}
	if m.Op == OpTransfer {
		v.Owner = bytes.Clone(m.NewOwner)
	}
	if m.Op == OpWithdraw {
		delete(e.staking, m.BucketIndex)
	}
	return nil
}
