// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package bucketguard

import (
	"errors"
	"fmt"
)

var (
	errTruncatedVarint  = errors.New("truncated varint")
	errOverflowVarint   = errors.New("varint overflows uint64")
	errNonCanonical     = errors.New("non-canonical varint")
	errInvalidWireType  = errors.New("invalid wire type")
	errUnknownField     = errors.New("unknown field")
	errDuplicateField   = errors.New("duplicate field")
	errInvalidFieldTag  = errors.New("invalid field tag")
	errInvalidBytesSize = errors.New("invalid bytes length")
)

// MarshalBinary returns the canonical proto3 encoding of the BucketGuard state
// record. The Guarded bool is derived from a non-empty public key and is not
// encoded.
func (g *Guard) MarshalBinary() ([]byte, error) {
	if g == nil {
		return nil, errors.New("nil guard")
	}
	b := make([]byte, 0, 96+len(g.PublicKey))
	b = protoUint32(b, 1, g.AlgID)
	b = protoBytes(b, 2, g.PublicKey)
	b = protoUint64(b, 3, g.Generation)
	b = protoUint64(b, 4, g.Nonce)
	b = protoUint64(b, 5, g.BoundHeight)
	return b, nil
}

// UnmarshalBinary parses the canonical proto3 encoding of a BucketGuard state
// record. Unknown, duplicate, non-minimal, and truncated fields are rejected.
func (g *Guard) UnmarshalBinary(data []byte) error {
	if g == nil {
		return errors.New("nil guard")
	}
	*g = Guard{}
	var seen [6]bool
	for len(data) > 0 {
		tag, n, err := readVarint(data)
		if err != nil {
			return err
		}
		if tag == 0 {
			return errInvalidFieldTag
		}
		data = data[n:]
		field := tag >> 3
		wire := tag & 7
		if field == 0 || field > 5 {
			return fmt.Errorf("%w %d", errUnknownField, field)
		}
		if seen[field] {
			return fmt.Errorf("%w %d", errDuplicateField, field)
		}
		seen[field] = true

		switch field {
		case 1:
			if wire != wireVarint {
				return fmt.Errorf("%w for field 1: %d", errInvalidWireType, wire)
			}
			v, n, err := readVarint(data)
			if err != nil {
				return err
			}
			if v > uint64(^uint32(0)) {
				return fmt.Errorf("algID overflows uint32")
			}
			g.AlgID = uint32(v)
			data = data[n:]
		case 2:
			if wire != wireBytes {
				return fmt.Errorf("%w for field 2: %d", errInvalidWireType, wire)
			}
			l, n, err := readVarint(data)
			if err != nil {
				return err
			}
			data = data[n:]
			if l > uint64(len(data)) {
				return errInvalidBytesSize
			}
			g.PublicKey = append([]byte(nil), data[:l]...)
			data = data[l:]
		case 3:
			if wire != wireVarint {
				return fmt.Errorf("%w for field 3: %d", errInvalidWireType, wire)
			}
			v, n, err := readVarint(data)
			if err != nil {
				return err
			}
			g.Generation = v
			data = data[n:]
		case 4:
			if wire != wireVarint {
				return fmt.Errorf("%w for field 4: %d", errInvalidWireType, wire)
			}
			v, n, err := readVarint(data)
			if err != nil {
				return err
			}
			g.Nonce = v
			data = data[n:]
		case 5:
			if wire != wireVarint {
				return fmt.Errorf("%w for field 5: %d", errInvalidWireType, wire)
			}
			v, n, err := readVarint(data)
			if err != nil {
				return err
			}
			g.BoundHeight = v
			data = data[n:]
		}
	}
	g.Guarded = len(g.PublicKey) != 0
	return nil
}

// Serialize implements the state serializer interface.
func (g *Guard) Serialize() ([]byte, error) { return g.MarshalBinary() }

// Deserialize implements the state deserializer interface.
func (g *Guard) Deserialize(data []byte) error { return g.UnmarshalBinary(data) }

func readVarint(b []byte) (uint64, int, error) {
	var x uint64
	var s uint
	for i, c := range b {
		if i == 10 {
			return 0, 0, errOverflowVarint
		}
		if c < 0x80 {
			if i > 0 && c == 0 {
				return 0, 0, errNonCanonical
			}
			if i == 9 && c > 1 {
				return 0, 0, errOverflowVarint
			}
			return x | uint64(c)<<s, i + 1, nil
		}
		x |= uint64(c&0x7f) << s
		s += 7
	}
	return 0, 0, errTruncatedVarint
}
