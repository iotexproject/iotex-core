// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package utils

import "github.com/iotexproject/iotex-core/common"

// Uint32ToBytes converts a uint32 to 4 bytes with the machine endian
func Uint32ToBytes(value uint32) []byte {
	bytes := make([]byte, 4)
	common.MachineEndian.PutUint32(bytes, value)
	return bytes
}

// BytesToUint32 converts 4 bytes with the machine endian to a uint32
func BytesToUint32(bytes []byte) uint32 {
	return common.MachineEndian.Uint32(bytes)
}
