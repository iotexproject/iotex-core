package randutil

import (
	"crypto/rand"
	"math"
	"math/big"
)

var (
	//Read is alias of rand.Read
	Read = rand.Read
)

// Intn returns a random int in [0, n) use cryto random source
func Intn(n int) int {
	if n <= 0 {
		panic("invalid argument to Intn")
	}
	rnd, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		panic(err)
	}
	return int(rnd.Int64())
}

// Int63 returns a random int in [0, math.MaxInt64) use cryto random source
func Int63() int64 {
	rnd, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	if err != nil {
		panic(err)
	}
	return rnd.Int64()
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
