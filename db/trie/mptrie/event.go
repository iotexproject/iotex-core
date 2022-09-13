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

//Event is a struct that contains the node and the operation performed on it
type Event struct {
	Node      node
	Op        eventOp
	LastVisit time.Time
}

// Events is a slice of events
type Events []Event

func (e Events) Len() int {
	return len(e)
}

func (e Events) Less(i, j int) bool {
	return e[i].LastVisit.Before(e[j].LastVisit)
}

func (e Events) Swap(i, j int) {
	e[i], e[j] = e[j], e[i]
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

// GetEvents returns a slice of events
func GetEvents() Events {
	var evv Events
	eventMap.Range(func(key, value interface{}) bool {
		evv = append(evv, Event{
			Node:      key.(node),
			Op:        value.(event).op,
			LastVisit: value.(event).time,
		})
		return true
	})
	return evv
}
