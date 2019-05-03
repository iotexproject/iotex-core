package throttle

import (
	"context"
	"testing"
	"time"
)

func TestThrotter(t *testing.T) {
	tr := New(10, SetQueueLen(10))
	tr.Start(context.Background())
	i := 0
	for ; i < 100; i++ {
		if tr.Allow() {
			a++
		}
		time.Sleep(50 * time.Millisecond)
	}
	if a == i {
		t.Error("Failed to throttle.")
	}
}
