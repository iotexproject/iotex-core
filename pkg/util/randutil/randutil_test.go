package randutil

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInt(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()

	tests := []struct {
		name string
		max  int64
		fn   func() int64
	}{
		{"Int63", math.MaxInt64, Int63},
		{"Int", int64(math.MaxInt), func() int64 { return int64(Int()) }},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var min, max, set, unset int64
			unset = math.MaxInt64
			for i := 0; i < 100; i++ {
				v := test.fn()
				if v < min {
					min = v
				}
				if v > max {
					max = v
				}
				set |= v
				unset &= v
			}
			assert.GreaterOrEqual(max, test.max-(test.max>>2), "no output near expected max")
			assert.Less(max, test.max, "shouldn't be less than test.max")
			assert.LessOrEqual(min, test.max>>2, "no output near expected min")
			assert.GreaterOrEqual(int64(0), min, "shouldn't be less than 0")
			assert.Equal(test.max, set, "all bits should've been set at least once")
			assert.Empty(unset, "all bit should've been unset at least once")
		})
	}
}

func TestUint64(t *testing.T) {
	assert := assert.New(t)
	n := Uint64()
	assert.GreaterOrEqual(n, uint64(0))
	assert.LessOrEqual(n, uint64(math.MaxUint64))
}

func TestIntn(t *testing.T) {
	assert := assert.New(t)
	n := Intn(10)
	assert.GreaterOrEqual(n, 0)
	assert.Panics(func() { Intn(0) }, "The code did not panic")
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

func TestRand(t *testing.T) {
	b := make([]byte, 10)
	Read(b)
	assert.Equal(t, 10, len(b))
}
