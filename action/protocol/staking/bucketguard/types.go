package bucketguard

import "fmt"

// Consensus constants fixed by the IIP draft.
const (
	// AlgSLHDSA192s is the SLH-DSA-SHA2-192s algorithm identifier.
	AlgSLHDSA192s uint32 = 0x12

	// PubKeySize is the serialized SLH-DSA-SHA2-192s public key size.
	PubKeySize = 48
	// SigSize is the serialized SLH-DSA-SHA2-192s signature size.
	SigSize = 16224

	// MaxBindSize is the canonical size limit for BindBucketGuard.
	MaxBindSize = 17_000
	// MaxOpSize is the canonical size limit for GuardedBucketOp without a new guard.
	MaxOpSize = 17_000
	// MaxOpWithNewGuardSize is the canonical size limit with a new guard.
	MaxOpWithNewGuardSize = 34_000

	// IntrinsicGasBase is the fixed gas charged for both new actions.
	IntrinsicGasBase = 10_000
	// GasPerSizeByte is the canonical-encoding byte price.
	GasPerSizeByte = 16
	// GVerify is the proposed per-verification gas constant. It remains
	// normative only after native linux/amd64 calibration.
	GVerify = 30_000
)

// Guarded operation tags. Unknown tags are invalid.
const (
	OpUnstake         uint32 = 1
	OpWithdraw        uint32 = 2
	OpTransfer        uint32 = 3
	OpChangeCandidate uint32 = 4
	OpRestake         uint32 = 5
	OpRotate          uint32 = 6
)

// Existing staking validation bounds that guarded actions must reuse.
const (
	// MaxRestakeDurationDays mirrors staking._stakeDurationLimit.
	MaxRestakeDurationDays = 1050
	// MaxCandidateNameLen mirrors action.IsValidCandidateName.
	MaxCandidateNameLen = 12
)

// Transcript domain separators and context fixed by the IIP.
const (
	// Context is the SLH-DSA signing context.
	Context = "IOTEX-PQ-BUCKET-v1"
	// DomainCall prefixes GuardedBucketOp CALL transcripts.
	DomainCall = "IOTX-QSB-CALL"
	// DomainAccept prefixes incoming-key ACCEPT transcripts.
	DomainAccept = "IOTX-QSB-ACCEPT"
)

// Address is a 20-byte native account address. Integration converts to and
// from iotex-address at the action boundary.
type Address [20]byte

// AddressFromBytes copies a 20-byte address.
func AddressFromBytes(b []byte) (Address, error) {
	if len(b) != len(Address{}) {
		return Address{}, fmt.Errorf("address must be 20 bytes, got %d", len(b))
	}
	var a Address
	copy(a[:], b)
	return a, nil
}

// Bytes returns the raw 20 address bytes.
func (a Address) Bytes() []byte { return a[:] }

// Guard is the on-chain guard record for a bucket. Guarded=false with a
// non-zero generation is a tombstone.
type Guard struct {
	Guarded     bool
	AlgID       uint32
	PublicKey   []byte
	Generation  uint64
	Nonce       uint64
	BoundHeight uint64
}

// NewGuard is the incoming guard key carried by TRANSFER and ROTATE.
type NewGuard struct {
	AlgID     uint32
	PublicKey []byte
	AcceptSig []byte
}

// BindBucketGuard attaches a guard to a native bucket.
type BindBucketGuard struct {
	BucketIndex uint64
	AlgID       uint32
	PublicKey   []byte
	AcceptSig   []byte
}

// GuardedBucketOp performs one operation on a guarded bucket. The outer ECDSA
// sender is only a sponsor; the guard signature authorizes the bucket effect.
type GuardedBucketOp struct {
	BucketIndex         uint64
	Generation          uint64
	Nonce               uint64
	Deadline            uint64
	Op                  uint32
	Recipient           []byte
	NewOwner            []byte
	CandidateName       string
	ExpectedCandidateID []byte
	Duration            uint32
	AutoStake           bool
	NewGuard            *NewGuard
	ClearGuard          bool
	Sig                 []byte
}

// OpName returns the IIP operation name.
func OpName(op uint32) string {
	switch op {
	case OpUnstake:
		return "UNSTAKE"
	case OpWithdraw:
		return "WITHDRAW"
	case OpTransfer:
		return "TRANSFER"
	case OpChangeCandidate:
		return "CHANGE_CANDIDATE"
	case OpRestake:
		return "RESTAKE"
	case OpRotate:
		return "ROTATE"
	default:
		return fmt.Sprintf("OP_%d", op)
	}
}

// Active reports whether the draft rules are active at blockHeight. The caller
// supplies forkHeight from FeatureCtx; this package must not choose one. A zero
// fork height means the feature is not activated.
func Active(blockHeight, forkHeight uint64) bool {
	return forkHeight != 0 && blockHeight >= forkHeight
}
