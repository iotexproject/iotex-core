package bucketguard

import "google.golang.org/protobuf/encoding/protowire"

// Canonical proto3 encoding C(m) of IIP §8: fields in ascending field-number
// order, each at most once; default scalars and empty bytes/strings omitted;
// minimal varints and length prefixes; nested messages emitted iff set; no
// unknown fields. Size limits and gas are defined over C(m) of the decoded
// message, never over received bytes.

func appendVarintField(b []byte, num protowire.Number, v uint64) []byte {
	if v == 0 {
		return b
	}
	b = protowire.AppendTag(b, num, protowire.VarintType)
	return protowire.AppendVarint(b, v)
}

func appendBoolField(b []byte, num protowire.Number, v bool) []byte {
	if !v {
		return b
	}
	return appendVarintField(b, num, 1)
}

func appendBytesField(b []byte, num protowire.Number, v []byte) []byte {
	if len(v) == 0 {
		return b
	}
	b = protowire.AppendTag(b, num, protowire.BytesType)
	return protowire.AppendBytes(b, v)
}

func (n *NewGuard) canonicalBytes() []byte {
	var b []byte
	b = appendVarintField(b, 1, uint64(n.AlgID))
	b = appendBytesField(b, 2, n.PublicKey)
	b = appendBytesField(b, 3, n.AcceptSig)
	return b
}

func (m *BindBucketGuard) canonicalBytes() []byte {
	b := make([]byte, 0, 32+len(m.PublicKey)+len(m.AcceptSig))
	b = appendVarintField(b, 1, m.BucketIndex)
	b = appendVarintField(b, 2, uint64(m.AlgID))
	b = appendBytesField(b, 3, m.PublicKey)
	b = appendBytesField(b, 4, m.AcceptSig)
	return b
}

func (m *GuardedBucketOp) canonicalBytes() []byte {
	b := make([]byte, 0, 128+len(m.Sig))
	b = appendVarintField(b, 1, m.BucketIndex)
	b = appendVarintField(b, 2, m.Generation)
	b = appendVarintField(b, 3, m.Nonce)
	b = appendVarintField(b, 4, m.Deadline)
	b = appendVarintField(b, 5, uint64(m.Op))
	b = appendBytesField(b, 6, m.Recipient)
	b = appendBytesField(b, 7, m.NewOwner)
	b = appendBytesField(b, 8, []byte(m.CandidateName))
	b = appendBytesField(b, 9, m.ExpectedCandidateID)
	b = appendVarintField(b, 10, uint64(m.Duration))
	b = appendBoolField(b, 11, m.AutoStake)
	if m.NewGuard != nil {
		// A set nested message is emitted even when all its fields are empty.
		b = protowire.AppendTag(b, 12, protowire.BytesType)
		b = protowire.AppendBytes(b, m.NewGuard.canonicalBytes())
	}
	b = appendBoolField(b, 13, m.ClearGuard)
	b = appendBytesField(b, 14, m.Sig)
	return b
}

// CanonicalSize is the consensus size of the action, len(C(m)).
func (m *BindBucketGuard) CanonicalSize() int { return len(m.canonicalBytes()) }

// CanonicalSize is the consensus size of the action, len(C(m)).
func (m *GuardedBucketOp) CanonicalSize() int { return len(m.canonicalBytes()) }

// IntrinsicGas prices BIND, which needs one verification (ACCEPT).
func (m *BindBucketGuard) IntrinsicGas(p Params) uint64 {
	return p.gas(m.CanonicalSize(), 1)
}

// IntrinsicGas prices an op: one verification (CALL), plus one (ACCEPT) when
// the op installs a new key.
func (m *GuardedBucketOp) IntrinsicGas(p Params) uint64 {
	k := uint64(1)
	if m.installsKey() {
		k = 2
	}
	return p.gas(m.CanonicalSize(), k)
}
