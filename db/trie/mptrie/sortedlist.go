package mptrie

import (
	"container/list"
	"sort"

	"github.com/iotexproject/iotex-core/db/trie"
	"github.com/iotexproject/iotex-core/db/trie/triepb"
)

type (
	sortedList struct {
		bucket [radix]*listElement
		li     *list.List
	}

	listElement struct {
		idx  int16
		data node
	}
)

var (
	dummy = &listElement{-1, nil}
)

func NewSortList() *sortedList {
	newlist := list.New()
	newlist.PushBack(dummy)
	return &sortedList{
		li: newlist,
	}
}

// assume it is sorted?
func NewSortListFromProtoPb(mpt *merklePatriciaTrie, in []*triepb.BranchNodePb) *sortedList {
	sort.Slice(in, func(i, j int) bool {
		return in[i].Index < in[j].Index
	})

	sl := NewSortList()
	for _, v := range in {
		tmp := &listElement{int16(v.Index), newHashNode(mpt, v.Path)}
		sl.bucket[v.Index] = tmp
		sl.li.PushBack(tmp)
	}
	return sl
}

func (sl *sortedList) Upsert(key uint8, value node) {
	// key exists -> update
	if sl.bucket[key] != nil {
		sl.bucket[key].data = value
		return
	}
	// insert new key
	ptr := sl.li.Front()
	for ptr.Next() != nil && ptr.Next().Value.(*listElement).idx < int16(key) {
		ptr = ptr.Next()
	}
	newEle := &listElement{int16(key), value}
	sl.bucket[key] = newEle
	sl.li.InsertAfter(newEle, ptr)
}

func (sl *sortedList) Front() *list.Element {
	return sl.li.Front().Next()
}

func (sl *sortedList) Size() uint16 {
	return uint16(sl.li.Len() - 1)
}

func (sl *sortedList) Get(key uint8) node {
	val := sl.bucket[key]
	if val == nil {
		return nil
	}
	return val.data
}

func (sl *sortedList) Delete(key uint8) error {
	if sl.bucket[key] == nil {
		return trie.ErrNotExist
	}
	ptr := sl.li.Front().Next()
	for ptr != nil && ptr.Value.(*listElement).idx != int16(key) {
		ptr = ptr.Next()
	}
	if ptr == nil {
		// panic?
		return trie.ErrNotExist
	}
	sl.bucket[key] = nil
	sl.li.Remove(ptr)
	return nil
}
