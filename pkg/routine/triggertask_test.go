package routine_test

import (
	"context"
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
	for {
		select {
		case <-ctx.Done():
			goto done
		default:
			task.Trigger()
		}
	}
done:
	require.Equal(uint(5), h.Count)
	require.NoError(task.Stop(ctx))
	task.Trigger()
	require.Equal(uint(5), h.Count)
}
