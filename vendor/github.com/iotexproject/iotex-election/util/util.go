// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package util

import (
	"encoding/binary"
)

func Uint64ToBytes(u uint64) []byte {
	retval := make([]byte, 8)
	binary.LittleEndian.PutUint64(retval, u)

	return retval
}

func BytesToUint64(b []byte) uint64 {
	return binary.LittleEndian.Uint64(b)
}

func CopyBytes(b []byte) []byte {
	c := make([]byte, len(b))
	copy(c, b)

	return c
}
