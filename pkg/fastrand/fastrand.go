// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package fastrand

import (
	_ "unsafe" // for go link runtime fastrand
)

// Uint32 returns a random 32-bit
//
//go:linkname Uint32 runtime.fastrand
func Uint32() uint32

// Uint32n returns a random 32-bit in the range [0..n).
//
//go:linkname Uint32n runtime.fastrandn
func Uint32n(n uint32) uint32

// Read generates len(p) random bytes and writes them into p
func Read(p []byte) (n int) {
	for n < len(p) {
		val := Uint32()
		i := 0
		for ; i < 4; i++ {
			if n+i > len(p)-1 {
				break
			}
			p[n+i] = byte(val)
			val >>= 8
		}
		n = n + i
	}
	return
}
