package bucketguard

import (
	"crypto/sha256"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto"
)

// EncodeCall returns the byte-exact CALL transcript of IIP §7, signed by the
// current guard key under CallContext. owner is the bucket owner at execution
// time. CALL covers the incoming key of op.NewGuard but not its ACCEPT
// signature, so it can be built before the incoming key signs.
func EncodeCall(d Domain, owner address.Address, op *GuardedBucketOp) []byte {
	t := pqcrypto.NewTranscript().
		Fixed(d[:]).
		Uint64(op.BucketIndex).
		Fixed(owner.Bytes()).
		Uint64(op.Generation).
		Uint64(op.Nonce).
		Uint64(op.Deadline).
		Uint32(uint32(op.Op)).
		Bytes(op.Recipient).
		Bytes(op.NewOwner).
		Bytes([]byte(op.CandidateName)).
		Bytes(op.ExpectedCandidateID).
		Uint32(op.Duration).
		Bool(op.AutoStake).
		Bool(op.NewGuard != nil)
	if op.NewGuard != nil {
		t.Uint32(uint32(op.NewGuard.AlgID)).Bytes(op.NewGuard.PublicKey)
	}
	return t.Bool(op.ClearGuard).Bytes(op.Sponsor).Message()
}

// CallHash returns SHA-256 of the CALL transcript, the operation an incoming
// key accepts in ROTATE and TRANSFER with a new guard.
func CallHash(d Domain, owner address.Address, op *GuardedBucketOp) [32]byte {
	return sha256.Sum256(EncodeCall(d, owner, op))
}

// EncodeAccept returns the byte-exact ACCEPT transcript of IIP §7, signed by
// an incoming key under AcceptContext: newGeneration is the generation the
// key will have, resultingOwner the bucket owner after the action, and
// callHash zero for BIND or CallHash of the operation otherwise.
func EncodeAccept(d Domain, bucketIndex, newGeneration uint64, resultingOwner address.Address, alg pqcrypto.AlgID, publicKey []byte, callHash [32]byte) []byte {
	return pqcrypto.NewTranscript().
		Fixed(d[:]).
		Uint64(bucketIndex).
		Uint64(newGeneration).
		Fixed(resultingOwner.Bytes()).
		Uint32(uint32(alg)).
		Bytes(publicKey).
		Fixed(callHash[:]).
		Message()
}
