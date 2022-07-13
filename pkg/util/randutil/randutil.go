package randutil

import (
	"crypto/rand"
	_ "unsafe" // For go:linkname
)

var (
	//Read is alias of rand.Read
	Read = rand.Read
)

// Uint64 returns a pseudo-random 64-bit value as a uint64.
func Uint64() uint64 {
	return uint64(fastrand())<<32 | uint64(fastrand())
}

// Intn returns a random int in [0, n) use cryto random source
func Intn(n int) int {
	return int(Int63n(int64(n)))
}

// Int63 returns a non-negative pseudo-random 63-bit integer as an int64.
func Int63() int64 {
	return int64(Uint64() & ((1 << 63) - 1))
}

// Int63n returns, as an int64, a non-negative pseudo-random number in the half-open interval [0,n).
// It panics if n <= 0.
func Int63n(n int64) int64 {
	if n <= 0 {
		panic("invalid argument to Int63n")
	}
	if n&(n-1) == 0 { // n is power of two, can mask
		return Int63() & (n - 1)
	}
	max := int64((1 << 63) - 1 - (1<<63)%uint64(n))
	v := Int63()
	for v > max {
		v = Int63()
	}
	return v % n
}

// Int returns a non-negative pseudo-random int.
func Int() int {
	u := uint(Int63())
	return int(u << 1 >> 1) // clear sign bit if int == int32
}

// fastrand is a fast thread local random function built into the Go runtime
// but not normally exposed.  On Linux x86_64, this is aesrand seeded by
// /dev/urandom.
//go:linkname fastrand runtime.fastrand
func fastrand() uint32
