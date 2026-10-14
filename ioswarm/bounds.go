package ioswarm

import (
	"sync"
	"time"

	pb "github.com/iotexproject/iotex-core/v2/ioswarm/proto"
)

const (
	// maxTasksPerPoll caps how many pending txs one poll turns into tasks.
	maxTasksPerPoll = 1000
	// maxOutstandingTasks caps tasks awaiting a result across all agents.
	maxOutstandingTasks = 100000
	// outstandingTaskTTL is how long a dispatched task can still be answered.
	outstandingTaskTTL = 2 * time.Minute
	// maxResultsPerSubmit caps the results one SubmitResults call may carry.
	maxResultsPerSubmit = 1000
	// maxRecentResults caps the results kept for EVM shadow comparison.
	maxRecentResults = 10000
	// maxShadowResults caps the agent results awaiting a block to compare with.
	maxShadowResults = 10000
	// maxL3CacheEntries caps the per-tip L3 simulation cache.
	maxL3CacheEntries = 10000
)

type outstandingTask struct {
	agentID string
	at      time.Time
}

// outstandingTasks records which agent each dispatched task went to.
type outstandingTasks struct {
	mu       sync.Mutex
	tasks    map[uint32]outstandingTask
	perAgent map[string]int
}

func newOutstandingTasks() *outstandingTasks {
	return &outstandingTasks{tasks: make(map[uint32]outstandingTask), perAgent: make(map[string]int)}
}

func (o *outstandingTasks) drop(id uint32, t outstandingTask) {
	delete(o.tasks, id)
	if o.perAgent[t.agentID]--; o.perAgent[t.agentID] <= 0 {
		delete(o.perAgent, t.agentID)
	}
}

// pending returns how many tasks agentID has been sent and not answered.
func (o *outstandingTasks) pending(agentID string) int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.perAgent[agentID]
}

// add records the tasks of a batch as dispatched to agentID, before the batch
// is sent, so a result that comes back at once is already expected. A task
// that is re-dispatched moves to the new agent. If the batch does not fit,
// nothing is recorded and add returns false: the caller must not send it.
// Otherwise undo puts back what add changed, for a batch that was not sent.
func (o *outstandingTasks) add(agentID string, batch *pb.TaskBatch) (undo func(), ok bool) {
	now := time.Now()
	o.mu.Lock()
	defer o.mu.Unlock()
	added := 0
	for _, t := range batch.Tasks {
		if _, ok := o.tasks[t.TaskID]; !ok {
			added++
		}
	}
	if len(o.tasks)+added > maxOutstandingTasks {
		return nil, false
	}
	prevs := make(map[uint32]outstandingTask)
	for _, t := range batch.Tasks {
		if prev, ok := o.tasks[t.TaskID]; ok {
			if _, seen := prevs[t.TaskID]; !seen {
				prevs[t.TaskID] = prev
			}
			o.drop(t.TaskID, prev)
		}
		o.tasks[t.TaskID] = outstandingTask{agentID: agentID, at: now}
		o.perAgent[agentID]++
	}
	return func() {
		o.mu.Lock()
		defer o.mu.Unlock()
		for _, t := range batch.Tasks {
			cur, ok := o.tasks[t.TaskID]
			if !ok || cur.agentID != agentID || !cur.at.Equal(now) {
				continue
			}
			o.drop(t.TaskID, cur)
			if prev, ok := prevs[t.TaskID]; ok {
				o.tasks[t.TaskID] = prev
				o.perAgent[prev.agentID]++
			}
		}
	}, true
}

// take reports whether taskID was dispatched to agentID and is still
// unanswered, and if so marks it answered.
func (o *outstandingTasks) take(agentID string, taskID uint32) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	t, ok := o.tasks[taskID]
	if !ok || t.agentID != agentID {
		return false
	}
	o.drop(taskID, t)
	return true
}

// expire drops tasks dispatched before cutoff.
func (o *outstandingTasks) expire(cutoff time.Time) {
	o.mu.Lock()
	defer o.mu.Unlock()
	for id, t := range o.tasks {
		if t.at.Before(cutoff) {
			o.drop(id, t)
		}
	}
}

func (o *outstandingTasks) len() int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return len(o.tasks)
}

type l3Simulation struct {
	slots map[string][]string
	err   error
}

// l3EnrichmentCache memoises SimulateAccessList per tx for one tip height.
type l3EnrichmentCache struct {
	mu     sync.Mutex
	height uint64
	sims   map[string]l3Simulation
}

func newL3EnrichmentCache() *l3EnrichmentCache {
	return &l3EnrichmentCache{sims: make(map[string]l3Simulation)}
}

func (c *l3EnrichmentCache) simulate(height uint64, txHash string, run func() (map[string][]string, error)) (map[string][]string, error) {
	c.mu.Lock()
	if c.height != height {
		c.height = height
		c.sims = make(map[string]l3Simulation)
	}
	if s, ok := c.sims[txHash]; ok {
		c.mu.Unlock()
		return s.slots, s.err
	}
	c.mu.Unlock()

	slots, err := run()

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.height == height && len(c.sims) < maxL3CacheEntries {
		c.sims[txHash] = l3Simulation{slots: slots, err: err}
	}
	return slots, err
}
