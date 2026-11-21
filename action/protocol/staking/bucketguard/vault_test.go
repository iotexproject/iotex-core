package bucketguard

import (
	"bytes"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/binary"
	"errors"
	"math"
	"math/big"
	"reflect"
	"sync"
	"testing"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto"
	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto/signer"
)

var vaultKeys struct {
	sync.Once
	keys [2]*signer.PrivateKey
	err  error
}

func testVaultKeys(t testing.TB) (*signer.PrivateKey, *signer.PrivateKey) {
	t.Helper()
	vaultKeys.Do(func() {
		for i := range vaultKeys.keys {
			vaultKeys.keys[i], vaultKeys.err = signer.NewKeyFromSeeds(pqcrypto.AlgSLHDSA192s, bytes.Repeat([]byte{byte(i + 1)}, 72))
			if vaultKeys.err != nil {
				return
			}
		}
	})
	if vaultKeys.err != nil {
		t.Fatal(vaultKeys.err)
	}
	return vaultKeys.keys[0], vaultKeys.keys[1]
}

// Reuse already generated offline authorizations across identical fixtures.
// Every engine call still performs real SLH-DSA verification. This avoids
// spending seconds signing the exact same setup PoP in each scenario.
var vaultSignatures = struct {
	sync.Mutex
	values map[[32]byte][]byte
}{values: make(map[[32]byte][]byte)}

func vaultSign(t testing.TB, k *signer.PrivateKey, ctx string, msg []byte) []byte {
	t.Helper()
	cacheKey := sha256.Sum256(bytes.Join([][]byte{k.PublicKey().Bytes(), []byte{byte(len(ctx))}, []byte(ctx), msg}, nil))
	vaultSignatures.Lock()
	defer vaultSignatures.Unlock()
	if sig := vaultSignatures.values[cacheKey]; sig != nil {
		return bytes.Clone(sig)
	}
	sig, err := k.Sign([]byte(ctx), msg)
	if err != nil {
		t.Fatal(err)
	}
	if len(k.PublicKey().Bytes()) != 48 || len(sig) != 16224 || !k.PublicKey().Verify([]byte(ctx), msg, sig) {
		t.Fatal("real SLH-DSA signing verification failed")
	}
	vaultSignatures.values[cacheKey] = bytes.Clone(sig)
	return sig
}
func vaultAddress(n byte) []byte { return bytes.Repeat([]byte{n}, 20) }
func vaultEnv(n byte, h uint64) OpEnv {
	a, _ := address.FromBytes(vaultAddress(n))
	return OpEnv{Height: h, Sender: a, BlockHash: func(uint64) ([32]byte, error) { return [32]byte{9}, nil }}
}
func vaultError(t testing.TB, err, want error) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Fatalf("got %v, want %v", err, want)
	}
}
func freshVaultEngine(t testing.TB) *VaultEngine {
	t.Helper()
	e, err := NewVaultEngine(NewDomain(1, [32]byte{8}), big.NewInt(100))
	vaultError(t, err, nil)
	vaultError(t, e.Fund(vaultAddress(1), big.NewInt(100_000)), nil)
	return e
}
func createRequest(t testing.TB, e *VaultEngine, nonce uint64, k *signer.PrivateKey) *VaultCreateRequest {
	t.Helper()
	m := &VaultCreateRequest{Amount: big.NewInt(1000), OwnerTxNonce: nonce, Guard: NewGuard{AlgID: pqcrypto.AlgSLHDSA192s, PublicKey: k.PublicKey().Bytes()}}
	m.Guard.AcceptSig = vaultSign(t, k, VaultAcceptContext, EncodeVaultAccept(e.domain, 0, nonce, 1, vaultAddress(1), UnbindDelayBlocks, m.Guard.AlgID, m.Guard.PublicKey, [64]byte{}))
	return m
}
func vaultFixture(t testing.TB) (*VaultEngine, *signer.PrivateKey, *signer.PrivateKey) {
	t.Helper()
	k, k2 := testVaultKeys(t)
	e := freshVaultEngine(t)
	m := createRequest(t, e, 7, k)
	idx, err := e.VaultCreate(VaultEnv{vaultEnv(1, 100), 7}, m)
	vaultError(t, err, nil)
	if idx != 0 {
		t.Fatal(idx)
	}
	return e, k, k2
}
func unsignedVaultCall(e *VaultEngine, index uint64, kind uint32, h uint64) *VaultCall {
	g := e.GuardV2(ClassAVault, index)
	return &VaultCall{VaultIndex: index, Generation: g.Generation, GuardNonce: g.GuardNonce, Deadline: h + 100,
		AnchorHeight: h - 1, AnchorHash: append([]byte{9}, make([]byte, 31)...), Op: kind, Amount: new(big.Int)}
}
func signVaultCall(t testing.TB, e *VaultEngine, k *signer.PrivateKey, m *VaultCall) {
	t.Helper()
	v := e.Vault(m.VaultIndex)
	g := e.GuardV2(ClassAVault, m.VaultIndex)
	m.Sig = vaultSign(t, k, VaultCallContext, EncodeVaultCall(e.domain, v.Owner, g.KeyID(), m))
}
func withdrawCall(t testing.TB, e *VaultEngine, k *signer.PrivateKey, n int64, h uint64) *VaultCall {
	m := unsignedVaultCall(e, 0, VaultOpWithdraw, h)
	m.Amount.SetInt64(n)
	m.Recipient = vaultAddress(3)
	signVaultCall(t, e, k, m)
	return m
}
func cloneVaultEngine(e *VaultEngine) *VaultEngine {
	c := *e
	c.minBalance = new(big.Int).Set(e.minBalance)
	c.vaults = make(map[[9]byte]*VaultBucket)
	c.guards = make(map[[9]byte]*BucketGuardV2)
	c.accounts = make(map[string]*big.Int)
	c.createNonces = make(map[string]bool)
	c.staking = make(map[uint64]*VaultBucket)
	for k, v := range e.vaults {
		c.vaults[k] = v.clone()
	}
	for k, v := range e.guards {
		c.guards[k] = v.clone()
	}
	for k, v := range e.accounts {
		c.accounts[k] = new(big.Int).Set(v)
	}
	for k, v := range e.createNonces {
		c.createNonces[k] = v
	}
	for k, v := range e.staking {
		c.staking[k] = v.clone()
	}
	return &c
}
func assertVaultUnchanged(t testing.TB, before, after *VaultEngine) {
	t.Helper()
	if !reflect.DeepEqual(before, after) {
		t.Fatal("failed action mutated engine state")
	}
}
func bindB(t testing.TB, e *VaultEngine, k *signer.PrivateKey, index, delay uint64) {
	t.Helper()
	if e.StakingBucket(index) == nil {
		vaultError(t, e.SeedStakingBucket(index, vaultAddress(1), big.NewInt(1000)), nil)
	}
	gen := uint64(1)
	if g := e.GuardV2(ClassBStaking, index); g != nil {
		gen = g.Generation + 1
	}
	ng := &NewGuard{AlgID: pqcrypto.AlgSLHDSA192s, PublicKey: k.PublicKey().Bytes()}
	ng.AcceptSig = vaultSign(t, k, AcceptContext, EncodeAcceptBV2(e.domain, index, gen, vaultAddress(1), delay, ng.AlgID, ng.PublicKey, [64]byte{}))
	vaultError(t, e.BindGuardBV2(vaultEnv(1, 100), index, delay, ng), nil)
}
func cancelCall(t testing.TB, e *VaultEngine, k *signer.PrivateKey, index, request, h uint64) *GuardCancel {
	t.Helper()
	m := &GuardCancel{BucketIndex: index, Generation: e.GuardV2(ClassBStaking, index).Generation, RequestID: request,
		Deadline: h + 100, AnchorHeight: h - 1, AnchorHash: append([]byte{9}, make([]byte, 31)...)}
	m.Sig = vaultSign(t, k, GuardCancelContext, EncodeGuardCancel(e.domain, m))
	return m
}

func TestVaultLifecycle(t *testing.T) {
	e, k, k2 := vaultFixture(t)
	pool := VaultPool()
	if e.Balance(pool[:]).Int64() != 1000 || e.TotalVaultCount() != 1 {
		t.Fatal("creation accounting")
	}
	m := withdrawCall(t, e, k, 200, 101)
	vaultError(t, e.VaultAddDeposit(vaultEnv(1, 101), 0, big.NewInt(500)), nil)
	vaultError(t, e.VaultWithdraw(vaultEnv(1, 101), m), nil)
	if new(big.Int).SetBytes(e.Vault(0).Amount).Int64() != 1300 || e.Balance(vaultAddress(3)).Int64() != 200 {
		t.Fatal("explicit payout changed with deposit")
	}
	rotate := unsignedVaultCall(e, 0, VaultOpRotateOwner, 102)
	rotate.NewOwner = vaultAddress(2)
	signVaultCall(t, e, k, rotate)
	vaultError(t, e.VaultRotateOwner(vaultEnv(1, 102), rotate), nil)
	vaultError(t, e.VaultAddDeposit(vaultEnv(1, 103), 0, big.NewInt(1)), ErrUnauthorizedOperator)
	rotate = unsignedVaultCall(e, 0, VaultOpRotateGuard, 103)
	rotate.NewGuard = &NewGuard{AlgID: pqcrypto.AlgSLHDSA192s, PublicKey: k2.PublicKey().Bytes()}
	call := EncodeVaultCall(e.domain, e.Vault(0).Owner, e.GuardV2(ClassAVault, 0).KeyID(), rotate)
	rotate.NewGuard.AcceptSig = vaultSign(t, k2, VaultAcceptContext, EncodeVaultAccept(e.domain, 0, 0, 3, vaultAddress(2), UnbindDelayBlocks, rotate.NewGuard.AlgID, rotate.NewGuard.PublicKey, sha512.Sum512(call)))
	signVaultCall(t, e, k, rotate)
	vaultError(t, e.VaultRotateGuard(vaultEnv(2, 103), rotate), nil)
	vaultError(t, e.VaultInitiateExit(vaultEnv(2, 104), 0), nil)
	pending := *e.GuardV2(ClassAVault, 0).Pending
	m = withdrawCall(t, e, k2, 200, 105)
	vaultError(t, e.VaultWithdraw(vaultEnv(2, 105), m), nil)
	if *e.GuardV2(ClassAVault, 0).Pending != pending {
		t.Fatal("partial withdraw changed exit")
	}
	revoke := unsignedVaultCall(e, 0, VaultOpRevokeSignatures, 106)
	signVaultCall(t, e, k2, revoke)
	vaultError(t, e.VaultRevokeSignatures(vaultEnv(9, 106), revoke), nil)
	if *e.GuardV2(ClassAVault, 0).Pending != pending {
		t.Fatal("revoke extended exit")
	}
	vaultError(t, e.VaultFinalizeExit(vaultEnv(9, pending.ExpiresAt-1), 0), ErrUnbindNotElapsed)
	vaultError(t, e.VaultFinalizeExit(vaultEnv(9, pending.ExpiresAt), 0), nil)
	g := e.GuardV2(ClassAVault, 0)
	if e.Vault(0) != nil || g.AlgID != 0 || len(g.PublicKey) != 0 || g.Generation != 4 || g.GuardNonce != 5 || g.NextRequestID != 2 || g.Pending != nil {
		t.Fatalf("bad tombstone: %+v", g)
	}
	if e.Balance(vaultAddress(2)).Int64() != 1100 || e.Balance(vaultAddress(9)).Sign() != 0 || e.Balance(pool[:]).Sign() != 0 {
		t.Fatal("final payout destination or pool")
	}
	for _, fn := range []func() error{
		func() error { return e.VaultWithdraw(vaultEnv(2, 107), m) }, func() error { return e.VaultRotateOwner(vaultEnv(2, 107), m) },
		func() error { return e.VaultRotateGuard(vaultEnv(2, 107), m) }, func() error { return e.VaultRevokeSignatures(vaultEnv(9, 107), revoke) },
		func() error { return e.VaultAddDeposit(vaultEnv(2, 107), 0, big.NewInt(100)) }, func() error { return e.VaultInitiateExit(vaultEnv(2, 107), 0) },
		func() error { return e.VaultCancelExit(vaultEnv(2, 107), 0) }, func() error { return e.VaultFinalizeExit(vaultEnv(2, 107), 0) },
	} {
		vaultError(t, fn(), ErrVaultClosed)
	}
	req := createRequest(t, e, 8, k)
	idx, err := e.VaultCreate(VaultEnv{vaultEnv(1, 200), 8}, req)
	vaultError(t, err, nil)
	if idx != 1 {
		t.Fatal("index reused")
	}
}

func TestVaultCreatePoPRaces(t *testing.T) {
	e, k, _ := vaultFixture(t)
	req := createRequest(t, e, 9, k)
	// Another vault is allocated after offline signing, without invalidating PoP.
	other := createRequest(t, e, 8, k)
	_, err := e.VaultCreate(VaultEnv{vaultEnv(1, 101), 8}, other)
	vaultError(t, err, nil)
	idx, err := e.VaultCreate(VaultEnv{vaultEnv(1, 102), 9}, req)
	vaultError(t, err, nil)
	if idx != 2 {
		t.Fatal(idx)
	}
	before := cloneVaultEngine(e)
	_, err = e.VaultCreate(VaultEnv{vaultEnv(1, 103), 9}, req)
	vaultError(t, err, ErrGuardMismatch)
	req.OwnerTxNonce = 10
	_, err = e.VaultCreate(VaultEnv{vaultEnv(1, 103), 10}, req)
	vaultError(t, err, ErrGuardSignature)
	req.OwnerTxNonce = 9
	_, err = e.VaultCreate(VaultEnv{vaultEnv(2, 103), 9}, req)
	vaultError(t, err, ErrInsufficientBalance)
	assertVaultUnchanged(t, before, e)
}

func TestVaultForbiddenAddresses(t *testing.T) {
	e, k, _ := vaultFixture(t)
	for _, name := range []string{"account", "staking", "rewarding", "poll", "rolldpos", "smart_contract", "bucketPool", "vaultPool"} {
		t.Run(name, func(t *testing.T) {
			recipient := vaultHash160([]byte(name))
			m := unsignedVaultCall(e, 0, VaultOpWithdraw, 101)
			m.Amount.SetInt64(100)
			m.Recipient = recipient[:]
			signVaultCall(t, e, k, m)
			vaultError(t, m.Validate(), ErrMalformed)
			before := cloneVaultEngine(e)
			vaultError(t, e.VaultWithdraw(vaultEnv(1, 101), m), ErrMalformed)
			assertVaultUnchanged(t, before, e)

			m.Op = VaultOpRotateOwner
			m.Amount = new(big.Int)
			m.NewOwner, m.Recipient = m.Recipient, nil
			vaultError(t, m.Validate(), ErrMalformed)
		})
	}
}

func TestVaultAdversarialWithdraw(t *testing.T) {
	base, k, _ := vaultFixture(t)
	good := withdrawCall(t, base, k, 100, 101)
	cases := []struct {
		name   string
		mutate func(*VaultCall)
		sender byte
		want   error
	}{
		{"guard cannot withdraw", func(*VaultCall) {}, 9, ErrUnauthorizedOperator},
		{"zero amount", func(m *VaultCall) { m.Amount.SetInt64(0) }, 1, ErrMalformed},
		{"negative amount", func(m *VaultCall) { m.Amount.SetInt64(-1) }, 1, ErrMalformed},
		{"uint256 overflow", func(m *VaultCall) { m.Amount.Lsh(big.NewInt(1), 256) }, 1, ErrMalformed},
		{"dust remainder", func(m *VaultCall) { m.Amount.SetInt64(901) }, 1, ErrBelowMinBalance},
		{"insufficient balance", func(m *VaultCall) { m.Amount.SetInt64(1001) }, 1, ErrInsufficientBalance},
		{"zero recipient", func(m *VaultCall) { m.Recipient = make([]byte, 20) }, 1, ErrMalformed},
		{"pool recipient", func(m *VaultCall) { p := VaultPool(); m.Recipient = p[:] }, 1, ErrMalformed},
		{"unused owner", func(m *VaultCall) { m.NewOwner = vaultAddress(2) }, 1, ErrMalformed},
		{"unknown op", func(m *VaultCall) { m.Op = 99 }, 1, ErrMalformed},
		{"wrong generation", func(m *VaultCall) { m.Generation++ }, 1, ErrGuardMismatch},
		{"wrong nonce", func(m *VaultCall) { m.GuardNonce++ }, 1, ErrGuardMismatch},
		{"wrong anchor hash", func(m *VaultCall) { m.AnchorHash[0]++ }, 1, ErrGuardMismatch},
		{"current anchor", func(m *VaultCall) { m.AnchorHeight = 101 }, 1, ErrGuardMismatch},
		{"future anchor", func(m *VaultCall) { m.AnchorHeight = 102 }, 1, ErrGuardMismatch},
		{"expired deadline", func(m *VaultCall) { m.Deadline = 100 }, 1, ErrGuardMismatch},
		{"excess deadline window", func(m *VaultCall) { m.Deadline = m.AnchorHeight + 241921 }, 1, ErrMalformed},
		{"missing anchor", func(m *VaultCall) { m.AnchorHeight = 0 }, 1, ErrMalformed},
		{"short anchor", func(m *VaultCall) { m.AnchorHash = m.AnchorHash[:31] }, 1, ErrMalformed},
		{"unauthorized sponsor", func(m *VaultCall) { m.Sponsor = vaultAddress(9) }, 1, ErrUnauthorizedOperator},
		{"invalid signature", func(m *VaultCall) { m.Sig[0] ^= 1 }, 1, ErrGuardSignature},
		{"short signature", func(m *VaultCall) { m.Sig = m.Sig[:100] }, 1, ErrMalformed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := cloneVaultEngine(base)
			m := *good
			m.Amount = new(big.Int).Set(good.Amount)
			m.Sig = bytes.Clone(good.Sig)
			m.AnchorHash = bytes.Clone(good.AnchorHash)
			tc.mutate(&m)
			before := cloneVaultEngine(e)
			vaultError(t, e.VaultWithdraw(vaultEnv(tc.sender, 101), &m), tc.want)
			assertVaultUnchanged(t, before, e)
		})
	}
	t.Run("exact minimum remainder", func(t *testing.T) {
		e := cloneVaultEngine(base)
		m := withdrawCall(t, e, k, 900, 101)
		vaultError(t, e.VaultWithdraw(vaultEnv(1, 101), m), nil)
	})
	t.Run("full withdrawal closes pending", func(t *testing.T) {
		e := cloneVaultEngine(base)
		vaultError(t, e.VaultInitiateExit(vaultEnv(1, 101), 0), nil)
		m := withdrawCall(t, e, k, 1000, 102)
		vaultError(t, e.VaultWithdraw(vaultEnv(1, 102), m), nil)
		g := e.GuardV2(ClassAVault, 0)
		if e.Vault(0) != nil || g.Pending != nil || g.Generation != 2 || g.GuardNonce != 1 {
			t.Fatal(g)
		}
		vaultError(t, e.VaultWithdraw(vaultEnv(1, 102), m), ErrVaultClosed)
	})
}

func TestVaultExitAndRevocation(t *testing.T) {
	e, k, k2 := vaultFixture(t)
	presigned := withdrawCall(t, e, k, 100, 101)
	vaultError(t, e.VaultInitiateExit(vaultEnv(9, 101), 0), ErrUnauthorizedOperator)
	vaultError(t, e.VaultInitiateExit(vaultEnv(1, 101), 0), nil)
	vaultError(t, e.VaultInitiateExit(vaultEnv(1, 102), 0), ErrAlreadyUnbinding)
	vaultError(t, e.VaultAddDeposit(vaultEnv(1, 102), 0, big.NewInt(100)), ErrVaultUnbinding)
	for _, kind := range []uint32{VaultOpRotateOwner, VaultOpRotateGuard} {
		m := unsignedVaultCall(e, 0, kind, 102)
		if kind == VaultOpRotateOwner {
			m.NewOwner = vaultAddress(2)
		} else {
			m.NewGuard = &NewGuard{AlgID: pqcrypto.AlgSLHDSA192s, PublicKey: k2.PublicKey().Bytes(), AcceptSig: make([]byte, 16224)}
		}
		signVaultCall(t, e, k, m)
		vaultError(t, e.VaultExecute(vaultEnv(1, 102), m), ErrVaultUnbinding)
		vaultError(t, e.VaultExecute(vaultEnv(9, 102), m), ErrUnauthorizedOperator)
	}
	revoke := unsignedVaultCall(e, 0, VaultOpRevokeSignatures, 102)
	signVaultCall(t, e, k, revoke)
	before := e.GuardV2(ClassAVault, 0)
	vaultError(t, e.VaultRevokeSignatures(vaultEnv(9, 102), revoke), nil)
	if !reflect.DeepEqual(e.GuardV2(ClassAVault, 0).Pending, before.Pending) || e.Balance(vaultAddress(9)).Sign() != 0 {
		t.Fatal("guard altered exit or moved funds")
	}
	vaultError(t, e.VaultWithdraw(vaultEnv(1, 102), presigned), ErrGuardMismatch)
	expires := before.Pending.ExpiresAt
	vaultError(t, e.VaultCancelExit(vaultEnv(9, expires-1), 0), ErrUnauthorizedOperator)
	vaultError(t, e.VaultCancelExit(vaultEnv(1, expires-1), 0), nil)
	vaultError(t, e.VaultInitiateExit(vaultEnv(1, expires), 0), nil)
	g := e.GuardV2(ClassAVault, 0)
	if g.Pending.RequestID != 2 || g.Pending.ExpiresAt != expires+UnbindDelayBlocks || g.GuardNonce != 1 {
		t.Fatal(g)
	}
	vaultError(t, e.VaultCancelExit(vaultEnv(1, g.Pending.ExpiresAt), 0), ErrUnbindElapsed)
	vaultError(t, e.VaultFinalizeExit(vaultEnv(9, g.Pending.ExpiresAt), 0), nil)
}

func TestVaultRotationPoPAndOwnerRules(t *testing.T) {
	e, k, k2 := vaultFixture(t)
	for _, a := range [][]byte{vaultAddress(1), make([]byte, 20), vaultAddress(0), func() []byte { p := VaultPool(); return p[:] }(), func() []byte { p := vaultHash160([]byte("bucketPool")); return p[:] }()} {
		m := unsignedVaultCall(e, 0, VaultOpRotateOwner, 101)
		m.NewOwner = a
		signVaultCall(t, e, k, m)
		vaultError(t, e.VaultRotateOwner(vaultEnv(1, 101), m), ErrMalformed)
	}
	// Use actual nonzero index to catch a rotation accidentally using create's index=0.
	req := createRequest(t, e, 8, k)
	idx, err := e.VaultCreate(VaultEnv{vaultEnv(1, 101), 8}, req)
	vaultError(t, err, nil)
	m := unsignedVaultCall(e, idx, VaultOpRotateGuard, 102)
	m.NewGuard = &NewGuard{AlgID: pqcrypto.AlgSLHDSA192s, PublicKey: k2.PublicKey().Bytes()}
	call := EncodeVaultCall(e.domain, e.Vault(idx).Owner, e.GuardV2(ClassAVault, idx).KeyID(), m)
	wrong := EncodeVaultAccept(e.domain, 0, 0, 2, vaultAddress(1), UnbindDelayBlocks, m.NewGuard.AlgID, m.NewGuard.PublicKey, sha512.Sum512(call))
	m.NewGuard.AcceptSig = vaultSign(t, k2, VaultAcceptContext, wrong)
	signVaultCall(t, e, k, m)
	before := cloneVaultEngine(e)
	vaultError(t, e.VaultRotateGuard(vaultEnv(1, 102), m), ErrGuardSignature)
	assertVaultUnchanged(t, before, e)
	m.NewGuard.AcceptSig = vaultSign(t, k2, VaultAcceptContext, EncodeVaultAccept(e.domain, idx, 0, 2, vaultAddress(1), UnbindDelayBlocks, m.NewGuard.AlgID, m.NewGuard.PublicKey, sha512.Sum512(call)))
	vaultError(t, e.VaultRotateGuard(vaultEnv(1, 102), m), nil)
}

func TestGuardUnbindCancelFiveBranches(t *testing.T) {
	e, k, _ := vaultFixture(t)
	bindB(t, e, k, 0, UnbindDelayBlocks)
	vaultError(t, e.GuardUnbindInitiate(vaultEnv(9, 101), 0), ErrUnauthorizedOperator)
	vaultError(t, e.GuardUnbindInitiate(vaultEnv(1, 101), 0), nil)
	vaultError(t, e.GuardUnbindInitiate(vaultEnv(1, 102), 0), ErrAlreadyUnbinding)
	future := cancelCall(t, e, k, 0, 2, 102)
	before := cloneVaultEngine(e)
	vaultError(t, e.GuardUnbindCancel(vaultEnv(9, 102), future), ErrFutureRequestId)
	assertVaultUnchanged(t, before, e)
	expires := e.GuardV2(ClassBStaking, 0).Pending.ExpiresAt
	// Sign close to expiration so anchor-window validation does not mask boundary semantics.
	cancel := cancelCall(t, e, k, 0, 1, expires-1)
	expired := cloneVaultEngine(e)
	vaultError(t, expired.GuardUnbindCancel(vaultEnv(9, expires), cancel), ErrUnbindElapsed)
	vaultError(t, e.GuardUnbindCancel(vaultEnv(9, expires-1), cancel), nil)
	g := e.GuardV2(ClassBStaking, 0)
	if g.GuardNonce != 0 || g.CancelCooldown != expires-1+241920 || g.Pending != nil {
		t.Fatal(g)
	}
	before = cloneVaultEngine(e)
	vaultError(t, e.GuardUnbindCancel(vaultEnv(8, expires), cancel), nil)
	assertVaultUnchanged(t, before, e)
	vaultError(t, e.GuardUnbindInitiate(vaultEnv(1, g.CancelCooldown-1), 0), ErrCancelCooldown)
	vaultError(t, e.GuardUnbindInitiate(vaultEnv(1, g.CancelCooldown), 0), nil)
	stale := cancelCall(t, e, k, 0, 1, g.CancelCooldown+1)
	before = cloneVaultEngine(e)
	vaultError(t, e.GuardUnbindCancel(vaultEnv(9, g.CancelCooldown+1), stale), ErrStaleRequestId)
	assertVaultUnchanged(t, before, e)
	expires = e.GuardV2(ClassBStaking, 0).Pending.ExpiresAt
	vaultError(t, e.GuardUnbindFinalize(vaultEnv(9, expires-1), 0), ErrUnbindNotElapsed)
	vaultError(t, e.GuardUnbindFinalize(vaultEnv(9, expires), 0), nil)
	tomb := e.GuardV2(ClassBStaking, 0)
	if tomb.Generation != 2 || tomb.GuardNonce != 0 || tomb.NextRequestID != 3 || tomb.CancelCooldown != g.CancelCooldown || tomb.Pending != nil || len(tomb.PublicKey) != 0 {
		t.Fatal(tomb)
	}
	if new(big.Int).SetBytes(e.StakingBucket(0).Amount).Int64() != 1000 {
		t.Fatal("finalize moved PoS principal")
	}
	bindB(t, e, k, 0, 0)
	rebound := e.GuardV2(ClassBStaking, 0)
	if rebound.Generation != 3 || rebound.NextRequestID != 3 || rebound.CancelCooldown != tomb.CancelCooldown || rebound.UnbindDelay != 0 {
		t.Fatal(rebound)
	}
	vaultError(t, e.GuardUnbindInitiate(vaultEnv(1, expires+1), 0), ErrBucketGuarded)
	// Both index zero records coexist, with immutable distinct classes.
	if e.GuardV2(ClassAVault, 0).BucketClass != ClassAVault || rebound.BucketClass != ClassBStaking {
		t.Fatal("namespace collision")
	}
}

func TestGuardNonceIsolationAndCompound(t *testing.T) {
	e, k, _ := vaultFixture(t)
	bindB(t, e, k, 5, UnbindDelayBlocks)
	vaultError(t, e.GuardUnbindInitiate(vaultEnv(1, 101), 5), nil)
	cancel := cancelCall(t, e, k, 5, 1, 102)
	for i := 0; i < 100; i++ {
		ok, err := e.StakingDeposit(vaultEnv(1, 102), 5, big.NewInt(1), false)
		vaultError(t, err, nil)
		if !ok {
			t.Fatal("owner deposit skipped")
		}
		ok, err = e.StakingDeposit(vaultEnv(9, 102), 5, big.NewInt(1), true)
		vaultError(t, err, nil)
		if ok {
			t.Fatal("pending compound must stay claimable")
		}
	}
	if e.GuardV2(ClassBStaking, 5).GuardNonce != 0 || new(big.Int).SetBytes(e.StakingBucket(5).Amount).Int64() != 1100 {
		t.Fatal("nonce starvation or compound isolation failure")
	}
	vaultError(t, e.GuardUnbindCancel(vaultEnv(9, 103), cancel), nil)
	ok, err := e.StakingDeposit(vaultEnv(9, 104), 5, big.NewInt(10), true)
	vaultError(t, err, nil)
	if !ok || e.GuardV2(ClassBStaking, 5).GuardNonce != 0 {
		t.Fatal("compound consumed nonce")
	}
}

func TestVaultCrossClassAndDomainReplay(t *testing.T) {
	e, k, _ := vaultFixture(t)
	bindB(t, e, k, 0, UnbindDelayBlocks)
	m := withdrawCall(t, e, k, 100, 101)
	transcript := EncodeVaultCall(e.domain, e.Vault(0).Owner, e.GuardV2(ClassAVault, 0).KeyID(), m)
	for _, ctx := range []string{CallContext, AcceptContext, VaultAcceptContext, GuardCancelContext} {
		if k.PublicKey().Verify([]byte(ctx), transcript, m.Sig) {
			t.Fatal("FIPS context confusion:", ctx)
		}
	}
	m.Sig = vaultSign(t, k, CallContext, transcript)
	vaultError(t, e.VaultWithdraw(vaultEnv(1, 101), m), ErrGuardSignature)
	signVaultCall(t, e, k, m)
	other := cloneVaultEngine(e)
	other.domain = NewDomain(2, [32]byte{8})
	vaultError(t, other.VaultWithdraw(vaultEnv(1, 101), m), ErrGuardSignature)
	// Class A ACCEPT cannot authorize a Class B bind, even under the B context.
	vaultError(t, e.SeedStakingBucket(1, vaultAddress(1), big.NewInt(1000)), nil)
	req := createRequest(t, e, 8, k)
	ng := req.Guard
	vaultError(t, e.BindGuardBV2(vaultEnv(1, 101), 1, UnbindDelayBlocks, &ng), ErrGuardSignature)
	ng.AcceptSig = vaultSign(t, k, AcceptContext, EncodeVaultAccept(e.domain, 1, 0, 1, vaultAddress(1), UnbindDelayBlocks, ng.AlgID, ng.PublicKey, [64]byte{}))
	vaultError(t, e.BindGuardBV2(vaultEnv(1, 101), 1, UnbindDelayBlocks, &ng), ErrGuardSignature)
}

func TestVaultStorageAndCounterOverflow(t *testing.T) {
	e, k, _ := vaultFixture(t)
	key := VaultStateKey(VaultBucketPrefix, 0)
	if !bytes.Equal(key[:], []byte{10, 0, 0, 0, 0, 0, 0, 0, 0}) || binary.BigEndian.Uint64(e.State([]byte{12})) != 1 {
		t.Fatal("state layout")
	}
	v := e.Vault(0)
	if v.Amount[0] == 0 {
		t.Fatal("nonminimal amount")
	}
	v.Amount[0] = 0
	if _, err := v.Serialize(); err == nil {
		t.Fatal("accepted nonminimal stored amount")
	}
	g := e.GuardV2(ClassAVault, 0)
	g.PublicKey[0] ^= 1
	if reflect.DeepEqual(g, e.GuardV2(ClassAVault, 0)) {
		t.Fatal("read aliases state")
	}
	if !reflect.DeepEqual(e.StateEntries(), cloneVaultEngine(e).StateEntries()) {
		t.Fatal("nondeterministic storage")
	}
	e.guard(ClassAVault, 0).GuardNonce = math.MaxUint64
	m := withdrawCall(t, e, k, 100, 101)
	vaultError(t, e.VaultWithdraw(vaultEnv(1, 101), m), ErrGuardMismatch)
	e.guard(ClassAVault, 0).GuardNonce = 0
	e.guard(ClassAVault, 0).NextRequestID = math.MaxUint64
	vaultError(t, e.VaultInitiateExit(vaultEnv(1, 101), 0), ErrGuardMismatch)
}

func TestVaultTranscriptEncoding(t *testing.T) {
	// Independent encoding oracle: do not use pqcrypto.Transcript here.
	u32 := func(n uint32) []byte { return binary.BigEndian.AppendUint32(nil, n) }
	u64 := func(n uint64) []byte { return binary.BigEndian.AppendUint64(nil, n) }
	blob := func(b []byte) []byte { return append(u32(uint32(len(b))), b...) }
	join := func(parts ...[]byte) []byte { return bytes.Join(parts, nil) }
	d := Domain{1}
	owner := vaultAddress(2)
	key := bytes.Repeat([]byte{3}, 48)
	keyID := [32]byte{4}
	h := [64]byte{5}
	m := &VaultCall{VaultIndex: 6, Generation: 7, GuardNonce: 8, Deadline: 10, AnchorHeight: 9,
		AnchorHash: bytes.Repeat([]byte{11}, 32), Op: VaultOpRotateGuard, Amount: new(big.Int), Sponsor: owner,
		NewGuard: &NewGuard{AlgID: pqcrypto.AlgSLHDSA192s, PublicKey: key}}
	want := join(d[:], u32(1), u64(6), owner, keyID[:], u64(7), u64(8), u64(10), u64(9), m.AnchorHash, u32(3), make([]byte, 32), u32(0), u32(0), []byte{1}, u32(0x12), blob(key), blob(owner))
	if !bytes.Equal(want, EncodeVaultCall(d, owner, keyID, m)) {
		t.Fatal("VAULT_CALL byte order")
	}
	m.Op = VaultOpWithdraw
	m.NewGuard = nil
	m.Amount.SetInt64(0x102)
	m.Recipient = vaultAddress(12)
	m.Sponsor = nil
	amount := make([]byte, 32)
	amount[30] = 1
	amount[31] = 2
	want = join(d[:], u32(1), u64(6), owner, keyID[:], u64(7), u64(8), u64(10), u64(9), m.AnchorHash, u32(1), amount, blob(m.Recipient), u32(0), []byte{0}, u32(0))
	if !bytes.Equal(want, EncodeVaultCall(d, owner, keyID, m)) {
		t.Fatal("VAULT_CALL amount or absent key")
	}
	want = join(d[:], u32(1), u64(6), u64(13), u64(7), owner, u64(UnbindDelayBlocks), u32(0x12), blob(key), h[:])
	if !bytes.Equal(want, EncodeVaultAccept(d, 6, 13, 7, owner, UnbindDelayBlocks, pqcrypto.AlgSLHDSA192s, key, h)) {
		t.Fatal("VAULT_ACCEPT byte order")
	}
	want = join(d[:], u32(2), u64(6), u64(7), owner, u64(UnbindDelayBlocks), u32(0x12), blob(key), h[:])
	if !bytes.Equal(want, EncodeAcceptBV2(d, 6, 7, owner, UnbindDelayBlocks, pqcrypto.AlgSLHDSA192s, key, h)) {
		t.Fatal("ACCEPT_B_V2 byte order")
	}
	c := &GuardCancel{BucketIndex: 6, Generation: 7, RequestID: 8, Deadline: 10, AnchorHeight: 9, AnchorHash: m.AnchorHash, Sponsor: owner}
	want = join(d[:], u32(2), u64(6), u64(7), u64(8), u64(10), u64(9), m.AnchorHash, blob(owner))
	if !bytes.Equal(want, EncodeGuardCancel(d, c)) {
		t.Fatal("GUARD_CANCEL byte order")
	}
	if len(VaultCallContext) != 22 || len(VaultAcceptContext) != 24 || len(GuardCancelContext) != 25 {
		t.Fatal("context bytes")
	}
	if vaultHash160([]byte("staking")) != [20]byte(address.StakingProtocolAddrHash) {
		t.Fatal("native pool hash derivation")
	}
}

func TestVaultCreateMinimumAndMalformed(t *testing.T) {
	k, _ := testVaultKeys(t)
	e := freshVaultEngine(t)
	req := createRequest(t, e, 7, k)
	for _, n := range []*big.Int{nil, new(big.Int), big.NewInt(-1)} {
		if _, err := NewVaultEngine(e.domain, n); !errors.Is(err, ErrMalformed) {
			t.Fatal("accepted invalid minimum")
		}
	}
	req.Amount = big.NewInt(99)
	_, err := e.VaultCreate(VaultEnv{vaultEnv(1, 100), 7}, req)
	vaultError(t, err, ErrBelowMinBalance)
	req.Amount = big.NewInt(100)
	idx, err := e.VaultCreate(VaultEnv{vaultEnv(1, 100), 7}, req)
	vaultError(t, err, nil)
	if idx != 0 || new(big.Int).SetBytes(e.Vault(0).Amount).Int64() != 100 {
		t.Fatal("minimum deposit boundary")
	}
	req.OwnerTxNonce = 8
	req.Amount = big.NewInt(100)
	_, err = e.VaultCreate(VaultEnv{vaultEnv(1, 100), 9}, req)
	vaultError(t, err, ErrGuardMismatch)
	req.Guard.AlgID = 1
	_, err = e.VaultCreate(VaultEnv{vaultEnv(1, 100), 8}, req)
	vaultError(t, err, ErrGuardSignature)
}

func TestGuardCancelAuthentication(t *testing.T) {
	e, k, _ := vaultFixture(t)
	bindB(t, e, k, 0, UnbindDelayBlocks)
	vaultError(t, e.GuardUnbindInitiate(vaultEnv(1, 101), 0), nil)
	good := cancelCall(t, e, k, 0, 1, 102)
	for _, tc := range []struct {
		name   string
		mutate func(*GuardCancel)
		want   error
	}{
		{"request bound", func(m *GuardCancel) { m.RequestID = 2 }, ErrGuardSignature},
		{"generation", func(m *GuardCancel) { m.Generation++ }, ErrGuardMismatch},
		{"anchor", func(m *GuardCancel) { m.AnchorHash = make([]byte, 32) }, ErrGuardMismatch},
		{"deadline", func(m *GuardCancel) { m.Deadline = 101 }, ErrGuardMismatch},
		{"sponsor", func(m *GuardCancel) { m.Sponsor = vaultAddress(8) }, ErrSponsorMismatch},
		{"signature", func(m *GuardCancel) { m.Sig = make([]byte, 16224) }, ErrGuardSignature},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := *good
			tc.mutate(&m)
			before := cloneVaultEngine(e)
			vaultError(t, e.GuardUnbindCancel(vaultEnv(9, 102), &m), tc.want)
			assertVaultUnchanged(t, before, e)
		})
	}
	// Changing guardNonce alone cannot invalidate a pre-signed cancel.
	e.guard(ClassBStaking, 0).GuardNonce = 37
	vaultError(t, e.GuardUnbindCancel(vaultEnv(9, 102), good), nil)
	if e.GuardV2(ClassBStaking, 0).GuardNonce != 37 {
		t.Fatal("cancel consumed guardNonce")
	}
}

func TestClassBGuardedPendingWriteSets(t *testing.T) {
	e, k, k2 := vaultFixture(t)
	bindB(t, e, k, 0, UnbindDelayBlocks)
	vaultError(t, e.GuardUnbindInitiate(vaultEnv(1, 101), 0), nil)
	for _, kind := range []Op{OpUnstake, OpWithdraw, OpTransfer, OpChangeCandidate, OpRestake, OpRotate} {
		t.Run(kind.String(), func(t *testing.T) {
			iteration := cloneVaultEngine(e)
			g := iteration.GuardV2(ClassBStaking, 0)
			m := &GuardedBucketOp{BucketIndex: 0, Generation: g.Generation, Nonce: g.GuardNonce, Deadline: 200, AnchorHeight: 100, AnchorHash: append([]byte{9}, make([]byte, 31)...), Op: kind}
			switch kind {
			case OpWithdraw:
				m.Recipient = vaultAddress(3)
			case OpTransfer:
				m.NewOwner = vaultAddress(2)
				m.ClearGuard = true
			case OpChangeCandidate:
				m.CandidateName = "candidate"
				m.ExpectedCandidateID = vaultAddress(3)
			case OpRotate:
				m.NewGuard = &NewGuard{AlgID: pqcrypto.AlgSLHDSA192s, PublicKey: k2.PublicKey().Bytes()}
			}
			owner, _ := address.FromBytes(vaultAddress(1))
			call := EncodeCall(iteration.domain, owner, m)
			if m.NewGuard != nil {
				m.NewGuard.AcceptSig = vaultSign(t, k2, AcceptContext, EncodeAcceptBV2(iteration.domain, 0, 2, vaultAddress(1), g.UnbindDelay, m.NewGuard.AlgID, m.NewGuard.PublicKey, sha512.Sum512(call)))
			}
			m.Sig = vaultSign(t, k, CallContext, call)
			nativeWait := errors.New("native PoS waiting period")
			before := cloneVaultEngine(iteration)
			vaultError(t, iteration.StakingGuardedOp(vaultEnv(9, 102), m, func() error { return nativeWait }), nativeWait)
			assertVaultUnchanged(t, before, iteration)
			vaultError(t, iteration.StakingGuardedOp(vaultEnv(9, 102), m, func() error { return nil }), nil)
			next := iteration.GuardV2(ClassBStaking, 0)
			clear := kind == OpWithdraw || kind == OpTransfer || kind == OpRotate
			if (next.Pending == nil) != clear || next.GuardNonce != 1 {
				t.Fatal("incorrect Class B write-set", next)
			}
		})
	}
}
