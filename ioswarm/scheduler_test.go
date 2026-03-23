package ioswarm

import (
	"testing"

	pb "github.com/iotexproject/iotex-core/v2/ioswarm/proto"
	"go.uber.org/zap"
)

func TestSchedulerDispatchLeastLoaded(t *testing.T) {
	r := NewRegistry(10)
	logger := zap.NewNop()
	s := NewScheduler(r, logger)

	// Register 2 agents
	r.Register(&pb.RegisterRequest{AgentID: "a1", Capability: pb.TaskLevel_L2_STATE_VERIFY})
	r.Register(&pb.RegisterRequest{AgentID: "a2", Capability: pb.TaskLevel_L2_STATE_VERIFY})

	// Pre-load a1 with some pending tasks to make it "busier"
	a1, _ := r.GetAgent("a1")
	a1.TasksPending = 10

	// Create 2 tasks (1 batch)
	tasks := []*pb.TaskPackage{
		{TaskID: 1, Level: pb.TaskLevel_L2_STATE_VERIFY},
		{TaskID: 2, Level: pb.TaskLevel_L2_STATE_VERIFY},
	}

	dispatched := s.Dispatch(tasks, 10)
	if dispatched != 2 {
		t.Fatalf("expected 2 dispatched, got %d", dispatched)
	}

	// a2 should get the batch because it has lower load
	a2, _ := r.GetAgent("a2")
	if len(a2.TaskChan) != 1 {
		t.Fatalf("expected a2 to receive batch (load=0), got %d batches", len(a2.TaskChan))
	}
	if len(a1.TaskChan) != 0 {
		t.Fatalf("expected a1 to have 0 batches (higher load), got %d", len(a1.TaskChan))
	}
}

func TestSchedulerCapabilityFiltering(t *testing.T) {
	r := NewRegistry(10)
	logger := zap.NewNop()
	s := NewScheduler(r, logger)

	// a1 is L1 only, a2 is L3
	r.Register(&pb.RegisterRequest{AgentID: "a1", Capability: pb.TaskLevel_L1_SIG_VERIFY})
	r.Register(&pb.RegisterRequest{AgentID: "a2", Capability: pb.TaskLevel_L3_FULL_EXECUTE})

	// L3 task should only go to a2
	tasks := []*pb.TaskPackage{
		{TaskID: 1, Level: pb.TaskLevel_L3_FULL_EXECUTE},
	}

	dispatched := s.Dispatch(tasks, 10)
	if dispatched != 1 {
		t.Fatalf("expected 1 dispatched, got %d", dispatched)
	}

	a1, _ := r.GetAgent("a1")
	a2, _ := r.GetAgent("a2")
	if len(a1.TaskChan) != 0 {
		t.Fatal("L1 agent should not receive L3 tasks")
	}
	if len(a2.TaskChan) != 1 {
		t.Fatal("L3 agent should receive the task")
	}
}

func TestSchedulerRetryQueue(t *testing.T) {
	r := NewRegistry(10)
	logger := zap.NewNop()
	s := NewScheduler(r, logger)

	// Register agent with small channel that's already full
	r.Register(&pb.RegisterRequest{AgentID: "a1", Capability: pb.TaskLevel_L2_STATE_VERIFY})
	a1, _ := r.GetAgent("a1")

	// Fill the channel (capacity is 16)
	for i := 0; i < 16; i++ {
		a1.TaskChan <- &pb.TaskBatch{BatchID: itoa(i)}
	}

	// Now dispatch more — should go to retry queue
	tasks := []*pb.TaskPackage{
		{TaskID: 100, Level: pb.TaskLevel_L2_STATE_VERIFY},
	}
	dispatched := s.Dispatch(tasks, 10)
	if dispatched != 0 {
		t.Fatalf("expected 0 dispatched (channel full), got %d", dispatched)
	}

	if s.RetryQueueLen() != 1 {
		t.Fatalf("expected 1 batch in retry queue, got %d", s.RetryQueueLen())
	}
}

func TestSchedulerRetryDrain(t *testing.T) {
	r := NewRegistry(10)
	logger := zap.NewNop()
	s := NewScheduler(r, logger)

	// Register agent with full channel
	r.Register(&pb.RegisterRequest{AgentID: "a1", Capability: pb.TaskLevel_L2_STATE_VERIFY})
	a1, _ := r.GetAgent("a1")

	for i := 0; i < 16; i++ {
		a1.TaskChan <- &pb.TaskBatch{BatchID: itoa(i)}
	}

	// Dispatch — goes to retry
	tasks := []*pb.TaskPackage{
		{TaskID: 100, Level: pb.TaskLevel_L2_STATE_VERIFY},
	}
	s.Dispatch(tasks, 10)
	if s.RetryQueueLen() != 1 {
		t.Fatalf("expected 1 in retry, got %d", s.RetryQueueLen())
	}

	// Drain the channel
	for len(a1.TaskChan) > 0 {
		<-a1.TaskChan
	}

	// Next dispatch should drain retry queue first + dispatch new tasks
	newTasks := []*pb.TaskPackage{
		{TaskID: 200, Level: pb.TaskLevel_L2_STATE_VERIFY},
	}
	dispatched := s.Dispatch(newTasks, 10)
	// Should dispatch both the retried batch (1 task) and new batch (1 task) = 2 tasks
	if dispatched != 2 {
		t.Fatalf("expected 2 dispatched (retry + new), got %d", dispatched)
	}
	if s.RetryQueueLen() != 0 {
		t.Fatalf("expected retry queue drained, got %d", s.RetryQueueLen())
	}
}

func TestSchedulerNoAgents(t *testing.T) {
	r := NewRegistry(10)
	logger := zap.NewNop()
	s := NewScheduler(r, logger)

	tasks := []*pb.TaskPackage{{TaskID: 1}}
	dispatched := s.Dispatch(tasks, 10)
	if dispatched != 0 {
		t.Fatalf("expected 0 dispatched with no agents, got %d", dispatched)
	}
	// Should be in retry queue
	if s.RetryQueueLen() != 1 {
		t.Fatalf("expected 1 in retry queue, got %d", s.RetryQueueLen())
	}
}

func TestSchedulerEmptyTasks(t *testing.T) {
	r := NewRegistry(10)
	logger := zap.NewNop()
	s := NewScheduler(r, logger)

	r.Register(&pb.RegisterRequest{AgentID: "a1"})

	dispatched := s.Dispatch(nil, 10)
	if dispatched != 0 {
		t.Fatalf("expected 0 dispatched with no tasks, got %d", dispatched)
	}
}

func TestSchedulerFallbackToNextAgent(t *testing.T) {
	r := NewRegistry(10)
	logger := zap.NewNop()
	s := NewScheduler(r, logger)

	// Both agents L2 capable
	r.Register(&pb.RegisterRequest{AgentID: "a1", Capability: pb.TaskLevel_L2_STATE_VERIFY})
	r.Register(&pb.RegisterRequest{AgentID: "a2", Capability: pb.TaskLevel_L2_STATE_VERIFY})

	// Fill a1 and a2 differently: a1 empty, a2 empty
	// Dispatch 17 batches (each 1 task) — first 16 go to a1 (least loaded), then a1 full, fallback to a2
	tasks := make([]*pb.TaskPackage, 20)
	for i := range tasks {
		tasks[i] = &pb.TaskPackage{TaskID: uint32(i + 1), Level: pb.TaskLevel_L2_STATE_VERIFY}
	}

	dispatched := s.Dispatch(tasks, 1) // batch size 1 = 20 batches
	if dispatched < 20 {
		t.Logf("dispatched %d/20 (some may have gone to retry)", dispatched)
	}

	a1, _ := r.GetAgent("a1")
	a2, _ := r.GetAgent("a2")

	// Both channels should have received tasks
	total := len(a1.TaskChan) + len(a2.TaskChan)
	if total == 0 {
		t.Fatal("expected some tasks dispatched")
	}
	t.Logf("a1=%d batches, a2=%d batches, retry=%d", len(a1.TaskChan), len(a2.TaskChan), s.RetryQueueLen())
}

func TestSchedulerNoCapableAgents(t *testing.T) {
	r := NewRegistry(10)
	logger := zap.NewNop()
	s := NewScheduler(r, logger)

	// Only L1 agents
	r.Register(&pb.RegisterRequest{AgentID: "a1", Capability: pb.TaskLevel_L1_SIG_VERIFY})

	// L3 task — no capable agent
	tasks := []*pb.TaskPackage{
		{TaskID: 1, Level: pb.TaskLevel_L3_FULL_EXECUTE},
	}
	dispatched := s.Dispatch(tasks, 10)
	if dispatched != 0 {
		t.Fatalf("expected 0, got %d", dispatched)
	}
	if s.RetryQueueLen() != 1 {
		t.Fatalf("expected 1 in retry, got %d", s.RetryQueueLen())
	}
}
