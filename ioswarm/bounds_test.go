package ioswarm

import (
	"context"
	"fmt"
	"math/big"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	pb "github.com/iotexproject/iotex-core/v2/ioswarm/proto"
)

// countingState counts the state reads and simulations it serves.
type countingState struct {
	mockState
	reads, sims atomic.Int64
}

func (m *countingState) AccountState(address string) (*pb.AccountSnapshot, error) {
	m.reads.Add(1)
	// a code hash makes every receiver look like a contract, so L3 simulates
	return &pb.AccountSnapshot{Address: address, Balance: "1", CodeHash: []byte{1}}, nil
}

func (m *countingState) SimulateAccessList(string, string, []byte, string, uint64) (map[string][]string, error) {
	m.sims.Add(1)
	return nil, nil
}

func newBoundsTestCoordinator(t *testing.T, level string, ap ActPoolReader, sr StateReader) (*Coordinator, *grpcHandler) {
	logger := zap.NewNop()
	cfg := Config{MaxAgents: 10, TaskLevel: level, ShadowMode: true}
	registry := NewRegistry(cfg.MaxAgents)
	c := &Coordinator{
		cfg:             cfg,
		actPool:         ap,
		prefetcher:      NewPrefetcher(sr, logger),
		registry:        registry,
		scheduler:       NewScheduler(registry, logger),
		shadow:          NewShadowComparator(logger),
		reward:          NewRewardDistributor(DefaultRewardConfig(), "", logger),
		diffBroadcaster: NewStateDiffBroadcaster(10, logger),
		logger:          logger,
	}
	c.initBounds()
	h := &grpcHandler{coord: c}
	c.grpcHandler = h
	return c, h
}

func register(t *testing.T, h *grpcHandler, id string, level pb.TaskLevel) *AgentInfo {
	resp, err := h.Register(context.Background(), &pb.RegisterRequest{AgentID: id, Capability: level})
	require.NoError(t, err)
	require.True(t, resp.Accepted)
	a, ok := h.coord.registry.GetAgent(id)
	require.True(t, ok)
	return a
}

func results(ids ...uint32) []*pb.TaskResult {
	out := make([]*pb.TaskResult, len(ids))
	for i, id := range ids {
		out[i] = &pb.TaskResult{TaskID: id, Valid: true}
	}
	return out
}

func tasksProcessed(c *Coordinator, agentID string) uint64 {
	if w, ok := c.reward.CurrentWork()[agentID]; ok {
		return w.TasksProcessed
	}
	return 0
}

func TestSubmitResultsCountsOnlyDispatchedTasks(t *testing.T) {
	r := require.New(t)
	c, h := newBoundsTestCoordinator(t, "L2", &mockActPool{}, &mockState{})
	alice := register(t, h, "alice", pb.TaskLevel_L2_STATE_VERIFY)
	register(t, h, "mallory", pb.TaskLevel_L2_STATE_VERIFY)

	// three tasks go to alice only (mallory's channel is left full)
	mallory, _ := c.registry.GetAgent("mallory")
	for len(mallory.TaskChan) < cap(mallory.TaskChan) {
		mallory.TaskChan <- &pb.TaskBatch{}
	}
	tasks := []*pb.TaskPackage{{TaskID: 1, Level: pb.TaskLevel_L2_STATE_VERIFY}, {TaskID: 2, Level: pb.TaskLevel_L2_STATE_VERIFY}, {TaskID: 3, Level: pb.TaskLevel_L2_STATE_VERIFY}}
	r.Equal(3, c.scheduler.Dispatch(tasks, 10))
	<-alice.TaskChan

	submit := func(agent string, res []*pb.TaskResult) *pb.SubmitResponse {
		resp, err := h.SubmitResults(context.Background(), &pb.BatchResult{AgentID: agent, Results: res})
		r.NoError(err)
		return resp
	}
	// mallory claims alice's tasks plus fabricated ones: nothing counts
	r.True(submit("mallory", results(1, 2, 3, 1000, 1001)).Accepted)
	r.Zero(tasksProcessed(c, "mallory"))
	// alice's real results count once, fabricated and repeated ones do not
	r.True(submit("alice", results(1, 2, 3, 2, 3, 4000, 4001)).Accepted)
	r.Equal(uint64(3), tasksProcessed(c, "alice"))
	r.True(submit("alice", results(1, 2, 3)).Accepted)
	r.Equal(uint64(3), tasksProcessed(c, "alice"))
	r.Zero(c.outstanding.len())

	// an oversized submission is refused outright
	ids := make([]uint32, maxResultsPerSubmit+1)
	for i := range ids {
		ids[i] = uint32(10000 + i)
	}
	r.False(submit("alice", results(ids...)).Accepted)
}

func TestOutstandingTasksExpire(t *testing.T) {
	r := require.New(t)
	o := newOutstandingTasks()
	o.add("a", &pb.TaskBatch{Tasks: []*pb.TaskPackage{{TaskID: 1}, {TaskID: 2}}})
	o.expire(time.Now().Add(time.Second))
	r.Zero(o.len())
	r.False(o.take("a", 1))
}

func TestSubmitResultsKeepsNoStateForUnregisteredAgents(t *testing.T) {
	r := require.New(t)
	_, h := newBoundsTestCoordinator(t, "L2", &mockActPool{}, &mockState{})
	for i := 0; i < 100; i++ {
		resp, err := h.SubmitResults(context.Background(), &pb.BatchResult{AgentID: fmt.Sprintf("ghost-%d", i)})
		r.NoError(err)
		r.False(resp.Accepted)
	}
	n := 0
	h.rateLimiter.Range(func(any, any) bool { n++; return true })
	r.Zero(n)
}

func TestResultBuffersAreBounded(t *testing.T) {
	r := require.New(t)
	c, h := newBoundsTestCoordinator(t, "L2", &mockActPool{}, &mockState{})
	register(t, h, "alice", pb.TaskLevel_L2_STATE_VERIFY)
	const n = maxRecentResults + 500
	for start := uint32(0); start < n; start += maxResultsPerSubmit {
		batch := &pb.TaskBatch{}
		ids := make([]uint32, 0, maxResultsPerSubmit)
		for id := start; id < start+maxResultsPerSubmit && id < n; id++ {
			batch.Tasks = append(batch.Tasks, &pb.TaskPackage{TaskID: id})
			ids = append(ids, id)
		}
		c.outstanding.add("alice", batch)
		c.handleResults(&pb.BatchResult{AgentID: "alice", Results: results(ids...)})
	}
	r.Equal(uint64(n), tasksProcessed(c, "alice"))
	r.LessOrEqual(c.numRecentResults.Load(), int64(maxRecentResults))
	c.shadow.mu.Lock()
	r.LessOrEqual(len(c.shadow.results), maxShadowResults)
	c.shadow.mu.Unlock()
	r.Len(c.DrainRecentResults(), maxRecentResults)
	r.Zero(c.numRecentResults.Load())
}

func TestPollSkipsWorkWithoutCapableAgents(t *testing.T) {
	r := require.New(t)
	ap := &mockActPool{height: 7}
	for i := 0; i < 20; i++ {
		ap.AddTx(&PendingTx{Hash: fmt.Sprintf("tx-%d", i), From: "io1from", To: fmt.Sprintf("io1to%d", i), GasLimit: 100000})
	}
	sr := &countingState{}
	c, h := newBoundsTestCoordinator(t, "L3", ap, sr)

	// no agents, or only agents below L3: no reads, no simulations
	c.pollAndDispatch()
	register(t, h, "l2", pb.TaskLevel_L2_STATE_VERIFY)
	c.pollAndDispatch()
	r.Zero(sr.reads.Load())
	r.Zero(sr.sims.Load())

	// with an L3 agent each tx is simulated once per tip, however often we poll
	l3 := register(t, h, "l3", pb.TaskLevel_L3_FULL_EXECUTE)
	c.pollAndDispatch()
	for len(l3.TaskChan) > 0 {
		<-l3.TaskChan
	}
	c.pollAndDispatch()
	r.Equal(int64(20), sr.sims.Load())
	ap.mu.Lock()
	ap.height = 8
	ap.mu.Unlock()
	c.pollAndDispatch()
	r.Equal(int64(40), sr.sims.Load())
}

func TestStartRequiresMasterSecret(t *testing.T) {
	c, _ := newBoundsTestCoordinator(t, "L2", &mockActPool{}, &mockState{})
	require.ErrorContains(t, c.Start(context.Background()), "masterSecret")
}

func TestComputePayoutsLargeWeights(t *testing.T) {
	r := require.New(t)
	rd := NewRewardDistributor(DefaultRewardConfig(), "", zap.NewNop())
	// weight*1e9 overflows int64 once weight passes ~9.2e9
	rd.RecordWork("a", 20_000_000_000, 0, 0)
	rd.RecordWork("b", 10_000_000_000, 0, 0)
	pool := new(big.Int).Mul(big.NewInt(800), big.NewInt(1e18))
	summary := rd.computePayouts(pool)
	total := new(big.Int)
	for _, p := range summary.Payouts {
		r.True(p.Amount.Sign() > 0, p.AgentID)
		total.Add(total, p.Amount)
	}
	r.True(total.Cmp(summary.AgentPool) <= 0)
	r.Equal("a", summary.Payouts[0].AgentID)
	r.Equal(summary.Payouts[0].Amount, new(big.Int).Mul(summary.Payouts[1].Amount, big.NewInt(2)))
}

func TestLiveAgentsSnapshotsAreRaceFree(t *testing.T) {
	r := NewRegistry(10)
	r.Register(&pb.RegisterRequest{AgentID: "a", Capability: pb.TaskLevel_L2_STATE_VERIFY})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 1000; i++ {
			r.Heartbeat(&pb.HeartbeatRequest{AgentID: "a", TasksPending: uint32(i)})
			r.Register(&pb.RegisterRequest{AgentID: "a", Capability: pb.TaskLevel_L3_FULL_EXECUTE})
		}
	}()
	for i := 0; i < 1000; i++ {
		for _, a := range r.LiveAgents(time.Minute) {
			_ = a.TasksPending + uint32(a.Capability)
			_ = a.LastHeartbeat
		}
	}
	<-done
}

func TestSchedulerIgnoresSelfReportedPending(t *testing.T) {
	r := require.New(t)
	c, h := newBoundsTestCoordinator(t, "L2", &mockActPool{}, &mockState{})
	busy := register(t, h, "busy", pb.TaskLevel_L2_STATE_VERIFY)
	idle := register(t, h, "idle", pb.TaskLevel_L2_STATE_VERIFY)
	// busy holds 5 unanswered tasks but claims to have none pending
	c.outstanding.add("busy", &pb.TaskBatch{Tasks: []*pb.TaskPackage{{TaskID: 1}, {TaskID: 2}, {TaskID: 3}, {TaskID: 4}, {TaskID: 5}}})
	c.registry.Heartbeat(&pb.HeartbeatRequest{AgentID: "busy", TasksPending: 0})
	c.registry.Heartbeat(&pb.HeartbeatRequest{AgentID: "idle", TasksPending: 1000})
	r.Equal(1, c.scheduler.Dispatch([]*pb.TaskPackage{{TaskID: 9, Level: pb.TaskLevel_L2_STATE_VERIFY}}, 10))
	r.Len(idle.TaskChan, 1)
	r.Len(busy.TaskChan, 0)
}

func TestOutstandingTasksAcceptResultsOnlyForSentBatches(t *testing.T) {
	r := require.New(t)
	batch := &pb.TaskBatch{Tasks: []*pb.TaskPackage{{TaskID: 7}}}
	for _, handedOver := range []bool{true, false} {
		o := newOutstandingTasks()
		answer := make(chan bool, 1)
		sent, tracked := o.send("alice", batch, func() bool {
			// the agent answers while the batch is being handed over; its
			// answer has to wait until the hand-over is settled
			go func() { answer <- o.take("alice", 7) }()
			select {
			case got := <-answer:
				answer <- got
			case <-time.After(50 * time.Millisecond):
			}
			return handedOver
		})
		r.True(tracked)
		r.Equal(handedOver, sent)
		// a fast answer counts for a batch that went out, and never for
		// one that did not
		r.Equal(handedOver, <-answer)
		r.Zero(o.len())
	}
}

func TestDispatchTracksOnlyTheAgentThatGotTheBatch(t *testing.T) {
	r := require.New(t)
	c, h := newBoundsTestCoordinator(t, "L2", &mockActPool{}, &mockState{})
	full := register(t, h, "full", pb.TaskLevel_L2_STATE_VERIFY)
	alice := register(t, h, "alice", pb.TaskLevel_L2_STATE_VERIFY)
	for i := 0; i < cap(full.TaskChan); i++ {
		full.TaskChan <- &pb.TaskBatch{}
	}
	r.Equal(1, c.scheduler.Dispatch([]*pb.TaskPackage{{TaskID: 7, Level: pb.TaskLevel_L2_STATE_VERIFY}}, 10))
	r.Len(alice.TaskChan, 1)
	// the agent whose channel was full cannot claim the task
	r.Zero(c.outstanding.pending("full"))
	c.handleResults(&pb.BatchResult{AgentID: "full", Results: results(7)})
	r.Zero(tasksProcessed(c, "full"))
	c.handleResults(&pb.BatchResult{AgentID: "alice", Results: results(7)})
	r.Equal(uint64(1), tasksProcessed(c, "alice"))
}

func TestDispatchDefersBatchesThatCannotBeTracked(t *testing.T) {
	r := require.New(t)
	c, h := newBoundsTestCoordinator(t, "L2", &mockActPool{}, &mockState{})
	alice := register(t, h, "alice", pb.TaskLevel_L2_STATE_VERIFY)
	batch := &pb.TaskBatch{Tasks: make([]*pb.TaskPackage, maxOutstandingTasks)}
	for i := range batch.Tasks {
		batch.Tasks[i] = &pb.TaskPackage{TaskID: uint32(1_000_000 + i)}
	}
	r.True(c.outstanding.add("slow", batch))
	r.Zero(c.scheduler.Dispatch([]*pb.TaskPackage{{TaskID: 7, Level: pb.TaskLevel_L2_STATE_VERIFY}}, 10))
	r.Len(alice.TaskChan, 0)
	r.Equal(1, c.scheduler.RetryQueueLen())
	r.Equal(maxOutstandingTasks, c.outstanding.len())
	// once the slow agent's tasks expire, the deferred batch goes out
	c.outstanding.expire(time.Now().Add(time.Second))
	r.Equal(1, c.scheduler.Dispatch(nil, 10))
	r.Len(alice.TaskChan, 1)
	r.Equal(1, c.outstanding.pending("alice"))
}
