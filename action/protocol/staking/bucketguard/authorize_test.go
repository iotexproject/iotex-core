package bucketguard

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
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
	sig, err := k.Sign([]byte(AcceptContext), EncodeAccept(testDomain, bucket, newGen, owner, alg, pub, [CallHashLen]byte{}))
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

// blockHash is the chain of the tests: block h has hash SHA-256(uint64_be(h)).
func blockHash(h uint64) ([32]byte, error) {
	return sha256.Sum256(binary.BigEndian.AppendUint64(nil, h)), nil
}

// anchored returns op anchored to block h of the test chain.
func anchored(op *GuardedBucketOp, h uint64) *GuardedBucketOp {
	hash, _ := blockHash(h)
	op.AnchorHeight, op.AnchorHash = h, hash[:]
	return op
}

// env is the execution context at height with the given sender.
func env(height uint64, sender address.Address) OpEnv {
	return OpEnv{Height: height, Sender: sender, BlockHash: blockHash}
}

// testSender submits the ops of the tests that do not bind a sponsor.
var testSender, _ = address.FromBytes(bytes.Repeat([]byte{0x5e}, 20))

// authorize runs the GuardedBucketOp steps in IIP order and returns the next record.
func authorize(t *testing.T, cur *Guard, owner address.Address, op *GuardedBucketOp, height uint64) (*Guard, bool) {
	t.Helper()
	p := ParamsV1()
	if err := op.Validate(p); err != nil {
		t.Fatalf("%s: Validate: %v", op.Op, err)
	}
	if err := CheckOp(p, cur, op, env(height, testSender)); err != nil {
		t.Fatalf("%s: CheckOp: %v", op.Op, err)
	}
	if err := verifyOp(p, testDomain, owner, cur, op); err != nil {
		t.Fatalf("%s: VerifyOp: %v", op.Op, err)
	}
	return Next(cur, op, height)
}

// verifyOp is VerifyOp for tests that only look at the error.
func verifyOp(p Params, d Domain, owner address.Address, cur *Guard, op *GuardedBucketOp) error {
	_, err := VerifyOp(p, d, owner, cur, op)
	return err
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
	unstake := anchored(&GuardedBucketOp{BucketIndex: bucket, Generation: 1, Nonce: 0, Deadline: 200, Op: OpUnstake}, 140)
	signOp(t, keyA, alice, unstake, nil, nil)
	g, _ = authorize(t, g, alice, unstake, 150)
	if g.Generation != 1 || g.Nonce != 1 {
		t.Fatalf("after unstake: %+v", g)
	}
	wantErr(t, "replay unstake", CheckOp(p, g, unstake, env(150, testSender)), ErrGuardMismatch)

	// ROTATE A -> B.
	rotate := anchored(&GuardedBucketOp{BucketIndex: bucket, Generation: 1, Nonce: 1, Deadline: 300, Op: OpRotate}, 240)
	signOp(t, keyA, alice, rotate, keyB, alice)
	g, _ = authorize(t, g, alice, rotate, 250)
	if !g.IsGuarded() || g.Generation != 2 || g.Nonce != 2 || g.BoundHeight != 250 || !hasKey(g, keyB) {
		t.Fatalf("after rotate: %+v", g)
	}

	// Key A no longer authorizes anything.
	stale := anchored(&GuardedBucketOp{BucketIndex: bucket, Generation: 2, Nonce: 2, Deadline: 400, Op: OpUnstake}, 300)
	signOp(t, keyA, alice, stale, nil, nil)
	wantErr(t, "old key", verifyOp(p, testDomain, alice, g, stale), ErrGuardSignature)

	// TRANSFER to bob with bob's key C.
	transfer := anchored(&GuardedBucketOp{BucketIndex: bucket, Generation: 2, Nonce: 2, Deadline: 400, Op: OpTransfer, NewOwner: bob.Bytes()}, 340)
	signOp(t, keyB, alice, transfer, keyC, bob)
	g, _ = authorize(t, g, alice, transfer, 350)
	if g.Generation != 3 || g.Nonce != 3 || !hasKey(g, keyC) {
		t.Fatalf("after transfer: %+v", g)
	}

	// TRANSFER back to alice clearing the guard: the tombstone keeps the
	// counters, advances the generation and records the clearing height.
	clear := anchored(&GuardedBucketOp{BucketIndex: bucket, Generation: 3, Nonce: 3, Deadline: 500, Op: OpTransfer, NewOwner: alice.Bytes(), ClearGuard: true}, 440)
	signOp(t, keyC, bob, clear, nil, nil)
	g, _ = authorize(t, g, bob, clear, 450)
	if g.IsGuarded() || g.Generation != 4 || g.Nonce != 4 || g.AlgID != 0 || g.BoundHeight != 450 {
		t.Fatalf("after clear: %+v", g)
	}
	if err := RejectIfGuarded(g); err != nil {
		t.Fatalf("tombstone rejected legacy path: %v", err)
	}
	wantErr(t, "op on tombstone", CheckOp(p, g, clear, env(450, testSender)), ErrBucketNotGuarded)

	// Rebind with key D: generation continues from the tombstone.
	g, err = AuthorizeBind(p, testDomain, alice, BindScope{}, g, signBind(t, keyD, bucket, 5, alice), 500)
	if err != nil {
		t.Fatal(err)
	}
	if g.Generation != 5 || g.Nonce != 4 {
		t.Fatalf("after rebind: %+v", g)
	}

	// WITHDRAW deletes the record.
	withdraw := anchored(&GuardedBucketOp{BucketIndex: bucket, Generation: 5, Nonce: 4, Deadline: 600, Op: OpWithdraw, Recipient: bob.Bytes()}, 540)
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
	op := anchored(&GuardedBucketOp{BucketIndex: 7, Generation: 3, Nonce: 8, Deadline: 100, Op: OpWithdraw, Recipient: bob.Bytes()}, 40)
	signOp(t, k, alice, op, nil, nil)
	at := func(h uint64) OpEnv { return env(h, testSender) }

	wantErr(t, "no record", CheckOp(p, nil, op, at(50)), ErrBucketNotGuarded)
	wantErr(t, "generation", CheckOp(p, &Guard{AlgID: alg, PublicKey: g.PublicKey, Generation: 4, Nonce: 8}, op, at(50)), ErrGuardMismatch)
	wantErr(t, "nonce", CheckOp(p, &Guard{AlgID: alg, PublicKey: g.PublicKey, Generation: 3, Nonce: 9}, op, at(50)), ErrGuardMismatch)
	if err := CheckOp(p, g, op, at(100)); err != nil {
		t.Fatalf("deadline height must be inclusive: %v", err)
	}
	wantErr(t, "deadline", CheckOp(p, g, op, at(101)), ErrGuardMismatch)
	maxed := *op
	maxed.Nonce = math.MaxUint64
	wantErr(t, "nonce overflow", CheckOp(p, &Guard{AlgID: alg, PublicKey: g.PublicKey, Generation: 3, Nonce: math.MaxUint64}, &maxed, at(50)), ErrGuardMismatch)
	clearMax := anchored(&GuardedBucketOp{BucketIndex: 7, Generation: math.MaxUint64, Nonce: 8, Deadline: 100, Op: OpTransfer, NewOwner: bob.Bytes(), ClearGuard: true}, 40)
	wantErr(t, "clear generation overflow", CheckOp(p, &Guard{AlgID: alg, PublicKey: g.PublicKey, Generation: math.MaxUint64, Nonce: 8}, clearMax, at(50)), ErrGuardMismatch)

	if err := verifyOp(p, testDomain, alice, g, op); err != nil {
		t.Fatal(err)
	}
	// The owner is bound at execution time: a transfer after signing voids the op.
	wantErr(t, "owner changed", verifyOp(p, testDomain, bob, g, op), ErrGuardSignature)
	wantErr(t, "chain", verifyOp(p, otherChainDomain, alice, g, op), ErrGuardSignature)
	wantErr(t, "genesis", verifyOp(p, otherGenesisDomain, alice, g, op), ErrGuardSignature)
	tampered := *op
	tampered.Recipient = alice.Bytes()
	wantErr(t, "recipient changed", verifyOp(p, testDomain, alice, g, &tampered), ErrGuardSignature)
	tampered = *op
	tampered.Sponsor = bob.Bytes()
	wantErr(t, "sponsor changed", verifyOp(p, testDomain, alice, g, &tampered), ErrGuardSignature)

	// A CALL signed under the ACCEPT context is not a CALL signature.
	wrongCtx := *op
	var err error
	if wrongCtx.Sig, err = k.Sign([]byte(AcceptContext), EncodeCall(testDomain, alice, op)); err != nil {
		t.Fatal(err)
	}
	wantErr(t, "CALL under ACCEPT context", verifyOp(p, testDomain, alice, g, &wrongCtx), ErrGuardSignature)

	// ACCEPT signed for the wrong resulting owner.
	rot := anchored(&GuardedBucketOp{BucketIndex: 7, Generation: 3, Nonce: 8, Deadline: 100, Op: OpRotate}, 40)
	signOp(t, k, alice, rot, newTestKey(t, 2), bob)
	wantErr(t, "accept owner", verifyOp(p, testDomain, alice, g, rot), ErrGuardSignature)

	// An ACCEPT is bound to one CALL: reusing it with another CALL that has
	// the same bucket, generation, owner and key fails, even when the current
	// key signs the new CALL.
	rot = anchored(&GuardedBucketOp{BucketIndex: 7, Generation: 3, Nonce: 8, Deadline: 100, Op: OpRotate}, 40)
	signOp(t, k, alice, rot, newTestKey(t, 2), alice)
	if err := verifyOp(p, testDomain, alice, g, rot); err != nil {
		t.Fatal(err)
	}
	other := *rot
	other.Deadline = 99
	if other.Sig, err = k.Sign([]byte(CallContext), EncodeCall(testDomain, alice, &other)); err != nil {
		t.Fatal(err)
	}
	wantErr(t, "accept reused", verifyOp(p, testDomain, alice, g, &other), ErrGuardSignature)

	// The anchor is signed: moving it to another block voids the CALL.
	tampered = *op
	anchored(&tampered, 41)
	wantErr(t, "anchor changed", verifyOp(p, testDomain, alice, g, &tampered), ErrGuardSignature)
}

// TestCheckOpAnchor covers IIP §9.2 step 4 for the anchor: heights first, the
// block hash only when they pass.
func TestCheckOpAnchor(t *testing.T) {
	p := ParamsV1()
	k := newTestKey(t, 1)
	g := &Guard{AlgID: alg, PublicKey: k.PublicKey().Bytes(), Generation: 3, Nonce: 8, BoundHeight: 10}
	op := func(anchor, deadline uint64) *GuardedBucketOp {
		return anchored(&GuardedBucketOp{BucketIndex: 7, Generation: 3, Nonce: 8, Deadline: deadline, Op: OpUnstake}, anchor)
	}
	var looked []uint64
	recording := func(h uint64) OpEnv {
		return OpEnv{Height: h, Sender: testSender, BlockHash: func(a uint64) ([32]byte, error) {
			looked = append(looked, a)
			return blockHash(a)
		}}
	}
	if err := CheckOp(p, g, op(40, 100), recording(41)); err != nil {
		t.Fatalf("op right after its anchor: %v", err)
	}
	if err := CheckOp(p, g, op(1000, 1000+p.MaxDeadlineWindow), recording(1000+p.MaxDeadlineWindow)); err != nil {
		t.Fatalf("op at the end of a full window: %v", err)
	}
	if len(looked) != 2 || looked[0] != 40 || looked[1] != 1000 {
		t.Fatalf("looked up %v", looked)
	}
	looked = nil
	wrong := op(40, 100)
	wrong.AnchorHash = bytes.Repeat([]byte{0x33}, AnchorHashLen)
	wantErr(t, "wrong anchor hash", CheckOp(p, g, wrong, recording(50)), ErrGuardMismatch)
	// Height failures do not look up any hash.
	n := len(looked)
	wantErr(t, "anchor above height", CheckOp(p, g, op(60, 100), recording(50)), ErrGuardMismatch)
	wantErr(t, "anchor is the current block", CheckOp(p, g, op(50, 100), recording(50)), ErrGuardMismatch)
	wantErr(t, "deadline passed", CheckOp(p, g, op(40, 100), recording(101)), ErrGuardMismatch)
	// Validate rejects this too; execution checks it again.
	wantErr(t, "window exceeded", CheckOp(p, g, op(40, 41+p.MaxDeadlineWindow), recording(50)), ErrGuardMismatch)
	if len(looked) != n {
		t.Fatalf("height failures looked up %v", looked[n:])
	}
	// A failed lookup is a storage error, not a guard verdict.
	storage := errors.New("storage")
	err := CheckOp(p, g, op(40, 100), OpEnv{Height: 50, Sender: testSender, BlockHash: func(uint64) ([32]byte, error) { return [32]byte{}, storage }})
	if !errors.Is(err, storage) || errors.Is(err, ErrGuardMismatch) {
		t.Fatalf("lookup failure: %v", err)
	}
}

// TestCheckOpSponsor covers IIP §9.2 step 3, which comes after the guard
// check and before the counters.
func TestCheckOpSponsor(t *testing.T) {
	p := ParamsV1()
	k := newTestKey(t, 1)
	g := &Guard{AlgID: alg, PublicKey: k.PublicKey().Bytes(), Generation: 3, Nonce: 8, BoundHeight: 10}
	sponsor, other := testAddr(t, 0x22), testAddr(t, 0x23)
	op := anchored(&GuardedBucketOp{BucketIndex: 7, Generation: 3, Nonce: 8, Deadline: 100, Op: OpUnstake, Sponsor: sponsor.Bytes()}, 40)
	if err := CheckOp(p, g, op, env(50, sponsor)); err != nil {
		t.Fatalf("bound sponsor: %v", err)
	}
	wantErr(t, "other sender", CheckOp(p, g, op, env(50, other)), ErrSponsorMismatch)
	wantErr(t, "no sender", CheckOp(p, g, op, OpEnv{Height: 50, BlockHash: blockHash}), ErrSponsorMismatch)
	wantErr(t, "unguarded first", CheckOp(p, nil, op, env(50, other)), ErrBucketNotGuarded)
	stale := *op
	stale.Nonce = 7
	wantErr(t, "sponsor before counters", CheckOp(p, g, &stale, env(50, other)), ErrSponsorMismatch)
	bearer := *op
	bearer.Sponsor = nil
	if err := CheckOp(p, g, &bearer, env(50, other)); err != nil {
		t.Fatalf("bearer op: %v", err)
	}
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

func TestGrant(t *testing.T) {
	p := ParamsV1()
	alice := testAddr(t, 0xa1)
	k := newTestKey(t, 1)
	g := &Guard{AlgID: alg, PublicKey: k.PublicKey().Bytes(), Generation: 3, Nonce: 8, BoundHeight: 10}
	op := anchored(&GuardedBucketOp{BucketIndex: 7, Generation: 3, Nonce: 8, Deadline: 100, Op: OpUnstake}, 40)
	signOp(t, k, alice, op, nil, nil)
	grant, err := VerifyOp(p, testDomain, alice, g, op)
	if err != nil {
		t.Fatal(err)
	}
	if grant.Op() != OpUnstake || !grant.Covers(7, g) {
		t.Fatalf("grant %+v does not cover its own op", grant)
	}
	for name, tc := range map[string]struct {
		index uint64
		cur   *Guard
	}{
		"other bucket":    {8, g},
		"next nonce":      {7, &Guard{AlgID: alg, PublicKey: g.PublicKey, Generation: 3, Nonce: 9}},
		"next generation": {7, &Guard{AlgID: alg, PublicKey: g.PublicKey, Generation: 4, Nonce: 8}},
		"tombstone":       {7, &Guard{Generation: 3, Nonce: 8}},
		"no guard record": {7, nil},
	} {
		if grant.Covers(tc.index, tc.cur) {
			t.Errorf("%s: grant covers it", name)
		}
	}
	if (*Grant)(nil).Covers(7, g) {
		t.Error("nil grant covers a write")
	}
	if grant, err := VerifyOp(p, otherChainDomain, alice, g, op); err == nil || grant != nil {
		t.Fatalf("failed verification returned grant %+v, err %v", grant, err)
	}
}
