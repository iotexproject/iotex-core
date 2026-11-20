package bucketguard

import (
	"bytes"
	"crypto/sha512"
	"encoding/binary"
	"math"
	"math/big"

	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto"
)

// VaultEnv contains authenticated outer-envelope data and consensus block data.
// The caller verifies ECDSA and account nonce before entry, as a native handler
// would. The engine additionally rejects reuse of a successful creation PoP.
type VaultEnv struct {
	OpEnv
	OwnerTxNonce uint64
}

// VaultEngine is a serial, deterministic, standalone Draft 5 MVP. It never
// registers production handlers, changes fork parameters, or implements PoS.
// All exported reads return copies. Failed operations leave all state intact.
type VaultEngine struct {
	domain          Domain
	minBalance      *big.Int
	totalVaultCount uint64
	vaults          map[[9]byte]*VaultBucket
	guards          map[[9]byte]*BucketGuardV2
	accounts        map[string]*big.Int
	createNonces    map[string]bool
	staking         map[uint64]*VaultBucket
}

// NewVaultEngine requires an explicit provisional MIN_VAULT_BALANCE in Rau.
func NewVaultEngine(d Domain, minBalance *big.Int) (*VaultEngine, error) {
	if !validAmount(minBalance) || minBalance.Sign() == 0 {
		return nil, ErrMalformed
	}
	return &VaultEngine{domain: d, minBalance: new(big.Int).Set(minBalance),
		vaults: make(map[[9]byte]*VaultBucket), guards: make(map[[9]byte]*BucketGuardV2),
		accounts: make(map[string]*big.Int), createNonces: make(map[string]bool), staking: make(map[uint64]*VaultBucket)}, nil
}

// VaultStateKey is prefix || uint64_be(index), independently of host endianness.
func VaultStateKey(prefix byte, index uint64) [9]byte {
	var k [9]byte
	k[0] = prefix
	binary.BigEndian.PutUint64(k[1:], index)
	return k
}

func (e *VaultEngine) guard(class uint32, index uint64) *BucketGuardV2 {
	prefix := VaultGuardPrefix
	if class == ClassBStaking {
		prefix = 9
	}
	return e.guards[VaultStateKey(prefix, index)]
}

// Vault returns a copy, or nil after closure.
func (e *VaultEngine) Vault(index uint64) *VaultBucket {
	return e.vaults[VaultStateKey(VaultBucketPrefix, index)].clone()
}

// GuardV2 reads a class-specific namespace, including retained tombstones.
func (e *VaultEngine) GuardV2(class uint32, index uint64) *BucketGuardV2 {
	return e.guard(class, index).clone()
}

// TotalVaultCount is the monotonic allocator, initially zero.
func (e *VaultEngine) TotalVaultCount() uint64 { return e.totalVaultCount }

// Balance returns a copy of an account's liquid balance (including vaultPool).
func (e *VaultEngine) Balance(account []byte) *big.Int {
	if v := e.accounts[string(account)]; v != nil {
		return new(big.Int).Set(v)
	}
	return new(big.Int)
}

// Fund supplies genesis/test balances; it is not an action or a minting handler.
func (e *VaultEngine) Fund(account []byte, amount *big.Int) error {
	pool := VaultPool()
	if !validAddress(account) || bytes.Equal(account, pool[:]) || !validAmount(amount) {
		return ErrMalformed
	}
	e.credit(account, amount)
	return nil
}
func (e *VaultEngine) credit(a []byte, n *big.Int) {
	e.accounts[string(a)] = new(big.Int).Add(e.Balance(a), n)
}
func (e *VaultEngine) debit(a []byte, n *big.Int) {
	e.accounts[string(a)] = new(big.Int).Sub(e.Balance(a), n)
}
func validAmount(n *big.Int) bool { return n != nil && n.Sign() >= 0 && n.BitLen() <= 256 }
func validAddress(a []byte) bool  { return len(a) == 20 && !bytes.Equal(a, make([]byte, 20)) }
func sender(env OpEnv) []byte {
	if env.Sender == nil {
		return nil
	}
	return env.Sender.Bytes()
}
func ownerOnly(owner []byte, env OpEnv) error {
	if !bytes.Equal(owner, sender(env)) {
		return ErrUnauthorizedOperator
	}
	return nil
}
func verifyV2(alg pqcrypto.AlgID, key []byte, context string, msg, sig []byte) error {
	pk, err := pqcrypto.ParsePublicKey(alg, key)
	if err != nil || !pk.Verify([]byte(context), msg, sig) {
		return ErrGuardSignature
	}
	return nil
}
func (e *VaultEngine) activeVault(index uint64) (*VaultBucket, *BucketGuardV2, error) {
	v := e.vaults[VaultStateKey(VaultBucketPrefix, index)]
	g := e.guard(ClassAVault, index)
	if v == nil {
		return nil, nil, ErrVaultClosed
	}
	if g == nil || g.BucketClass != ClassAVault {
		return nil, nil, ErrClassMismatch
	}
	return v, g, nil
}

// VaultCreate atomically moves the initial deposit into the dedicated pool.
func (e *VaultEngine) VaultCreate(env VaultEnv, m *VaultCreateRequest) (uint64, error) {
	owner := sender(env.OpEnv)
	if m == nil || !validAddress(owner) || !validAmount(m.Amount) {
		return 0, ErrMalformed
	}
	if m.Amount.Cmp(e.minBalance) < 0 {
		return 0, ErrBelowMinBalance
	}
	nonceKey := string(append(bytes.Clone(owner), binary.BigEndian.AppendUint64(nil, m.OwnerTxNonce)...))
	if m.OwnerTxNonce != env.OwnerTxNonce || e.createNonces[nonceKey] || e.totalVaultCount == math.MaxUint64 {
		return 0, ErrGuardMismatch
	}
	if e.Balance(owner).Cmp(m.Amount) < 0 {
		return 0, ErrInsufficientBalance
	}
	msg := EncodeVaultAccept(e.domain, 0, m.OwnerTxNonce, 1, owner, UnbindDelayBlocks, m.Guard.AlgID, m.Guard.PublicKey, [64]byte{})
	if err := verifyV2(m.Guard.AlgID, m.Guard.PublicKey, VaultAcceptContext, msg, m.Guard.AcceptSig); err != nil {
		return 0, err
	}
	index := e.totalVaultCount
	e.vaults[VaultStateKey(VaultBucketPrefix, index)] = &VaultBucket{index, bytes.Clone(owner), m.Amount.Bytes()}
	e.guards[VaultStateKey(VaultGuardPrefix, index)] = &BucketGuardV2{AlgID: m.Guard.AlgID, PublicKey: bytes.Clone(m.Guard.PublicKey),
		BucketClass: ClassAVault, Generation: 1, NextRequestID: 1, BoundHeight: env.Height, UnbindDelay: UnbindDelayBlocks}
	e.totalVaultCount++
	e.createNonces[nonceKey] = true
	pool := VaultPool()
	e.debit(owner, m.Amount)
	e.credit(pool[:], m.Amount)
	return index, nil
}

// VaultAddDeposit is owner-only and never consumes a guard nonce.
func (e *VaultEngine) VaultAddDeposit(env OpEnv, index uint64, amount *big.Int) error {
	v, g, err := e.activeVault(index)
	if err != nil {
		return err
	}
	if err = ownerOnly(v.Owner, env); err != nil {
		return err
	}
	if g.Pending != nil {
		return ErrVaultUnbinding
	}
	if !validAmount(amount) || amount.Sign() == 0 {
		return ErrMalformed
	}
	n := new(big.Int).Add(new(big.Int).SetBytes(v.Amount), amount)
	if n.BitLen() > 256 {
		return ErrMalformed
	}
	if e.Balance(v.Owner).Cmp(amount) < 0 {
		return ErrInsufficientBalance
	}
	pool := VaultPool()
	e.debit(v.Owner, amount)
	e.credit(pool[:], amount)
	v.Amount = n.Bytes()
	return nil
}

// Validate rejects unused fields and non-canonical address/amount inputs.
func (m *VaultCall) Validate() error {
	if m == nil || !validAmount(m.Amount) || len(m.AnchorHash) != 32 || m.AnchorHeight == 0 ||
		m.Deadline < m.AnchorHeight || m.Deadline-m.AnchorHeight > ParamsV1().MaxDeadlineWindow ||
		len(m.Sig) != pqcrypto.AlgSLHDSA192s.SignatureSize() || (len(m.Sponsor) > 0 && !validAddress(m.Sponsor)) {
		return ErrMalformed
	}
	pool := VaultPool()
	switch m.Op {
	case VaultOpWithdraw:
		if m.Amount.Sign() == 0 || !validAddress(m.Recipient) || bytes.Equal(m.Recipient, pool[:]) || len(m.NewOwner) > 0 || m.NewGuard != nil {
			return ErrMalformed
		}
	case VaultOpRotateOwner:
		if m.Amount.Sign() != 0 || len(m.Recipient) > 0 || !validAddress(m.NewOwner) || forbiddenVaultOwner(m.NewOwner) || m.NewGuard != nil {
			return ErrMalformed
		}
	case VaultOpRotateGuard:
		if m.Amount.Sign() != 0 || len(m.Recipient) > 0 || len(m.NewOwner) > 0 || m.NewGuard == nil {
			return ErrMalformed
		}
		if m.NewGuard.AlgID != pqcrypto.AlgSLHDSA192s || len(m.NewGuard.PublicKey) != 48 || len(m.NewGuard.AcceptSig) != 16224 {
			return ErrMalformed
		}
	case VaultOpRevokeSignatures:
		if m.Amount.Sign() != 0 || len(m.Recipient) > 0 || len(m.NewOwner) > 0 || m.NewGuard != nil {
			return ErrMalformed
		}
	default:
		return ErrMalformed
	}
	return nil
}

func forbiddenVaultOwner(a []byte) bool {
	// §6's exact native protocol address list, extended by §13's vaultPool.
	for _, name := range []string{"account", "staking", "rewarding", "poll", "rolldpos", "smart_contract", "bucketPool", "vaultPool"} {
		h := vaultHash160([]byte(name))
		if bytes.Equal(a, h[:]) {
			return true
		}
	}
	return false
}

// checkWindow reuses Draft 4's anchor/deadline/sponsor rules, without tying
// cancel signatures to guardNonce or incrementing either replay counter.
func checkWindow(g *BucketGuardV2, generation, deadline, anchor uint64, anchorHash, sponsor []byte, env OpEnv) error {
	if len(anchorHash) != 32 || anchor == 0 || deadline < anchor || deadline-anchor > ParamsV1().MaxDeadlineWindow ||
		(len(sponsor) > 0 && !validAddress(sponsor)) {
		return ErrMalformed
	}
	if len(sponsor) > 0 && !bytes.Equal(sponsor, sender(env)) {
		return ErrSponsorMismatch
	}
	if generation != g.Generation || anchor >= env.Height || env.Height > deadline {
		return ErrGuardMismatch
	}
	if env.BlockHash == nil {
		return ErrGuardMismatch
	}
	h, err := env.BlockHash(anchor)
	if err != nil {
		return err
	}
	if !bytes.Equal(h[:], anchorHash) {
		return ErrGuardMismatch
	}
	return nil
}

// VaultExecute verifies every signature and precondition before any write.
// Named wrappers below keep the four action entry points explicit.
func (e *VaultEngine) VaultExecute(env OpEnv, m *VaultCall) error {
	if m == nil {
		return ErrMalformed
	}
	v, g, err := e.activeVault(m.VaultIndex)
	if err != nil {
		return err
	}
	if err = m.Validate(); err != nil {
		return err
	}
	if m.Op != VaultOpRevokeSignatures {
		if err = ownerOnly(v.Owner, env); err != nil {
			return err
		}
		if len(m.Sponsor) > 0 && !bytes.Equal(m.Sponsor, v.Owner) {
			return ErrUnauthorizedOperator
		}
	}
	if err = checkWindow(g, m.Generation, m.Deadline, m.AnchorHeight, m.AnchorHash, m.Sponsor, env); err != nil {
		return err
	}
	if m.GuardNonce != g.GuardNonce || g.GuardNonce == math.MaxUint64 {
		return ErrGuardMismatch
	}
	if m.Op == VaultOpRotateOwner || m.Op == VaultOpRotateGuard {
		if g.Pending != nil {
			return ErrVaultUnbinding
		}
		if bytes.Equal(m.NewOwner, v.Owner) {
			return ErrMalformed
		}
	}
	balance := new(big.Int).SetBytes(v.Amount)
	remaining := new(big.Int).Sub(balance, m.Amount)
	if m.Op == VaultOpWithdraw {
		if remaining.Sign() < 0 {
			return ErrInsufficientBalance
		}
		if remaining.Sign() > 0 && remaining.Cmp(e.minBalance) < 0 {
			return ErrBelowMinBalance
		}
	}
	changesGeneration := m.Op == VaultOpRotateOwner || m.Op == VaultOpRotateGuard || (m.Op == VaultOpWithdraw && remaining.Sign() == 0)
	if changesGeneration && g.Generation == math.MaxUint64 {
		return ErrGuardMismatch
	}
	call := EncodeVaultCall(e.domain, v.Owner, g.KeyID(), m)
	if err = verifyV2(g.AlgID, g.PublicKey, VaultCallContext, call, m.Sig); err != nil {
		return err
	}
	if m.Op == VaultOpRotateGuard {
		ng := m.NewGuard
		accept := EncodeVaultAccept(e.domain, m.VaultIndex, 0, g.Generation+1, v.Owner, g.UnbindDelay, ng.AlgID, ng.PublicKey, sha512.Sum512(call))
		if err = verifyV2(ng.AlgID, ng.PublicKey, VaultAcceptContext, accept, ng.AcceptSig); err != nil {
			return err
		}
	}
	g.GuardNonce++
	switch m.Op {
	case VaultOpWithdraw:
		pool := VaultPool()
		e.debit(pool[:], m.Amount)
		e.credit(m.Recipient, m.Amount)
		if remaining.Sign() == 0 {
			e.closeVault(m.VaultIndex, g, env.Height)
		} else {
			v.Amount = remaining.Bytes()
		}
	case VaultOpRotateOwner:
		v.Owner = bytes.Clone(m.NewOwner)
		g.Generation++
		g.BoundHeight = env.Height
	case VaultOpRotateGuard:
		g.AlgID = m.NewGuard.AlgID
		g.PublicKey = bytes.Clone(m.NewGuard.PublicKey)
		g.Generation++
		g.BoundHeight = env.Height
	}
	return nil
}

func (e *VaultEngine) executeKind(env OpEnv, m *VaultCall, kind uint32) error {
	if m == nil {
		return ErrMalformed
	}
	if _, _, err := e.activeVault(m.VaultIndex); err != nil {
		return err
	}
	if m.Op != kind {
		return ErrMalformed
	}
	return e.VaultExecute(env, m)
}

// VaultWithdraw executes op 1.
func (e *VaultEngine) VaultWithdraw(env OpEnv, m *VaultCall) error {
	return e.executeKind(env, m, VaultOpWithdraw)
}

// VaultRotateOwner executes op 2.
func (e *VaultEngine) VaultRotateOwner(env OpEnv, m *VaultCall) error {
	return e.executeKind(env, m, VaultOpRotateOwner)
}

// VaultRotateGuard executes op 3 with an incoming-key PoP.
func (e *VaultEngine) VaultRotateGuard(env OpEnv, m *VaultCall) error {
	return e.executeKind(env, m, VaultOpRotateGuard)
}

// VaultRevokeSignatures executes the only unilateral Class A guard action.
func (e *VaultEngine) VaultRevokeSignatures(env OpEnv, m *VaultCall) error {
	return e.executeKind(env, m, VaultOpRevokeSignatures)
}

func clearV2(g *BucketGuardV2, height uint64) {
	g.AlgID = 0
	g.PublicKey = nil
	g.Generation++
	g.Pending = nil
	g.BoundHeight = height
}
func (e *VaultEngine) closeVault(index uint64, g *BucketGuardV2, height uint64) {
	delete(e.vaults, VaultStateKey(VaultBucketPrefix, index))
	clearV2(g, height)
}
func initiate(g *BucketGuardV2, height uint64) error {
	if g.Pending != nil {
		return ErrAlreadyUnbinding
	}
	if g.NextRequestID == math.MaxUint64 || height > math.MaxUint64-g.UnbindDelay {
		return ErrGuardMismatch
	}
	g.Pending = &PendingUnbind{g.NextRequestID, height, height + g.UnbindDelay}
	g.NextRequestID++
	return nil
}

// VaultInitiateExit starts a new full mandatory delay, without consuming a nonce.
func (e *VaultEngine) VaultInitiateExit(env OpEnv, index uint64) error {
	v, g, err := e.activeVault(index)
	if err != nil {
		return err
	}
	if err = ownerOnly(v.Owner, env); err != nil {
		return err
	}
	return initiate(g, env.Height)
}

// VaultCancelExit is owner-only and excludes the exact expiry block.
func (e *VaultEngine) VaultCancelExit(env OpEnv, index uint64) error {
	v, g, err := e.activeVault(index)
	if err != nil {
		return err
	}
	if err = ownerOnly(v.Owner, env); err != nil {
		return err
	}
	if g.Pending == nil {
		return ErrUnbindNotElapsed
	}
	if env.Height >= g.Pending.ExpiresAt {
		return ErrUnbindElapsed
	}
	g.Pending = nil
	return nil
}

// VaultFinalizeExit is permissionless; payout always goes to the stored owner.
func (e *VaultEngine) VaultFinalizeExit(env OpEnv, index uint64) error {
	v, g, err := e.activeVault(index)
	if err != nil {
		return err
	}
	if g.Pending == nil || env.Height < g.Pending.ExpiresAt {
		return ErrUnbindNotElapsed
	}
	if g.Generation == math.MaxUint64 {
		return ErrGuardMismatch
	}
	amount := new(big.Int).SetBytes(v.Amount)
	pool := VaultPool()
	e.debit(pool[:], amount)
	e.credit(v.Owner, amount)
	e.closeVault(index, g, env.Height)
	return nil
}
