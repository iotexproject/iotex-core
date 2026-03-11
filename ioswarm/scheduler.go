package ioswarm

import (
	"sort"
	"sync"
	"time"

	pb "github.com/iotexproject/iotex-core/v2/ioswarm/proto"
	"go.uber.org/zap"
)

const maxRetryQueue = 1000

// Scheduler assigns task batches to agents using least-loaded dispatch
// with capability filtering and a bounded retry queue.
type Scheduler struct {
	registry   *Registry
	logger     *zap.Logger
	mu         sync.Mutex
	retryQueue []*pb.TaskBatch
}

// NewScheduler creates a new task scheduler.
func NewScheduler(registry *Registry, logger *zap.Logger) *Scheduler {
	return &Scheduler{
		registry: registry,
		logger:   logger,
	}
}

// Dispatch sends tasks to available agents using least-loaded selection.
// Tasks are split into batches and dispatched with capability filtering.
// Returns the number of tasks dispatched.
func (s *Scheduler) Dispatch(tasks []*pb.TaskPackage, batchSize int) int {
	if batchSize <= 0 {
		batchSize = 10
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Build batches from new tasks
	var newBatches []*pb.TaskBatch
	batchNum := 0
	for i := 0; i < len(tasks); i += batchSize {
		end := i + batchSize
		if end > len(tasks) {
			end = len(tasks)
		}
		newBatches = append(newBatches, &pb.TaskBatch{
			BatchID:   generateBatchID(batchNum),
			Tasks:     tasks[i:end],
			Timestamp: uint64(time.Now().UnixMilli()),
		})
		batchNum++
	}

	// Prepend retry queue items (drain retries first)
	allBatches := make([]*pb.TaskBatch, 0, len(s.retryQueue)+len(newBatches))
	allBatches = append(allBatches, s.retryQueue...)
	allBatches = append(allBatches, newBatches...)
	s.retryQueue = s.retryQueue[:0]

	dispatched := 0
	for _, batch := range allBatches {
		if s.dispatchBatch(batch) {
			dispatched += len(batch.Tasks)
		} else {
			s.enqueueRetry(batch)
		}
	}

	return dispatched
}

// RetryQueueLen returns the number of batches in the retry queue. For testing.
func (s *Scheduler) RetryQueueLen() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.retryQueue)
}

// dispatchBatch tries to send a batch to the least-loaded capable agent.
// Falls back through agents sorted by load. Returns true if dispatched.
func (s *Scheduler) dispatchBatch(batch *pb.TaskBatch) bool {
	// Determine required task level from the batch
	requiredLevel := pb.TaskLevel_L1_SIG_VERIFY
	for _, task := range batch.Tasks {
		if task.Level > requiredLevel {
			requiredLevel = task.Level
		}
	}

	agents := s.registry.LiveAgents(30 * time.Second)
	if len(agents) == 0 {
		return false
	}

	// Filter by capability and sort by load (least-loaded first)
	capable := filterAndSortByLoad(agents, requiredLevel)
	if len(capable) == 0 {
		s.logger.Debug("no capable agents for task level",
			zap.Int32("level", int32(requiredLevel)))
		return false
	}

	// Try each agent in order of increasing load
	for _, agent := range capable {
		select {
		case agent.TaskChan <- batch:
			s.logger.Debug("dispatched batch",
				zap.String("agent", agent.ID),
				zap.String("batch", batch.BatchID),
				zap.Int("tasks", len(batch.Tasks)))
			return true
		default:
			// Channel full, try next agent
			continue
		}
	}

	return false
}

// enqueueRetry adds a batch to the retry queue if there's space.
func (s *Scheduler) enqueueRetry(batch *pb.TaskBatch) {
	if len(s.retryQueue) >= maxRetryQueue {
		s.logger.Warn("retry queue full, dropping batch",
			zap.String("batch", batch.BatchID),
			zap.Int("tasks", len(batch.Tasks)))
		return
	}
	s.retryQueue = append(s.retryQueue, batch)
}

// filterAndSortByLoad returns agents capable of handling the given task level,
// sorted by estimated load (TasksPending + channel queue length).
func filterAndSortByLoad(agents []*AgentInfo, requiredLevel pb.TaskLevel) []*AgentInfo {
	var capable []*AgentInfo
	for _, a := range agents {
		if a.Capability >= requiredLevel {
			capable = append(capable, a)
		}
	}

	sort.Slice(capable, func(i, j int) bool {
		loadI := int(capable[i].TasksPending) + len(capable[i].TaskChan)
		loadJ := int(capable[j].TasksPending) + len(capable[j].TaskChan)
		return loadI < loadJ
	})

	return capable
}

func generateBatchID(seq int) string {
	return time.Now().Format("20060102-150405") + "-" + itoa(seq)
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(buf[pos:])
}
