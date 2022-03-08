package mptrie

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestListInsert(t *testing.T) {
	require := require.New(t)

	li := NewSortedList(nil)
	li.Insert(0)
	require.Equal([]uint8{0}, li.List())
	li.Insert(2)
	li.Insert(4)
	li.Insert(128)
	require.Equal([]uint8{0, 2, 4, 128}, li.List())
	li.Insert(1)
	require.Equal([]uint8{0, 1, 2, 4, 128}, li.List())
	li.Insert(0)
	require.Equal([]uint8{0, 1, 2, 4, 128}, li.List())
}

func TestDelete(t *testing.T) {
	require := require.New(t)

	li := NewSortedList(nil)
	li.Insert(1)
	li.Delete(1)
	li.Insert(0)
	require.Equal([]uint8{0}, li.List())
	li.Insert(2)
	li.Insert(4)
	require.Equal([]uint8{0, 2, 4}, li.List())
	li.Delete(1)
	require.Equal([]uint8{0, 2, 4}, li.List())
	li.Delete(0)
	require.Equal([]uint8{2, 4}, li.List())
	li.Insert(0)
	require.Equal([]uint8{0, 2, 4}, li.List())
	li.Delete(4)
	li.Delete(2)
	li.Delete(0)
	require.Equal([]uint8{}, li.List())
	li.Delete(1)
	require.Equal([]uint8{}, li.List())
}

func TestNewSortedList(t *testing.T) {
	require := require.New(t)
	sl := NewSortedList(map[byte]node{
		2: nil,
		3: nil,
		1: nil,
		5: nil,
		7: nil,
	})
	require.Equal([]uint8{1, 2, 3, 5, 7}, sl.List())
}

func BenchmarkDelete(b *testing.B) {
	num := 256
	li := NewSortedList(nil)
	for i := 0; i < num; i++ {
		li.Insert(uint8(i))
	}

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		for i := num - 1; i >= 0; i-- {
			li.Delete(uint8(i))
		}
	}
}

func BenchmarkInsert(b *testing.B) {
	num := 256
	li := NewSortedList(nil)
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		for i := 0; i < num; i++ {
			li.Insert(uint8(i))
		}
	}
}
