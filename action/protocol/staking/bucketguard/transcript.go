package bucketguard

import "encoding/binary"

// EncodeCall returns the byte-exact CALL transcript of IIP section 7. The
// owner is the bucket owner at execution time.
func EncodeCall(chainID uint32, owner Address, op *GuardedBucketOp) []byte {
	if op == nil {
		return nil
	}
	b := make([]byte, 0, 160+len(op.Recipient)+len(op.NewOwner)+len(op.CandidateName)+len(op.ExpectedCandidateID))
	b = append(b, DomainCall...)
	b = appendU32(b, chainID)
	b = appendU64(b, op.BucketIndex)
	b = append(b, owner[:]...)
	b = appendU64(b, op.Generation)
	b = appendU64(b, op.Nonce)
	b = appendU64(b, op.Deadline)
	b = appendU32(b, op.Op)
	b = appendBytes(b, op.Recipient)
	b = appendBytes(b, op.NewOwner)
	b = appendBytes(b, []byte(op.CandidateName))
	b = appendBytes(b, op.ExpectedCandidateID)
	b = appendU32(b, op.Duration)
	b = appendBool(b, op.AutoStake)
	if op.NewGuard != nil {
		b = appendBool(b, true)
		b = appendU32(b, op.NewGuard.AlgID)
		b = appendBytes(b, op.NewGuard.PublicKey)
	} else {
		b = appendBool(b, false)
	}
	b = appendBool(b, op.ClearGuard)
	return b
}

// EncodeAccept returns the byte-exact ACCEPT transcript of IIP section 7.
func EncodeAccept(chainID uint32, bucketIndex, newGeneration uint64, resultingOwner Address, algID uint32, publicKey []byte, deadline uint64) []byte {
	b := make([]byte, 0, 96+len(publicKey))
	b = append(b, DomainAccept...)
	b = appendU32(b, chainID)
	b = appendU64(b, bucketIndex)
	b = appendU64(b, newGeneration)
	b = append(b, resultingOwner[:]...)
	b = appendU32(b, algID)
	b = appendBytes(b, publicKey)
	b = appendU64(b, deadline)
	return b
}

func appendU32(b []byte, v uint32) []byte {
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], v)
	return append(b, buf[:]...)
}

func appendU64(b []byte, v uint64) []byte {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], v)
	return append(b, buf[:]...)
}

func appendBytes(b, v []byte) []byte {
	b = appendU32(b, uint32(len(v)))
	return append(b, v...)
}

func appendBool(b []byte, v bool) []byte {
	if v {
		return append(b, 0x01)
	}
	return append(b, 0x00)
}
