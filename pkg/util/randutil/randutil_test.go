package randutil

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIntn(t *testing.T) {
	assert := assert.New(t)
	n := Intn(10)
	assert.GreaterOrEqual(n, 0)
	assert.Panics(func() { Intn(0) }, "The code did not panic")
}

func TestInt63(t *testing.T) {
	assert := assert.New(t)
	for i := int64(0); i < 10; i++ {
		n := Int63()
		assert.GreaterOrEqual(n, int64(0))
		assert.LessOrEqual(n, int64(math.MaxInt64))

	}
}

func TestInt63n(t *testing.T) {
	assert := assert.New(t)
	for i := int64(0); i < 10; i++ {
		n := Int63n(10)
		assert.GreaterOrEqual(n, int64(0))
		assert.LessOrEqual(n, int64(10))
	}
	assert.Panics(func() { Int63n(0) }, "The code did not panic")
}

func TestInt(t *testing.T) {
	assert := assert.New(t)
	n := Int()
	assert.GreaterOrEqual(n, 0)
	assert.LessOrEqual(n, math.MaxInt)
}

func TestRand(t *testing.T) {
	b := make([]byte, 10)
	Read(b)
	assert.Equal(t, 10, len(b))
}
