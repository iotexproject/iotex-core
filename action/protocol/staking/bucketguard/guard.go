package bucketguard

import (
	"bytes"
	"fmt"

	"google.golang.org/protobuf/encoding/protowire"

	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto"
)

// Guard is the guard record of one native bucket (IIP §4). A record with an
// empty PublicKey is a tombstone: the bucket is unguarded, AlgID and
// BoundHeight are zero, and the counters are kept so they never decrease.
// A nil *Guard means no record.
type Guard struct {
	AlgID       pqcrypto.AlgID
	PublicKey   []byte
	Generation  uint64 // incremented on every key installation
	Nonce       uint64 // incremented on every successful GuardedBucketOp
	BoundHeight uint64 // height at which the current key was installed
}

// IsGuarded reports whether g guards its bucket.
func (g *Guard) IsGuarded() bool { return g != nil && len(g.PublicKey) > 0 }

func (g *Guard) counters() (generation, nonce uint64) {
	if g == nil {
		return 0, 0
	}
	return g.Generation, g.Nonce
}

// Serialize encodes g as the canonical protobuf message
//
//	message BucketGuard {
//	  uint32 algID = 1; bytes publicKey = 2; uint64 generation = 3;
//	  uint64 nonce = 4; uint64 boundHeight = 5;
//	}
func (g *Guard) Serialize() ([]byte, error) {
	if err := g.check(); err != nil {
		return nil, err
	}
	var b []byte
	b = appendVarintField(b, 1, uint64(g.AlgID))
	b = appendBytesField(b, 2, g.PublicKey)
	b = appendVarintField(b, 3, g.Generation)
	b = appendVarintField(b, 4, g.Nonce)
	b = appendVarintField(b, 5, g.BoundHeight)
	return b, nil
}

// Deserialize decodes a record written by Serialize. It rejects unknown,
// repeated or out-of-order fields and inconsistent records, so a decoded
// record always re-serializes to the same bytes.
func (g *Guard) Deserialize(b []byte) error {
	var out Guard
	orig := b
	last := protowire.Number(0)
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return fmt.Errorf("bucket guard: %w", protowire.ParseError(n))
		}
		if num <= last || num > 5 {
			return fmt.Errorf("bucket guard: unexpected field %d", num)
		}
		last = num
		b = b[n:]
		if num == 2 {
			if typ != protowire.BytesType {
				return fmt.Errorf("bucket guard: field 2 has wire type %d", typ)
			}
			v, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return fmt.Errorf("bucket guard: %w", protowire.ParseError(n))
			}
			out.PublicKey = bytes.Clone(v)
			b = b[n:]
			continue
		}
		if typ != protowire.VarintType {
			return fmt.Errorf("bucket guard: field %d has wire type %d", num, typ)
		}
		v, n := protowire.ConsumeVarint(b)
		if n < 0 {
			return fmt.Errorf("bucket guard: %w", protowire.ParseError(n))
		}
		b = b[n:]
		switch num {
		case 1:
			if v > 0xffffffff {
				return fmt.Errorf("bucket guard: algID %d overflows uint32", v)
			}
			out.AlgID = pqcrypto.AlgID(v)
		case 3:
			out.Generation = v
		case 4:
			out.Nonce = v
		case 5:
			out.BoundHeight = v
		}
	}
	if err := out.check(); err != nil {
		return err
	}
	// Reject encodings Serialize would not produce (explicit zero values,
	// non-minimal varints), so stored bytes and records map one to one.
	if enc, _ := out.Serialize(); !bytes.Equal(enc, orig) {
		return fmt.Errorf("bucket guard: non-canonical encoding")
	}
	*g = out
	return nil
}

func (g *Guard) check() error {
	if len(g.PublicKey) == 0 {
		if g.AlgID != 0 || g.BoundHeight != 0 {
			return fmt.Errorf("bucket guard: tombstone must have algID 0 and boundHeight 0")
		}
		return nil
	}
	if !g.AlgID.Supported() || len(g.PublicKey) != g.AlgID.PublicKeySize() {
		return fmt.Errorf("bucket guard: invalid %s public key of %d bytes", g.AlgID, len(g.PublicKey))
	}
	return nil
}
