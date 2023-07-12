package routine

import (
	"context"
	"log"
	"time"

	"github.com/iotexproject/iotex-core/pkg/lifecycle"
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

// WithTriggerTaskInterval sets the interval of the task
func WithTriggerTaskInterval(d time.Duration) TriggerTaskOption {
	return triggerTaskOption{
		setTriggerTaskOption: func(t *TriggerTask) {
			t.duration = d
		},
	}
}

// TriggerTask represents a task that can be triggered
type TriggerTask struct {
	lifecycle.Readiness
	duration time.Duration
	cb       Task
	ch       chan struct{}
}

// NewTriggerTask creates an instance of TriggerTask
func NewTriggerTask(cb Task, ops ...TriggerTaskOption) *TriggerTask {
	tt := &TriggerTask{
		cb:       cb,
		duration: 0,
		ch:       make(chan struct{}),
	}
	for _, opt := range ops {
		opt.SetTriggerTaskOption(tt)
	}
	return tt
}

// Start starts the task
func (t *TriggerTask) Start(_ context.Context) error {
	ready := make(chan struct{})
	go func() {
		close(ready)
		for range t.ch {
			if t.duration > 0 {
				time.Sleep(t.duration)
			}
			t.cb()
		}
	}()
	// ensure the goroutine has been running
	<-ready
	return t.TurnOn()
}

// Trigger triggers the task
func (t *TriggerTask) Trigger() {
	if !t.IsReady() {
		log.Println("[WARN] trigger task is not ready")
		return
	}
	t.ch <- struct{}{}
}

// Stop stops the task
func (t *TriggerTask) Stop(_ context.Context) error {
	// prevent stop is called before start.
	if err := t.TurnOff(); err != nil {
		return err
	}
	close(t.ch)
	return nil
}
