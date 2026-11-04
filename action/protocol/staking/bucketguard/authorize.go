package bucketguard

import (
	"bytes"
	"crypto/sha512"
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
	if err := verifyAccept(d, m.BucketIndex, gen+1, owner, m.AlgID, m.PublicKey, [CallHashLen]byte{}, m.AcceptSig); err != nil {
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

// OpEnv is what CheckOp needs from the execution context.
type OpEnv struct {
	// Height is the height of the block being executed.
	Height uint64
	// Sender is the outer sender of the action.
	Sender address.Address
	// BlockHash returns the hash of the committed block at a height below
	// Height. An error is a storage failure, not a mismatch; CheckOp returns
	// it unwrapped so the staking protocol treats it as fatal (IIP §9.3).
	BlockHash func(height uint64) ([32]byte, error)
}

// CheckOp runs the guard part of IIP §9.2 steps 2 to 4, in this order:
//
//  2. the bucket must be guarded (ErrBucketNotGuarded);
//  3. a non-empty sponsor must be the sender (ErrSponsorMismatch);
//  4. the op must match the stored generation and nonce, no counter may
//     overflow, anchorHeight <= height <= deadline, deadline <=
//     anchorHeight + MaxDeadlineWindow, and the block at anchorHeight must
//     have hash anchorHash (ErrGuardMismatch).
//
// The heights are compared before the block hash is looked up, so only the
// last MaxDeadlineWindow blocks are ever looked up. The block being executed
// has no hash yet, so an anchor at env.Height fails without a lookup. Call
// CheckOp after loading the native bucket (step 2), and before the
// op-specific staking preconditions (step 5) and VerifyOp (step 6).
func CheckOp(p Params, cur *Guard, op *GuardedBucketOp, env OpEnv) error {
	if !cur.IsGuarded() {
		return fmt.Errorf("%w: bucket %d", ErrBucketNotGuarded, op.BucketIndex)
	}
	if len(op.Sponsor) > 0 && (env.Sender == nil || !bytes.Equal(env.Sender.Bytes(), op.Sponsor)) {
		return fmt.Errorf("%w: op is bound to sponsor %x", ErrSponsorMismatch, op.Sponsor)
	}
	switch {
	case op.Generation != cur.Generation || op.Nonce != cur.Nonce:
		return fmt.Errorf("%w: op is for generation %d nonce %d, guard is at %d/%d",
			ErrGuardMismatch, op.Generation, op.Nonce, cur.Generation, cur.Nonce)
	case cur.Nonce == math.MaxUint64:
		return fmt.Errorf("%w: nonce overflow", ErrGuardMismatch)
	case op.changesKey() && cur.Generation == math.MaxUint64:
		return fmt.Errorf("%w: generation overflow", ErrGuardMismatch)
	case env.Height < op.AnchorHeight:
		return fmt.Errorf("%w: anchor %d is above height %d", ErrGuardMismatch, op.AnchorHeight, env.Height)
	case env.Height > op.Deadline:
		return fmt.Errorf("%w: deadline %d passed at height %d", ErrGuardMismatch, op.Deadline, env.Height)
	case op.Deadline-op.AnchorHeight > p.MaxDeadlineWindow:
		// op.AnchorHeight <= env.Height <= op.Deadline here, so no underflow.
		return fmt.Errorf("%w: deadline %d is more than %d blocks after anchor %d",
			ErrGuardMismatch, op.Deadline, p.MaxDeadlineWindow, op.AnchorHeight)
	case op.AnchorHeight == env.Height:
		return fmt.Errorf("%w: anchor %d is the block being executed", ErrGuardMismatch, op.AnchorHeight)
	}
	h, err := env.BlockHash(op.AnchorHeight)
	if err != nil {
		return fmt.Errorf("failed to read the hash of anchor block %d: %w", op.AnchorHeight, err)
	}
	if !bytes.Equal(h[:], op.AnchorHash) {
		return fmt.Errorf("%w: block %d has hash %x, op is anchored to %x", ErrGuardMismatch, op.AnchorHeight, h, op.AnchorHash)
	}
	return nil
}

// CheckCandidate is the CHANGE_CANDIDATE precondition of IIP §9.2 step 5:
// the candidate the staking protocol resolved from op.CandidateName at
// execution time must be the one the owner signed for.
func (m *GuardedBucketOp) CheckCandidate(resolved address.Address) error {
	if resolved == nil || !bytes.Equal(resolved.Bytes(), m.ExpectedCandidateID) {
		return ErrCandidateMismatch
	}
	return nil
}

// Grant is the authority of one verified guarded operation (IIP §6, §10).
// Only VerifyOp issues one, so code holding a Grant has checked the
// signatures. The staking protocol passes it to the writes that apply the
// operation, which accept it only for the bucket and counters it was issued
// for.
type Grant struct {
	bucketIndex, generation, nonce uint64
	op                             Op
}

// Op returns the operation g authorizes.
func (g *Grant) Op() Op { return g.op }

// Covers reports whether g authorizes a write to bucket index whose current
// guard record is cur.
func (g *Grant) Covers(index uint64, cur *Guard) bool {
	return g != nil && cur.IsGuarded() && g.bucketIndex == index &&
		g.generation == cur.Generation && g.nonce == cur.Nonce
}

// VerifyOp runs IIP §9.2 step 6: the CALL signature by the current key and,
// when the op installs a new key, the ACCEPT signature by that key over the
// hash of this CALL. owner is the bucket owner at execution time. On success
// it returns the Grant that authorizes applying op.
func VerifyOp(p Params, d Domain, owner address.Address, cur *Guard, op *GuardedBucketOp) (*Grant, error) {
	pk, err := pqcrypto.ParsePublicKey(cur.AlgID, cur.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("%w: stored key: %v", ErrGuardSignature, err)
	}
	call := EncodeCall(d, owner, op)
	if !pk.Verify([]byte(CallContext), call, op.Sig) {
		return nil, fmt.Errorf("%w: CALL", ErrGuardSignature)
	}
	if op.installsKey() {
		ng := op.NewGuard
		if !p.accepts(ng.AlgID) {
			return nil, malformed("unsupported algorithm %s", ng.AlgID)
		}
		resultingOwner := owner
		if op.Op == OpTransfer {
			if resultingOwner, err = address.FromBytes(op.NewOwner); err != nil {
				return nil, malformed("TRANSFER newOwner: %v", err)
			}
		}
		if err := verifyAccept(d, op.BucketIndex, cur.Generation+1, resultingOwner, ng.AlgID, ng.PublicKey, sha512.Sum512(call), ng.AcceptSig); err != nil {
			return nil, err
		}
	}
	return &Grant{bucketIndex: op.BucketIndex, generation: cur.Generation, nonce: cur.Nonce, op: op.Op}, nil
}

func verifyAccept(d Domain, bucketIndex, newGeneration uint64, resultingOwner address.Address, alg pqcrypto.AlgID, publicKey []byte, callHash [CallHashLen]byte, sig []byte) error {
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
// legacy staking action that changes a native bucket's owner, candidate,
// duration, auto-stake flag, stake status or existence calls it first, so it
// reports ErrBucketGuarded rather than a later precondition's status;
// deposits and IIP-59 compounding do not. The staking protocol's write layer
// enforces the same rule on every write (IIP §10).
func RejectIfGuarded(g *Guard) error {
	if g.IsGuarded() {
		return ErrBucketGuarded
	}
	return nil
}
