package routine

import (
	"context"
	"sync"
	"time"

	"github.com/iotexproject/iotex-core/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/pkg/log"
)

var _ lifecycle.StartStopper = (*TriggerTask)(nil)

// TriggerTaskOption is option to TriggerTask.
type TriggerTaskOption interface {
	SetTriggerTaskOption(*TriggerTask)
}

type triggerTaskOption struct {
	setTriggerTaskOption func(*TriggerTask)
}

func (o triggerTaskOption) SetTriggerTaskOption(t *TriggerTask) {
	o.setTriggerTaskOption(t)
}

// DelayTimeBeforeTrigger sets the delay time before trigger
func DelayTimeBeforeTrigger(d time.Duration) TriggerTaskOption {
	return triggerTaskOption{
		setTriggerTaskOption: func(t *TriggerTask) {
			t.delay = d
		},
	}
}

// TriggerBufferSize sets the buffer size of trigger channel
func TriggerBufferSize(sz int) TriggerTaskOption {
	return triggerTaskOption{
		setTriggerTaskOption: func(t *TriggerTask) {
			t.sz = sz
		},
	}
}

// TriggerTask represents a task that can be triggered
type TriggerTask struct {
	lifecycle.Readiness
	delay time.Duration
	cb    Task
	sz    int
	ch    chan struct{}
	mu    sync.Mutex
}

// NewTriggerTask creates an instance of TriggerTask
func NewTriggerTask(cb Task, ops ...TriggerTaskOption) *TriggerTask {
	tt := &TriggerTask{
		cb:    cb,
		delay: 0,
		sz:    0,
	}
	for _, opt := range ops {
		opt.SetTriggerTaskOption(tt)
	}
	tt.ch = make(chan struct{}, tt.sz)
	return tt
}

// Start starts the task
func (t *TriggerTask) Start(_ context.Context) error {
	ready := make(chan struct{})
	go func() {
		close(ready)
		for range t.ch {
			if t.delay > 0 {
				time.Sleep(t.delay)
			}
			t.cb()
		}
	}()
	// ensure the goroutine has been running
	<-ready
	return t.TurnOn()
}

// Trigger triggers the task, return true if the task is triggered successfully
// this function is non-blocking
func (t *TriggerTask) Trigger() bool {
	if !t.IsReady() {
		log.S().Warnf("trigger task is not ready")
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	select {
	case t.ch <- struct{}{}:
		return true
	default:
	}
	return false
}

// Stop stops the task
func (t *TriggerTask) Stop(_ context.Context) error {
	// prevent stop is called before start.
	if err := t.TurnOff(); err != nil {
		return err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	close(t.ch)
	return nil
}
