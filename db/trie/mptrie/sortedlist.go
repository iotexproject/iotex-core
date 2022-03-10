package mptrie

import (
	"sort"
)

// SortedList is a data structure where elements are in ascending order
type SortedList struct {
	li       []uint8
	isSorted bool // lazy-initilization
}

// NewSortedList create SortedList from keys in the children map
func NewSortedList(children map[byte]node) *SortedList {
	if len(children) == 0 {
		return &SortedList{
			li:       make([]uint8, 0),
			isSorted: true,
		}
	}
	li := make([]uint8, 0, len(children))
	for k := range children {
		li = append(li, k)
	}
	return &SortedList{
		li:       li,
		isSorted: false,
	}
}

// Insert insert key into sortedlist
func (sl *SortedList) Insert(key uint8) {
	sl.sortIfNeed()
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

// List returns sorted indices
func (sl *SortedList) List() []uint8 {
	sl.sortIfNeed()
	return sl.li
}

// Delete deletes key in the sortedlist
func (sl *SortedList) Delete(key uint8) {
	sl.sortIfNeed()
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

func (sl *SortedList) sortIfNeed() {
	if !sl.isSorted {
		sort.Slice(sl.li, func(i, j int) bool {
			return sl.li[i] < sl.li[j]
		})
		sl.isSorted = true
	}
}
