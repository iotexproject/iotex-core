package routine_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iotexproject/iotex-core/pkg/routine"
	"github.com/stretchr/testify/require"
)

func TestTriggerTask(t *testing.T) {
	require := require.New(t)
	h := &MockHandler{Count: 0}
	ctx := context.Background()
	task := routine.NewTriggerTask(h.Do, routine.DelayTimeBeforeTrigger(180*time.Millisecond))
	require.NoError(task.Start(ctx))
	ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()
	var succ uint32
done:
	for {
		select {
		case <-ctx.Done():
			break done
		default:
			if task.Trigger() {
				succ++
			}
		}
	}
	time.Sleep(200 * time.Millisecond)
	require.Equal(uint32(6), succ)
	require.Equal(uint(6), h.Count)
	require.NoError(task.Stop(ctx))
	task.Trigger()
	require.Equal(uint(6), h.Count)
}

func TestTriggerTaskWithBufferSize(t *testing.T) {
	require := require.New(t)
	h := &MockHandler{Count: 0}
	ctx := context.Background()
	task := routine.NewTriggerTask(h.Do,
		routine.DelayTimeBeforeTrigger(180*time.Millisecond),
		routine.TriggerBufferSize(2))
	require.NoError(task.Start(ctx))
	ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()
	wg := sync.WaitGroup{}
	wg.Add(5)
	var succ uint32
	for i := 0; i < 5; i++ {
		go func(i int) {
			defer wg.Done()
		done:
			for {
				select {
				case <-ctx.Done():
					//t.Logf("exit %d\n", i)
					break done
				default:
					if task.Trigger() {
						atomic.AddUint32(&succ, 1)
					}
				}
			}
		}(i)
	}
	wg.Wait()
	time.Sleep(500 * time.Millisecond)
	require.Equal(uint32(8), succ)
	require.Equal(uint(8), h.Count)
	require.NoError(task.Stop(ctx))
	task.Trigger()
	require.Equal(uint(8), h.Count)
}
