package bucketguard

import "encoding/binary"

// Minimal deterministic proto3 encoder. It implements the canonical encoding
// C(m) defined by IIP section 8: ascending field order, default-valued scalars
// and empty bytes/strings omitted, minimal varints and length prefixes, nested
// messages emitted iff set, and no unknown fields.

const (
	wireVarint  = 0
	wireFixed64 = 1
	wireBytes   = 2
	wireFixed32 = 5
)

func protoVarint(b []byte, v uint64) []byte {
	for v >= 0x80 {
		b = append(b, byte(v)|0x80)
		v >>= 7
	}
	return append(b, byte(v))
}

func protoTag(b []byte, field int, wire int) []byte {
	return protoVarint(b, uint64(field)<<3|uint64(wire))
}

func protoUint32(b []byte, field int, v uint32) []byte {
	if v == 0 {
		return b
	}
	b = protoTag(b, field, wireVarint)
	return protoVarint(b, uint64(v))
}

func protoUint64(b []byte, field int, v uint64) []byte {
	if v == 0 {
		return b
	}
	b = protoTag(b, field, wireVarint)
	return protoVarint(b, v)
}

func protoBool(b []byte, field int, v bool) []byte {
	if !v {
		return b
	}
	b = protoTag(b, field, wireVarint)
	return protoVarint(b, 1)
}

func protoBytes(b []byte, field int, v []byte) []byte {
	if len(v) == 0 {
		return b
	}
	b = protoTag(b, field, wireBytes)
	b = protoVarint(b, uint64(len(v)))
	return append(b, v...)
}

func protoString(b []byte, field int, v string) []byte {
	if v == "" {
		return b
	}
	return protoBytes(b, field, []byte(v))
}

func protoMessage(b []byte, field int, msg []byte, present bool) []byte {
	if !present {
		return b
	}
	b = protoTag(b, field, wireBytes)
	b = protoVarint(b, uint64(len(msg)))
	return append(b, msg...)
}

// ProtoVarint exposes the canonical varint encoding for vector tooling.
func ProtoVarint(v uint64) []byte { return protoVarint(nil, v) }

// Uint64BE returns the big-endian representation used by transcripts and logs.
func Uint64BE(v uint64) []byte {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], v)
	return b[:]
}

// CanonicalBytes returns C(newGuard).
func (n *NewGuard) CanonicalBytes() []byte {
	if n == nil {
		return nil
	}
	b := make([]byte, 0, 96+len(n.PublicKey)+len(n.AcceptSig))
	b = protoUint32(b, 1, n.AlgID)
	b = protoBytes(b, 2, n.PublicKey)
	b = protoBytes(b, 3, n.AcceptSig)
	return b
}

// CanonicalBytes returns C(bind).
func (m *BindBucketGuard) CanonicalBytes() []byte {
	b := make([]byte, 0, 96+len(m.PublicKey)+len(m.AcceptSig))
	b = protoUint64(b, 1, m.BucketIndex)
	b = protoUint32(b, 2, m.AlgID)
	b = protoBytes(b, 3, m.PublicKey)
	b = protoBytes(b, 4, m.AcceptSig)
	return b
}

// CanonicalSize is the consensus size used for limits and pricing.
func (m *BindBucketGuard) CanonicalSize() int { return len(m.CanonicalBytes()) }

// IntrinsicGas is 10,000 + 16*size + G_VERIFY for the single ACCEPT
// verification required by BIND.
func (m *BindBucketGuard) IntrinsicGas() uint64 {
	return intrinsicGas(m.CanonicalSize(), 1)
}

// CanonicalBytes returns C(op).
func (m *GuardedBucketOp) CanonicalBytes() []byte {
	capacity := 256 + len(m.Recipient) + len(m.NewOwner) + len(m.CandidateName) + len(m.ExpectedCandidateID) + len(m.Sig)
	if m.NewGuard != nil {
		capacity += 512 + len(m.NewGuard.PublicKey) + len(m.NewGuard.AcceptSig)
	}
	b := make([]byte, 0, capacity)
	b = protoUint64(b, 1, m.BucketIndex)
	b = protoUint64(b, 2, m.Generation)
	b = protoUint64(b, 3, m.Nonce)
	b = protoUint64(b, 4, m.Deadline)
	b = protoUint32(b, 5, m.Op)
	b = protoBytes(b, 6, m.Recipient)
	b = protoBytes(b, 7, m.NewOwner)
	b = protoString(b, 8, m.CandidateName)
	b = protoBytes(b, 9, m.ExpectedCandidateID)
	b = protoUint32(b, 10, m.Duration)
	b = protoBool(b, 11, m.AutoStake)
	if m.NewGuard != nil {
		b = protoMessage(b, 12, m.NewGuard.CanonicalBytes(), true)
	}
	b = protoBool(b, 13, m.ClearGuard)
	b = protoBytes(b, 14, m.Sig)
	return b
}

// CanonicalSize is the consensus size used for limits and pricing.
func (m *GuardedBucketOp) CanonicalSize() int { return len(m.CanonicalBytes()) }

// IntrinsicGas is 10,000 + 16*size + k*G_VERIFY, with k=2 when a new guard is
// installed and k=1 otherwise.
func (m *GuardedBucketOp) IntrinsicGas() uint64 {
	k := uint64(1)
	if m.NewGuard != nil {
		k = 2
	}
	return intrinsicGas(m.CanonicalSize(), k)
}

func intrinsicGas(size int, verifications uint64) uint64 {
	return IntrinsicGasBase + uint64(GasPerSizeByte)*uint64(size) + verifications*GVerify
}
