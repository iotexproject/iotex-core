package bucketguard

import (
	"fmt"

	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto"
)

// Signing context and transcript domains fixed by the IIP (§2, §7).
const (
	// Context is the SLH-DSA signing context.
	Context = "IOTEX-PQ-BUCKET-v1"
	// DomainCall prefixes GuardedBucketOp CALL transcripts.
	DomainCall = "IOTX-QSB-CALL"
	// DomainAccept prefixes incoming-key ACCEPT transcripts.
	DomainAccept = "IOTX-QSB-ACCEPT"
)

// Op is a guarded operation tag (IIP §6). Unknown tags are invalid.
type Op uint32

// Guarded operations.
const (
	OpUnstake         Op = 1
	OpWithdraw        Op = 2
	OpTransfer        Op = 3
	OpChangeCandidate Op = 4
	OpRestake         Op = 5
	OpRotate          Op = 6
)

// String returns the IIP operation name.
func (o Op) String() string {
	switch o {
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
		return fmt.Sprintf("OP_%d", uint32(o))
	}
}

// NewGuard is the incoming guard key carried by TRANSFER and ROTATE.
type NewGuard struct {
	AlgID     pqcrypto.AlgID
	PublicKey []byte
	AcceptSig []byte
}

// BindBucketGuard attaches a guard to a native bucket. It is sent by the
// bucket owner; AcceptSig is the new key's proof of possession.
type BindBucketGuard struct {
	BucketIndex uint64
	AlgID       pqcrypto.AlgID
	PublicKey   []byte
	AcceptSig   []byte
}

// GuardedBucketOp performs one operation on a guarded bucket. The outer ECDSA
// sender is only a sponsor; Sig, by the current guard key, authorizes it.
// Address fields are raw 20-byte addresses; fields unused by Op must be empty.
type GuardedBucketOp struct {
	BucketIndex         uint64
	Generation          uint64
	Nonce               uint64
	Deadline            uint64
	Op                  Op
	Recipient           []byte // WITHDRAW
	NewOwner            []byte // TRANSFER
	CandidateName       string // CHANGE_CANDIDATE
	ExpectedCandidateID []byte // CHANGE_CANDIDATE
	Duration            uint32 // RESTAKE
	AutoStake           bool   // RESTAKE
	NewGuard            *NewGuard
	ClearGuard          bool // TRANSFER
	Sig                 []byte
}

// installsKey reports whether op carries a new guard key.
func (m *GuardedBucketOp) installsKey() bool { return m.NewGuard != nil }
