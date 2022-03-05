// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package byteutil

import (
	"encoding/binary"

	"github.com/iotexproject/iotex-core/pkg/enc"
)

// Uint32ToBytes converts a uint32 to 4 bytes in little-endian
func Uint32ToBytes(value uint32) []byte {
	bytes := make([]byte, 4)
	enc.MachineEndian.PutUint32(bytes, value)
	return bytes
}

// Uint64ToBytes converts a uint64 to 8 bytes in little-endian
func Uint64ToBytes(value uint64) []byte {
	bytes := make([]byte, 8)
	enc.MachineEndian.PutUint64(bytes, value)
	return bytes
}

// BytesToUint64 converts 8 bytes to uint64 in little-endian
func BytesToUint64(value []byte) uint64 {
	return enc.MachineEndian.Uint64(value)
}

// Must is a helper wraps a call to a function returning ([]byte, error) and panics if the error is not nil.
func Must(d []byte, err error) []byte {
	if err != nil {
		panic(err)
	}
	return d
}

// convert number sequence 0, 1, 2, ... n to big-endian results in byte-sorted []byte slice
// we leverage this nice property to search/iterate action index stored within a bucket

// Uint32ToBytesBigEndian converts a uint32 to 4 bytes in big-endian
func Uint32ToBytesBigEndian(value uint32) []byte {
	bytes := make([]byte, 4)
	binary.BigEndian.PutUint32(bytes, value)
	return bytes
}

// Uint64ToBytesBigEndian converts a uint64 to 8 bytes in big-endian
func Uint64ToBytesBigEndian(value uint64) []byte {
	bytes := make([]byte, 8)
	binary.BigEndian.PutUint64(bytes, value)
	return bytes
}

// BytesToUint64BigEndian converts 8 bytes to uint64 in big-endian
func BytesToUint64BigEndian(value []byte) uint64 {
	return binary.BigEndian.Uint64(value)
}
