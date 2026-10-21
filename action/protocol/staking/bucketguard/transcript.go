package bucketguard

import (
	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto"
)

// EncodeCall returns the byte-exact CALL transcript of IIP §7, signed by the
// current guard key. chainID is the native chain ID (BlockchainCtx.ChainID:
// 1 on mainnet, 2 on testnet), not the EVM network ID. owner is the bucket
// owner at execution time.
func EncodeCall(chainID uint32, owner address.Address, op *GuardedBucketOp) []byte {
	t := pqcrypto.NewTranscript(DomainCall).
		Uint32(chainID).
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
	return t.Bool(op.ClearGuard).Message()
}

// EncodeAccept returns the byte-exact ACCEPT transcript of IIP §7, signed by
// an incoming key: newGeneration is the generation the key will have,
// resultingOwner the bucket owner after the action, and deadline 0 for BIND or
// the CALL's deadline otherwise.
func EncodeAccept(chainID uint32, bucketIndex, newGeneration uint64, resultingOwner address.Address, alg pqcrypto.AlgID, publicKey []byte, deadline uint64) []byte {
	return pqcrypto.NewTranscript(DomainAccept).
		Uint32(chainID).
		Uint64(bucketIndex).
		Uint64(newGeneration).
		Fixed(resultingOwner.Bytes()).
		Uint32(uint32(alg)).
		Bytes(publicKey).
		Uint64(deadline).
		Message()
}
