package bucketguard

import "github.com/iotexproject/iotex-core/v2/pkg/pqcrypto"

// EncodeVaultCall implements §13.5. Call Validate before encoding untrusted
// input; in particular Amount must fit uint256, and addresses have fixed widths.
func EncodeVaultCall(d Domain, owner []byte, keyID [32]byte, m *VaultCall) []byte {
	var amount [32]byte
	if m.Amount != nil {
		m.Amount.FillBytes(amount[:])
	}
	t := pqcrypto.NewTranscript().Fixed(d[:]).Uint32(ClassAVault).Uint64(m.VaultIndex).
		Fixed(owner).Fixed(keyID[:]).Uint64(m.Generation).Uint64(m.GuardNonce).
		Uint64(m.Deadline).Uint64(m.AnchorHeight).Fixed(m.AnchorHash).Uint32(m.Op).
		Fixed(amount[:]).Bytes(m.Recipient).Bytes(m.NewOwner).Bool(m.NewGuard != nil)
	if m.NewGuard != nil {
		t.Uint32(uint32(m.NewGuard.AlgID)).Bytes(m.NewGuard.PublicKey)
	}
	return t.Bytes(m.Sponsor).Message()
}

// EncodeVaultAccept encodes creation (index=0, txNonce=outer nonce, hash=zero)
// or rotation (real index, txNonce=0, hash=SHA-512(VAULT_CALL)).
func EncodeVaultAccept(d Domain, index, txNonce, generation uint64, owner []byte,
	delay uint64, alg pqcrypto.AlgID, key []byte, callHash [64]byte) []byte {
	return pqcrypto.NewTranscript().Fixed(d[:]).Uint32(ClassAVault).Uint64(index).
		Uint64(txNonce).Uint64(generation).Fixed(owner).Uint64(delay).
		Uint32(uint32(alg)).Bytes(key).Fixed(callHash[:]).Message()
}

// EncodeAcceptBV2 uses the existing Class B AcceptContext with the V2 layout.
func EncodeAcceptBV2(d Domain, index, generation uint64, owner []byte,
	delay uint64, alg pqcrypto.AlgID, key []byte, callHash [64]byte) []byte {
	return pqcrypto.NewTranscript().Fixed(d[:]).Uint32(ClassBStaking).Uint64(index).
		Uint64(generation).Fixed(owner).Uint64(delay).Uint32(uint32(alg)).Bytes(key).
		Fixed(callHash[:]).Message()
}

// EncodeGuardCancel binds one recovery request independently of guardNonce.
func EncodeGuardCancel(d Domain, m *GuardCancel) []byte {
	return pqcrypto.NewTranscript().Fixed(d[:]).Uint32(ClassBStaking).Uint64(m.BucketIndex).
		Uint64(m.Generation).Uint64(m.RequestID).Uint64(m.Deadline).Uint64(m.AnchorHeight).
		Fixed(m.AnchorHash).Bytes(m.Sponsor).Message()
}
