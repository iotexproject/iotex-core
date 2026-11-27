package bucketguard

import (
	"bytes"
	"errors"
	"math/big"

	"golang.org/x/crypto/sha3"

	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto"
)

// Draft 5 block-count parameters. ParamsV2 supplies the default 100 IOTX
// minimum; explicit engine fixtures may override it.
const (
	UnbindDelayBlocks       uint64 = 3_110_400
	CancelCooldownBlocks    uint64 = 241_920
	ClassAVault             uint32 = 1
	ClassBStaking           uint32 = 2
	VaultBucketPrefix       byte   = 10
	VaultGuardPrefix        byte   = 11
	VaultMetaPrefix         byte   = 12
	VaultCallContext               = "IoTeX-PQ-Vault-Call-v1"
	VaultAcceptContext             = "IoTeX-PQ-Vault-Accept-v1"
	GuardCancelContext             = "IoTeX-PQ-Bucket-Cancel-v1"
	VaultOpWithdraw         uint32 = 1
	VaultOpRotateOwner      uint32 = 2
	VaultOpRotateGuard      uint32 = 3
	VaultOpRevokeSignatures uint32 = 4
)

var (
	ErrClassMismatch        = errors.New("bucket class mismatch")
	ErrVaultUnbinding       = errors.New("vault is unbinding")
	ErrVaultClosed          = errors.New("vault is closed")
	ErrAlreadyUnbinding     = errors.New("already unbinding")
	ErrUnbindNotElapsed     = errors.New("unbind delay not elapsed or no pending request")
	ErrUnbindElapsed        = errors.New("unbind delay elapsed")
	ErrCancelCooldown       = errors.New("cancel cooldown active")
	ErrStaleRequestId       = errors.New("stale request ID")
	ErrFutureRequestId      = errors.New("future request ID")
	ErrBelowMinBalance      = errors.New("below minimum vault balance")
	ErrUnauthorizedOperator = errors.New("unauthorized operator")
	ErrInsufficientBalance  = errors.New("insufficient balance")
)

// VaultPool returns the reserved native pool address, by value.
func VaultPool() [20]byte { return vaultHash160([]byte("vaultPool")) }

// PendingUnbind is shared by both immutable bucket classes.
type PendingUnbind struct{ RequestID, StartHeight, ExpiresAt uint64 }

// BucketGuardV2 is a new record, leaving the Draft 4 Guard schema untouched.
type BucketGuardV2 struct {
	AlgID          pqcrypto.AlgID
	PublicKey      []byte
	BucketClass    uint32
	Generation     uint64
	GuardNonce     uint64
	NextRequestID  uint64
	BoundHeight    uint64
	UnbindDelay    uint64
	CancelCooldown uint64
	Pending        *PendingUnbind
}

func (g *BucketGuardV2) clone() *BucketGuardV2 {
	if g == nil {
		return nil
	}
	c := *g
	c.PublicKey = bytes.Clone(g.PublicKey)
	if g.Pending != nil {
		p := *g.Pending
		c.Pending = &p
	}
	return &c
}

// KeyID uses the same SHA-256 key identifier as Draft 4.
func (g *BucketGuardV2) KeyID() [32]byte {
	return (&Guard{AlgID: g.AlgID, PublicKey: g.PublicKey}).KeyID()
}

// VaultBucket.Amount is canonical minimal unsigned big-endian Rau.
type VaultBucket struct {
	VaultIndex    uint64
	Owner, Amount []byte
}

func (v *VaultBucket) clone() *VaultBucket {
	if v == nil {
		return nil
	}
	return &VaultBucket{v.VaultIndex, bytes.Clone(v.Owner), bytes.Clone(v.Amount)}
}

// VaultCreateRequest binds PoP to the authenticated outer transaction nonce,
// never to the globally allocated vault index.
type VaultCreateRequest struct {
	Amount       *big.Int
	OwnerTxNonce uint64
	Guard        NewGuard
}

// VaultCall is the complete Class A authorization. Amount is a uint256;
// unused fields must be zero/empty. Signatures are excluded from transcripts.
type VaultCall struct {
	VaultIndex, Generation, GuardNonce, Deadline, AnchorHeight uint64
	AnchorHash                                                 []byte
	Op                                                         uint32
	Amount                                                     *big.Int
	Recipient, NewOwner                                        []byte
	NewGuard                                                   *NewGuard
	Sponsor, Sig                                               []byte
}

// GuardCancel deliberately has no guardNonce: owner actions cannot invalidate it.
type GuardCancel struct {
	BucketIndex, Generation, RequestID, Deadline, AnchorHeight uint64
	AnchorHash, Sponsor, Sig                                   []byte
}

// vaultHash160 matches native protocol address derivation (the last 20 bytes
// of legacy Keccak-256), also used by iotex-address/address_special.go.
func vaultHash160(input []byte) [20]byte {
	h := sha3.NewLegacyKeccak256()
	_, _ = h.Write(input)
	var out [20]byte
	copy(out[:], h.Sum(nil)[12:])
	return out
}
