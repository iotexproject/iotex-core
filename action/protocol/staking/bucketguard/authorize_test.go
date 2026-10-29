package bucketguard

import (
	"bytes"
	"errors"
	"math"
	"testing"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto/signer"
)

var (
	testGenesis = [32]byte{0x9e}
	testDomain  = NewDomain(2, testGenesis) // testnet native chain ID
	// Same chain ID on another genesis, and the same genesis on another chain.
	otherGenesisDomain = NewDomain(2, [32]byte{0x9f})
	otherChainDomain   = NewDomain(1, testGenesis)
)

func signBind(t *testing.T, k *signer.PrivateKey, bucket, newGen uint64, owner address.Address) *BindBucketGuard {
	t.Helper()
	pub := k.PublicKey().Bytes()
	sig, err := k.Sign([]byte(AcceptContext), EncodeAccept(testDomain, bucket, newGen, owner, alg, pub, [32]byte{}))
	if err != nil {
		t.Fatal(err)
	}
	return &BindBucketGuard{BucketIndex: bucket, AlgID: alg, PublicKey: pub, AcceptSig: sig}
}

// signOp fills op.Sig with cur's CALL signature and, when next != nil, attaches
// next as the incoming key with its ACCEPT of this CALL for resultingOwner.
func signOp(t *testing.T, cur *signer.PrivateKey, owner address.Address, op *GuardedBucketOp, next *signer.PrivateKey, resultingOwner address.Address) {
	t.Helper()
	if next != nil {
		pub := next.PublicKey().Bytes()
		op.NewGuard = &NewGuard{AlgID: alg, PublicKey: pub}
		h := CallHash(testDomain, owner, op)
		acc, err := next.Sign([]byte(AcceptContext), EncodeAccept(testDomain, op.BucketIndex, op.Generation+1, resultingOwner, alg, pub, h))
		if err != nil {
			t.Fatal(err)
		}
		op.NewGuard.AcceptSig = acc
	}
	sig, err := cur.Sign([]byte(CallContext), EncodeCall(testDomain, owner, op))
	if err != nil {
		t.Fatal(err)
	}
	op.Sig = sig
}

// authorize runs the GuardedBucketOp steps in IIP order and returns the next record.
func authorize(t *testing.T, cur *Guard, owner address.Address, op *GuardedBucketOp, height uint64) (*Guard, bool) {
	t.Helper()
	p := ParamsV1()
	if err := op.Validate(p); err != nil {
		t.Fatalf("%s: Validate: %v", op.Op, err)
	}
	if err := CheckOp(cur, op, height); err != nil {
		t.Fatalf("%s: CheckOp: %v", op.Op, err)
	}
	if err := VerifyOp(p, testDomain, owner, cur, op); err != nil {
		t.Fatalf("%s: VerifyOp: %v", op.Op, err)
	}
	return Next(cur, op, height)
}

func wantErr(t *testing.T, name string, err, target error) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Errorf("%s: want %v, got %v", name, target, err)
	}
}

// TestLifecycle walks one bucket through bind, every op kind, a clear and a
// rebind, checking the record after each step and that stale signatures fail.
func TestLifecycle(t *testing.T) {
	p := ParamsV1()
	alice, bob := testAddr(t, 0xa1), testAddr(t, 0xb0)
	keyA, keyB, keyC, keyD := newTestKey(t, 1), newTestKey(t, 2), newTestKey(t, 3), newTestKey(t, 4)
	const bucket = 7

	// BIND by alice with key A.
	bind := signBind(t, keyA, bucket, 1, alice)
	if err := bind.Validate(p); err != nil {
		t.Fatal(err)
	}
	g, err := AuthorizeBind(p, testDomain, alice, BindScope{}, nil, bind, 100)
	if err != nil {
		t.Fatal(err)
	}
	if !g.IsGuarded() || g.Generation != 1 || g.Nonce != 0 || g.BoundHeight != 100 {
		t.Fatalf("after bind: %+v", g)
	}
	wantErr(t, "RejectIfGuarded", RejectIfGuarded(g), ErrBucketGuarded)
	_, err = AuthorizeBind(p, testDomain, alice, BindScope{}, g, bind, 101)
	wantErr(t, "bind twice", err, ErrGuardScope)

	// UNSTAKE with key A.
	unstake := &GuardedBucketOp{BucketIndex: bucket, Generation: 1, Nonce: 0, Deadline: 200, Op: OpUnstake}
	signOp(t, keyA, alice, unstake, nil, nil)
	g, _ = authorize(t, g, alice, unstake, 150)
	if g.Generation != 1 || g.Nonce != 1 {
		t.Fatalf("after unstake: %+v", g)
	}
	wantErr(t, "replay unstake", CheckOp(g, unstake, 150), ErrGuardMismatch)

	// ROTATE A -> B.
	rotate := &GuardedBucketOp{BucketIndex: bucket, Generation: 1, Nonce: 1, Deadline: 300, Op: OpRotate}
	signOp(t, keyA, alice, rotate, keyB, alice)
	g, _ = authorize(t, g, alice, rotate, 250)
	if !g.IsGuarded() || g.Generation != 2 || g.Nonce != 2 || g.BoundHeight != 250 || !hasKey(g, keyB) {
		t.Fatalf("after rotate: %+v", g)
	}

	// Key A no longer authorizes anything.
	stale := &GuardedBucketOp{BucketIndex: bucket, Generation: 2, Nonce: 2, Deadline: 400, Op: OpUnstake}
	signOp(t, keyA, alice, stale, nil, nil)
	wantErr(t, "old key", VerifyOp(p, testDomain, alice, g, stale), ErrGuardSignature)

	// TRANSFER to bob with bob's key C.
	transfer := &GuardedBucketOp{BucketIndex: bucket, Generation: 2, Nonce: 2, Deadline: 400, Op: OpTransfer, NewOwner: bob.Bytes()}
	signOp(t, keyB, alice, transfer, keyC, bob)
	g, _ = authorize(t, g, alice, transfer, 350)
	if g.Generation != 3 || g.Nonce != 3 || !hasKey(g, keyC) {
		t.Fatalf("after transfer: %+v", g)
	}

	// TRANSFER back to alice clearing the guard: the tombstone keeps the
	// counters, advances the generation and records the clearing height.
	clear := &GuardedBucketOp{BucketIndex: bucket, Generation: 3, Nonce: 3, Deadline: 500, Op: OpTransfer, NewOwner: alice.Bytes(), ClearGuard: true}
	signOp(t, keyC, bob, clear, nil, nil)
	g, _ = authorize(t, g, bob, clear, 450)
	if g.IsGuarded() || g.Generation != 4 || g.Nonce != 4 || g.AlgID != 0 || g.BoundHeight != 450 {
		t.Fatalf("after clear: %+v", g)
	}
	if err := RejectIfGuarded(g); err != nil {
		t.Fatalf("tombstone rejected legacy path: %v", err)
	}
	wantErr(t, "op on tombstone", CheckOp(g, clear, 450), ErrBucketNotGuarded)

	// Rebind with key D: generation continues from the tombstone.
	g, err = AuthorizeBind(p, testDomain, alice, BindScope{}, g, signBind(t, keyD, bucket, 5, alice), 500)
	if err != nil {
		t.Fatal(err)
	}
	if g.Generation != 5 || g.Nonce != 4 {
		t.Fatalf("after rebind: %+v", g)
	}

	// WITHDRAW deletes the record.
	withdraw := &GuardedBucketOp{BucketIndex: bucket, Generation: 5, Nonce: 4, Deadline: 600, Op: OpWithdraw, Recipient: bob.Bytes()}
	signOp(t, keyD, alice, withdraw, nil, nil)
	if next, deleted := authorize(t, g, alice, withdraw, 550); !deleted || next != nil {
		t.Fatalf("withdraw: next=%+v deleted=%v", next, deleted)
	}
}

func hasKey(g *Guard, k *signer.PrivateKey) bool {
	return g.AlgID == k.PublicKey().Alg() && bytes.Equal(g.PublicKey, k.PublicKey().Bytes())
}

func TestAuthorizeBindErrors(t *testing.T) {
	p := ParamsV1()
	alice := testAddr(t, 0xa1)
	k := newTestKey(t, 1)
	bind := signBind(t, k, 7, 1, alice)

	_, err := AuthorizeBind(p, testDomain, alice, BindScope{SelfStake: true}, nil, bind, 1)
	wantErr(t, "self-stake", err, ErrGuardScope)
	_, err = AuthorizeBind(p, testDomain, alice, BindScope{Endorsed: true}, nil, bind, 1)
	wantErr(t, "endorsed", err, ErrGuardScope)
	_, err = AuthorizeBind(p, testDomain, testAddr(t, 0xb0), BindScope{}, nil, bind, 1)
	wantErr(t, "other owner", err, ErrGuardSignature)
	_, err = AuthorizeBind(p, otherChainDomain, alice, BindScope{}, nil, bind, 1)
	wantErr(t, "other chain", err, ErrGuardSignature)
	_, err = AuthorizeBind(p, otherGenesisDomain, alice, BindScope{}, nil, bind, 1)
	wantErr(t, "other genesis", err, ErrGuardSignature)
	// ACCEPT was signed for generation 1; a tombstone at generation 5 needs 6.
	_, err = AuthorizeBind(p, testDomain, alice, BindScope{}, &Guard{Generation: 5, Nonce: 9}, bind, 1)
	wantErr(t, "stale generation", err, ErrGuardSignature)
	_, err = AuthorizeBind(p, testDomain, alice, BindScope{}, &Guard{Generation: math.MaxUint64}, bind, 1)
	wantErr(t, "generation overflow", err, ErrGuardMismatch)

	g, err := AuthorizeBind(p, testDomain, alice, BindScope{}, &Guard{Generation: 5, Nonce: 9}, signBind(t, k, 7, 6, alice), 1)
	if err != nil || g.Generation != 6 || g.Nonce != 9 {
		t.Fatalf("rebind over tombstone: %+v, %v", g, err)
	}
}

func TestCheckAndVerifyOpErrors(t *testing.T) {
	p := ParamsV1()
	alice, bob := testAddr(t, 0xa1), testAddr(t, 0xb0)
	k := newTestKey(t, 1)
	g := &Guard{AlgID: alg, PublicKey: k.PublicKey().Bytes(), Generation: 3, Nonce: 8, BoundHeight: 10}
	op := &GuardedBucketOp{BucketIndex: 7, Generation: 3, Nonce: 8, Deadline: 100, Op: OpWithdraw, Recipient: bob.Bytes()}
	signOp(t, k, alice, op, nil, nil)

	wantErr(t, "no record", CheckOp(nil, op, 50), ErrBucketNotGuarded)
	wantErr(t, "generation", CheckOp(&Guard{AlgID: alg, PublicKey: g.PublicKey, Generation: 4, Nonce: 8}, op, 50), ErrGuardMismatch)
	wantErr(t, "nonce", CheckOp(&Guard{AlgID: alg, PublicKey: g.PublicKey, Generation: 3, Nonce: 9}, op, 50), ErrGuardMismatch)
	if err := CheckOp(g, op, 100); err != nil {
		t.Fatalf("deadline height must be inclusive: %v", err)
	}
	wantErr(t, "deadline", CheckOp(g, op, 101), ErrGuardMismatch)
	maxed := *op
	maxed.Nonce = math.MaxUint64
	wantErr(t, "nonce overflow", CheckOp(&Guard{AlgID: alg, PublicKey: g.PublicKey, Generation: 3, Nonce: math.MaxUint64}, &maxed, 50), ErrGuardMismatch)
	clearMax := &GuardedBucketOp{BucketIndex: 7, Generation: math.MaxUint64, Nonce: 8, Deadline: 100, Op: OpTransfer, NewOwner: bob.Bytes(), ClearGuard: true}
	wantErr(t, "clear generation overflow", CheckOp(&Guard{AlgID: alg, PublicKey: g.PublicKey, Generation: math.MaxUint64, Nonce: 8}, clearMax, 50), ErrGuardMismatch)

	if err := VerifyOp(p, testDomain, alice, g, op); err != nil {
		t.Fatal(err)
	}
	// The owner is bound at execution time: a transfer after signing voids the op.
	wantErr(t, "owner changed", VerifyOp(p, testDomain, bob, g, op), ErrGuardSignature)
	wantErr(t, "chain", VerifyOp(p, otherChainDomain, alice, g, op), ErrGuardSignature)
	wantErr(t, "genesis", VerifyOp(p, otherGenesisDomain, alice, g, op), ErrGuardSignature)
	tampered := *op
	tampered.Recipient = alice.Bytes()
	wantErr(t, "recipient changed", VerifyOp(p, testDomain, alice, g, &tampered), ErrGuardSignature)
	tampered = *op
	tampered.Sponsor = bob.Bytes()
	wantErr(t, "sponsor changed", VerifyOp(p, testDomain, alice, g, &tampered), ErrGuardSignature)

	// A CALL signed under the ACCEPT context is not a CALL signature.
	wrongCtx := *op
	var err error
	if wrongCtx.Sig, err = k.Sign([]byte(AcceptContext), EncodeCall(testDomain, alice, op)); err != nil {
		t.Fatal(err)
	}
	wantErr(t, "CALL under ACCEPT context", VerifyOp(p, testDomain, alice, g, &wrongCtx), ErrGuardSignature)

	// ACCEPT signed for the wrong resulting owner.
	rot := &GuardedBucketOp{BucketIndex: 7, Generation: 3, Nonce: 8, Deadline: 100, Op: OpRotate}
	signOp(t, k, alice, rot, newTestKey(t, 2), bob)
	wantErr(t, "accept owner", VerifyOp(p, testDomain, alice, g, rot), ErrGuardSignature)

	// An ACCEPT is bound to one CALL: reusing it with another CALL that has
	// the same bucket, generation, owner and key fails, even when the current
	// key signs the new CALL.
	rot = &GuardedBucketOp{BucketIndex: 7, Generation: 3, Nonce: 8, Deadline: 100, Op: OpRotate}
	signOp(t, k, alice, rot, newTestKey(t, 2), alice)
	if err := VerifyOp(p, testDomain, alice, g, rot); err != nil {
		t.Fatal(err)
	}
	other := *rot
	other.Deadline = 99
	if other.Sig, err = k.Sign([]byte(CallContext), EncodeCall(testDomain, alice, &other)); err != nil {
		t.Fatal(err)
	}
	wantErr(t, "accept reused", VerifyOp(p, testDomain, alice, g, &other), ErrGuardSignature)
}

func TestCheckCandidate(t *testing.T) {
	cand := testAddr(t, 0xcc)
	op := &GuardedBucketOp{Op: OpChangeCandidate, CandidateName: "abc", ExpectedCandidateID: cand.Bytes()}
	if err := op.CheckCandidate(cand); err != nil {
		t.Fatal(err)
	}
	wantErr(t, "other candidate", op.CheckCandidate(testAddr(t, 0xcd)), ErrCandidateMismatch)
	wantErr(t, "unresolved", op.CheckCandidate(nil), ErrCandidateMismatch)
}

func TestNextDoesNotMutate(t *testing.T) {
	cur := &Guard{AlgID: alg, PublicKey: zeroPub(), Generation: 2, Nonce: 5, BoundHeight: 9}
	before := *cur
	next, _ := Next(cur, &GuardedBucketOp{Op: OpRotate, NewGuard: &NewGuard{AlgID: alg, PublicKey: make([]byte, 48)}}, 20)
	next.PublicKey[0] = 1
	if cur.Generation != before.Generation || cur.Nonce != before.Nonce || cur.PublicKey[0] != 0 {
		t.Fatal("Next modified its input")
	}
}
