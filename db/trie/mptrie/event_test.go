package mptrie

import (
	"sort"
	"testing"
	"time"
)

var fakeTime = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

func TestEvents(t *testing.T) {
	evv := Events{
		{Op: eventUpsert, LastVisit: fakeTime.Add(3 * time.Second)},
		{Op: eventSearch, LastVisit: fakeTime.Add(2 * time.Second)},
		{Op: eventDelete, LastVisit: fakeTime.Add(1 * time.Second)},
	}
	sort.Sort(evv)
	if evv[0].Op != eventDelete {
		t.Errorf("expected eventDelete, got %v", evv[0].Op)
	}
	if evv[1].Op != eventSearch {
		t.Errorf("expected eventSearch, got %v", evv[1].Op)
	}
	if evv[2].Op != eventUpsert {
		t.Errorf("expected eventUpsert, got %v", evv[2].Op)
	}
}
