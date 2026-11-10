package api

import (
	"sync"
	"testing"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/stretchr/testify/require"
)

func TestReadKey(t *testing.T) {
	r := require.New(t)

	var keys []hash.Hash160
	for _, v := range []ReadKey{
		{"staking", "10", []byte("activeBuckets"), [][]byte{[]byte{0, 1}, []byte{2, 3, 4, 5, 6, 7, 8}}},
		{"staking", "10", []byte("activeBuckets"), [][]byte{[]byte{0, 1, 2}, []byte{3, 4, 5, 6, 7, 8}}},
		{"staking", "10", []byte("activeBuckets"), [][]byte{[]byte{0, 1, 2, 3}, []byte{4, 5, 6, 7, 8}}},
		{"staking", "10", []byte("activeBuckets"), [][]byte{[]byte{0, 1, 2, 3, 4, 5}, []byte{6, 7, 8}}},
		{"staking", "10", []byte("activeBuckets"), [][]byte{[]byte{0, 1, 2, 3, 4, 5, 6, 7}, []byte{8}}},
	} {
		keys = append(keys, v.Hash())
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
		k hash.Hash160
		v []byte
	}{
		{hash.Hash160b([]byte{1}), []byte{1}},
		{hash.Hash160b([]byte{2}), []byte{2}},
		{hash.Hash160b([]byte{3}), []byte{1}},
		{hash.Hash160b([]byte{4}), []byte{2}},
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

func TestReadCacheConcurrentGet(t *testing.T) {
	r := require.New(t)

	c := NewReadCache()
	key := hash.Hash160b([]byte{1})
	c.Put(key, []byte{1})
	miss := hash.Hash160b([]byte{2})

	const (
		workers = 16
		rounds  = 1000
	)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < rounds; j++ {
				c.Get(key)
				c.Get(miss)
			}
		}()
	}
	wg.Wait()
	r.EqualValues(2*workers*rounds, c.total.Load())
	r.EqualValues(workers*rounds, c.hit.Load())
}
