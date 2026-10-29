package bucketguard

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"

	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto"
)

// SLH-DSA signing contexts, one per message type (IIP §7). FIPS 205 encodes
// the context with a length prefix, so it alone separates CALL from ACCEPT;
// the transcripts carry no further tag.
const (
	// CallContext is the context of CALL, signed by the current guard key.
	CallContext = "IoTeX-PQ-Bucket-Call-v1"
	// AcceptContext is the context of ACCEPT, signed by an incoming key.
	AcceptContext = "IoTeX-PQ-Bucket-Accept-v1"
)

// Domain identifies one chain instance in every transcript (IIP §7). It is a
// constant per network.
type Domain [32]byte

// NewDomain returns SHA-256("IoTeX-PQ-Bucket-Domain-v1" || uint32_be(chainID)
// || genesisHash). chainID is the native chain ID (1 on mainnet, 2 on
// testnet), not the EVM network ID; genesisHash is the network's genesis
// block hash (block.GenesisHash, equal to genesis.Genesis.Hash).
func NewDomain(chainID uint32, genesisHash [32]byte) Domain {
	b := append([]byte("IoTeX-PQ-Bucket-Domain-v1"), binary.BigEndian.AppendUint32(nil, chainID)...)
	return sha256.Sum256(append(b, genesisHash[:]...))
}

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
	// Sponsor, when set, is the only sender that may submit the op; empty
	// makes the op a bearer authorization. It is signed in CALL.
	Sponsor []byte
}

// installsKey reports whether op carries a new guard key.
func (m *GuardedBucketOp) installsKey() bool { return m.NewGuard != nil }

// changesKey reports whether op replaces or clears the guard key, which
// advances the generation.
func (m *GuardedBucketOp) changesKey() bool { return m.installsKey() || m.ClearGuard }
