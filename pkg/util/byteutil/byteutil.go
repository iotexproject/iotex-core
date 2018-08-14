// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package byteutil

import (
	"github.com/iotexproject/iotex-core/pkg/enc"
	"github.com/iotexproject/iotex-core/pkg/hash"
)

// Uint32ToBytes converts a uint32 to 4 bytes with the machine endian
func Uint32ToBytes(value uint32) []byte {
	bytes := make([]byte, 4)
	enc.MachineEndian.PutUint32(bytes, value)
	return bytes
}

// Uint64ToBytes converts a uint64 to 8 bytes with the machine endian
func Uint64ToBytes(value uint64) []byte {
	bytes := make([]byte, 8)
	enc.MachineEndian.PutUint64(bytes, value)
	return bytes
}

// BytesTo20B converts a byte slice to 20-Byte array
func BytesTo20B(b []byte) hash.AddrHash {
	var h hash.AddrHash
	copy(h[:], b)
	return h
}

// BytesTo32B converts a byte slice to 32-Byte array
func BytesTo32B(b []byte) hash.Hash32B {
	var h hash.Hash32B
	copy(h[:], b)
	return h
}
