package fastrand

import (
	_ "unsafe" // for go link runtime fastrand
)

// Uint32 returns a random 32-bit
//go:linkname Uint32 runtime.fastrand
func Uint32() uint32

// Uint32n returns a random 32-bit in the range [0..n).
//go:linkname Uint32n runtime.fastrandn
func Uint32n(n uint32) uint32

// Read generates len(p) random bytes and writes them into p
func Read(p []byte) (n int) {
	for ; n < len(p); n++ {
		p[n] = byte(Uint32())
	}
	return
}
