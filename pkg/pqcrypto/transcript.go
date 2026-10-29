// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package pqcrypto

import "encoding/binary"

// Transcript builds a byte-exact message to be signed directly (no
// intermediate digest). Integers are big-endian; booleans are one byte, 0x00
// or 0x01; variable-length byte strings carry a uint32 length prefix. Domain
// separation belongs in the signing context, not in the transcript.
type Transcript struct {
	b []byte
}

// NewTranscript starts an empty transcript.
func NewTranscript() *Transcript {
	return &Transcript{b: make([]byte, 0, 256)}
}

// Uint8 appends one byte.
func (t *Transcript) Uint8(v uint8) *Transcript {
	t.b = append(t.b, v)
	return t
}

// Bool appends 0x01 for true and 0x00 for false.
func (t *Transcript) Bool(v bool) *Transcript {
	if v {
		return t.Uint8(1)
	}
	return t.Uint8(0)
}

// Uint32 appends v as 4 big-endian bytes.
func (t *Transcript) Uint32(v uint32) *Transcript {
	t.b = binary.BigEndian.AppendUint32(t.b, v)
	return t
}

// Uint64 appends v as 8 big-endian bytes.
func (t *Transcript) Uint64(v uint64) *Transcript {
	t.b = binary.BigEndian.AppendUint64(t.b, v)
	return t
}

// Fixed appends a fixed-width field (for example a 20-byte address) without a
// length prefix. The caller is responsible for the width.
func (t *Transcript) Fixed(v []byte) *Transcript {
	t.b = append(t.b, v...)
	return t
}

// Bytes appends uint32_be(len(v)) || v.
func (t *Transcript) Bytes(v []byte) *Transcript {
	return t.Uint32(uint32(len(v))).Fixed(v)
}

// Message returns the encoded transcript. The returned slice must not be
// modified while the transcript is still being appended to.
func (t *Transcript) Message() []byte { return t.b }
