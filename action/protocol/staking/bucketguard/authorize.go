package bucketguard

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"math"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto"
)

// The functions below are the stateful rules of the IIP, written as pure
// functions over the current guard record. The staking protocol reads the
// record, calls them in the order of the IIP execution steps (that order
// decides which receipt status wins when several checks fail), applies the
// bucket effect, and stores the record returned by AuthorizeBind or Next.

// BindScope holds the IIP §3 scope facts the staking protocol reads from
// state. Both must be computed structurally: SelfStake means some candidate's
// SelfStakeBucketIdx equals the bucket index (regardless of candidate status);
// Endorsed means an endorsement record exists for the bucket in any state.
// Neither may be replaced by isSelfStakeBucket or endorsement Status.
type BindScope struct {
	SelfStake bool
	Endorsed  bool
}

// AuthorizeBind runs IIP §5 execution steps 4–7 and returns the record to
// store. The caller has already run steps 1–3: snapshot, bucket exists and is
// native, caller is the owner. cur is the bucket's current record or nil.
func AuthorizeBind(p Params, d Domain, owner address.Address, scope BindScope, cur *Guard, m *BindBucketGuard, height uint64) (*Guard, error) {
	if scope.SelfStake || scope.Endorsed {
		return nil, fmt.Errorf("%w: self-stake or endorsed bucket", ErrGuardScope)
	}
	if cur.IsGuarded() {
		return nil, fmt.Errorf("%w: bucket %d is already guarded", ErrGuardScope, m.BucketIndex)
	}
	gen, nonce := cur.counters()
	if gen == math.MaxUint64 {
		return nil, fmt.Errorf("%w: generation overflow", ErrGuardMismatch)
	}
	if !p.accepts(m.AlgID) {
		return nil, malformed("unsupported algorithm %s", m.AlgID)
	}
	if err := verifyAccept(d, m.BucketIndex, gen+1, owner, m.AlgID, m.PublicKey, [32]byte{}, m.AcceptSig); err != nil {
		return nil, err
	}
	return &Guard{
		AlgID:       m.AlgID,
		PublicKey:   bytes.Clone(m.PublicKey),
		Generation:  gen + 1,
		Nonce:       nonce,
		BoundHeight: height,
	}, nil
}

// CheckOp runs IIP §9.2 steps 2–3 on the guard record: the bucket must be
// guarded, the op must match the stored generation and nonce, the deadline
// must not have passed, and no counter may overflow. Call it before the
// op-specific staking preconditions (step 4) and before VerifyOp (step 5).
func CheckOp(cur *Guard, op *GuardedBucketOp, height uint64) error {
	if !cur.IsGuarded() {
		return fmt.Errorf("%w: bucket %d", ErrBucketNotGuarded, op.BucketIndex)
	}
	switch {
	case op.Generation != cur.Generation || op.Nonce != cur.Nonce:
		return fmt.Errorf("%w: op is for generation %d nonce %d, guard is at %d/%d",
			ErrGuardMismatch, op.Generation, op.Nonce, cur.Generation, cur.Nonce)
	case height > op.Deadline:
		return fmt.Errorf("%w: deadline %d passed at height %d", ErrGuardMismatch, op.Deadline, height)
	case cur.Nonce == math.MaxUint64:
		return fmt.Errorf("%w: nonce overflow", ErrGuardMismatch)
	case op.changesKey() && cur.Generation == math.MaxUint64:
		return fmt.Errorf("%w: generation overflow", ErrGuardMismatch)
	}
	return nil
}

// CheckCandidate is the CHANGE_CANDIDATE precondition of IIP §9.2 step 4:
// the candidate the staking protocol resolved from op.CandidateName at
// execution time must be the one the owner signed for.
func (m *GuardedBucketOp) CheckCandidate(resolved address.Address) error {
	if resolved == nil || !bytes.Equal(resolved.Bytes(), m.ExpectedCandidateID) {
		return ErrCandidateMismatch
	}
	return nil
}

// VerifyOp runs IIP §9.2 step 5: the CALL signature by the current key and,
// when the op installs a new key, the ACCEPT signature by that key over the
// hash of this CALL. owner is the bucket owner at execution time.
func VerifyOp(p Params, d Domain, owner address.Address, cur *Guard, op *GuardedBucketOp) error {
	pk, err := pqcrypto.ParsePublicKey(cur.AlgID, cur.PublicKey)
	if err != nil {
		return fmt.Errorf("%w: stored key: %v", ErrGuardSignature, err)
	}
	call := EncodeCall(d, owner, op)
	if !pk.Verify([]byte(CallContext), call, op.Sig) {
		return fmt.Errorf("%w: CALL", ErrGuardSignature)
	}
	if !op.installsKey() {
		return nil
	}
	ng := op.NewGuard
	if !p.accepts(ng.AlgID) {
		return malformed("unsupported algorithm %s", ng.AlgID)
	}
	resultingOwner := owner
	if op.Op == OpTransfer {
		if resultingOwner, err = address.FromBytes(op.NewOwner); err != nil {
			return malformed("TRANSFER newOwner: %v", err)
		}
	}
	return verifyAccept(d, op.BucketIndex, cur.Generation+1, resultingOwner, ng.AlgID, ng.PublicKey, sha256.Sum256(call), ng.AcceptSig)
}

func verifyAccept(d Domain, bucketIndex, newGeneration uint64, resultingOwner address.Address, alg pqcrypto.AlgID, publicKey []byte, callHash [32]byte, sig []byte) error {
	pk, err := pqcrypto.ParsePublicKey(alg, publicKey)
	if err != nil {
		return malformed("incoming key: %v", err)
	}
	msg := EncodeAccept(d, bucketIndex, newGeneration, resultingOwner, alg, publicKey, callHash)
	if !pk.Verify([]byte(AcceptContext), msg, sig) {
		return fmt.Errorf("%w: ACCEPT", ErrGuardSignature)
	}
	return nil
}

// Next returns the record after op has succeeded (IIP §9.4). deleted means the
// record must be removed together with the bucket (WITHDRAW). cur is not
// modified. op must have passed Validate, CheckOp and VerifyOp.
func Next(cur *Guard, op *GuardedBucketOp, height uint64) (next *Guard, deleted bool) {
	if op.Op == OpWithdraw {
		return nil, true
	}
	next = &Guard{
		AlgID:       cur.AlgID,
		PublicKey:   bytes.Clone(cur.PublicKey),
		Generation:  cur.Generation,
		Nonce:       cur.Nonce + 1,
		BoundHeight: cur.BoundHeight,
	}
	switch {
	case op.installsKey():
		next.AlgID = op.NewGuard.AlgID
		next.PublicKey = bytes.Clone(op.NewGuard.PublicKey)
		next.Generation++
		next.BoundHeight = height
	case op.ClearGuard:
		next.AlgID, next.PublicKey = 0, nil
		next.Generation++
		next.BoundHeight = height
	}
	return next, false
}

// RejectIfGuarded returns ErrBucketGuarded when g guards its bucket. Every
// legacy staking path that changes a native bucket's owner, candidate,
// duration, auto-stake flag, stake status or existence must call it (IIP §10);
// deposits and IIP-59 compounding do not.
func RejectIfGuarded(g *Guard) error {
	if g.IsGuarded() {
		return ErrBucketGuarded
	}
	return nil
}
