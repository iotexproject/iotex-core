// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package common

import "encoding/binary"

// MachineEndian is the endianess of the machine
var MachineEndian = binary.LittleEndian

// HashSize defines the size of hash
const (
	HashSize = 32
)

var (
	// ZeroHash32B is 32-bytes of all zero
	ZeroHash32B = Hash32B{}
)

// Hash32B is 32-byte hash value
type Hash32B [HashSize]byte

// ByteToNibbleLow return the low 4-bit nibble
func ByteToNibbleLow(b byte) uint8 {
	return b & 0xf
}

// ByteToNibbleHigh return the high 4-bit nibble
func ByteToNibbleHigh(b byte) uint8 {
	return (b >> 4) & 0xf
}
