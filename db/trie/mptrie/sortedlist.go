package mptrie

import (
	"sort"
)

type sortedList struct {
	li []uint8
}

func NewSortList(children map[byte]node) *sortedList {
	if len(children) == 0 {
		return &sortedList{
			li: make([]uint8, 0),
		}
	}
	li := make([]uint8, 0, len(children))
	for k := range children {
		li = append(li, k)
	}
	sort.Slice(li, func(i, j int) bool {
		return li[i] < li[j]
	})
	return &sortedList{
		li: li,
	}
}

func (sl *sortedList) Insert(key uint8) {
	i := sort.Search(len(sl.li), func(i int) bool {
		return sl.li[i] >= uint8(key)
	})
	if i == len(sl.li) {
		sl.li = append(sl.li, key)
	} else {
		if sl.li[i] != key {
			sl.li = append(sl.li, 0)
			copy(sl.li[i+1:], sl.li[i:])
			sl.li[i] = key
		}
	}
}

func (sl *sortedList) List() []uint8 {
	return sl.li
}

func (sl *sortedList) Delete(key uint8) {
	i := sort.Search(len(sl.li), func(i int) bool {
		return sl.li[i] >= key
	})
	if i < len(sl.li) && sl.li[i] == key {
		if i < len(sl.li)-1 {
			copy(sl.li[i:], sl.li[i+1:])
		}
		sl.li = sl.li[:len(sl.li)-1]
	}
}
