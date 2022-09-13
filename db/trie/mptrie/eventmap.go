package mptrie

import (
	"sync"
	"time"
)

var eventMap sync.Map

type eventOp int

const (
	eventUpsert eventOp = iota
	eventSearch
	eventDelete
)

type event struct {
	op   eventOp
	time time.Time
}

func eventOnUpsert(n node) {
	eventMap.Store(n, event{
		op:   eventUpsert,
		time: time.Now(),
	})
}

func eventOnDelete(n node) {
	eventMap.Store(n, event{
		op:   eventDelete,
		time: time.Now(),
	})
}

func eventOnSearch(n node) {
	eventMap.Store(n, event{
		op:   eventDelete,
		time: time.Now(),
	})
}
