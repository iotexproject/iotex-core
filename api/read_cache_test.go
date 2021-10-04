package api

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReadKey(t *testing.T) {
	r := require.New(t)

	var keys []string
	for _, v := range []ReadKey{
		{"staking", "10", []byte("activeBuckets"), [][]byte{[]byte{0, 1}, []byte{2, 3, 4, 5, 6, 7, 8}}},
		{"staking", "10", []byte("activeBuckets"), [][]byte{[]byte{0, 1, 2}, []byte{3, 4, 5, 6, 7, 8}}},
		{"staking", "10", []byte("activeBuckets"), [][]byte{[]byte{0, 1, 2, 3}, []byte{4, 5, 6, 7, 8}}},
		{"staking", "10", []byte("activeBuckets"), [][]byte{[]byte{0, 1, 2, 3, 4, 5}, []byte{6, 7, 8}}},
		{"staking", "10", []byte("activeBuckets"), [][]byte{[]byte{0, 1, 2, 3, 4, 5, 6, 7}, []byte{8}}},
	} {
		keys = append(keys, v.String())
	}

	// all keys are different
	for i := range keys {
		k := keys[i]
		for j := i + 1; j < len(keys); j++ {
			r.NotEqual(k, keys[j])
		}
	}
}

func TestReadCache(t *testing.T) {
	r := require.New(t)

	c := NewReadCache()
	rcTests := []struct {
		k string
		v []byte
	}{
		{"1", []byte{1}},
		{"2", []byte{2}},
		{"3", []byte{1}},
		{"4", []byte{2}},
	}
	for _, v := range rcTests {
		d, ok := c.Get(v.k)
		r.False(ok)
		r.Nil(d)
		c.Put(v.k, v.v)
	}

	for _, v := range rcTests {
		d, ok := c.Get(v.k)
		r.True(ok)
		r.Equal(v.v, d)
	}

	c.Clear()
	for _, v := range rcTests {
		d, ok := c.Get(v.k)
		r.False(ok)
		r.Nil(d)
	}
}
