package mptrie

import (
	"testing"

	"github.com/iotexproject/iotex-core/db/trie/triepb"
	"github.com/stretchr/testify/require"
)

var (
	node1 = newHashNode(nil, []byte{1})
	node2 = newHashNode(nil, []byte{2})
)

func TestUpsert(t *testing.T) {
	require := require.New(t)

	li := NewSortList()
	li.Upsert(0, node1)
	require.Equal([]uint8{0}, getIndexFromListArr(getAllElementFromList(li)))
	li.Upsert(2, node1)
	li.Upsert(4, node1)
	li.Upsert(128, node1)
	require.Equal([]uint8{0, 2, 4, 128}, getIndexFromListArr(getAllElementFromList(li)))
	li.Upsert(1, node1)
	require.Equal([]uint8{0, 1, 2, 4, 128}, getIndexFromListArr(getAllElementFromList(li)))
	li.Upsert(0, node2)
	arr1 := getAllElementFromList(li)
	require.True(len(arr1) > 2)
	require.Equal(node2, arr1[0].data)
	require.Equal(node2, li.bucket[0].data)
	require.Equal(node1, arr1[1].data)
	require.Equal(node1, li.bucket[1].data)
}

func TestDelete(t *testing.T) {
	require := require.New(t)

	li := NewSortList()
	li.Upsert(0, node1)
	require.Equal([]uint8{0}, getIndexFromListArr(getAllElementFromList(li)))
	li.Upsert(2, node1)
	li.Upsert(4, node1)
	require.Equal([]uint8{0, 2, 4}, getIndexFromListArr(getAllElementFromList(li)))
	err := li.Delete(1)
	require.Error(err)
	require.Equal([]uint8{0, 2, 4}, getIndexFromListArr(getAllElementFromList(li)))
	require.Equal(node1, li.bucket[0].data)
	err = li.Delete(0)
	require.NoError(err)
	require.Equal([]uint8{2, 4}, getIndexFromListArr(getAllElementFromList(li)))
	require.Nil(li.bucket[0])
	li.Upsert(0, node2)
	require.Equal([]uint8{0, 2, 4}, getIndexFromListArr(getAllElementFromList(li)))
	require.Equal(node2, li.bucket[0].data)
}

func TestGet(t *testing.T) {
	require := require.New(t)

	li := NewSortList()
	li.Upsert(0, node2)
	li.Upsert(2, node1)
	li.Upsert(4, node1)
	require.Equal([]uint8{0, 2, 4}, getIndexFromListArr(getAllElementFromList(li)))
	require.Equal(node2, li.Get(0))
	require.Nil(li.Get(1))
}

func TestNewSortListFromProtoPb(t *testing.T) {
	require := require.New(t)

	data := []*triepb.BranchNodePb{
		{
			Index: 0,
			Path:  []byte{1},
		},
		{
			Index: 2,
			Path:  []byte{2},
		},
		{
			Index: 4,
			Path:  []byte{4},
		},
		{
			Index: 5,
			Path:  []byte{5},
		},
	}
	li := NewSortListFromProtoPb(nil, data)
	require.Equal([]uint8{0, 2, 4, 5}, getIndexFromListArr(getAllElementFromList(li)))
}

func TestSize(t *testing.T) {
	require := require.New(t)
	li := NewSortList()
	for i := 0; i < int(radix); i++ {
		li.Upsert(uint8(i), node1)
	}
	require.Equal(uint16(radix), li.Size())
}

func getAllElementFromList(sl *sortedList) []listElement {
	ret := make([]listElement, 0, sl.Size())
	for ptr := sl.Front(); ptr != nil; ptr = ptr.Next() {
		tmp := ptr.Value.(*listElement)
		ret = append(ret, *tmp)
	}
	return ret
}

func getIndexFromListArr(in []listElement) []uint8 {
	ret := make([]uint8, 0, len(in))
	for _, v := range in {
		ret = append(ret, uint8(v.idx))
	}
	return ret
}
