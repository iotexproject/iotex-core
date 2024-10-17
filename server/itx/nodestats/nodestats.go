package nodestats

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/iotexproject/iotex-core/v2/pkg/routine"
)

const (
	// PeriodicReportInterval is the interval for generating periodic reports
	PeriodicReportInterval = 5 * time.Minute
)

// StatsReporter is the interface for stats reporter
type StatsReporter interface {
	BuildReport() string
}

// NodeStats is the struct for getting node stats
type NodeStats struct {
	list []StatsReporter
	task *routine.RecurringTask
}

// NewNodeStats creates a new NodeStats
func NewNodeStats(stats ...StatsReporter) *NodeStats {
	return &NodeStats{
		list: append(stats, newSystemStats()),
	}
}

// Start starts the node stats
func (s *NodeStats) Start(ctx context.Context) error {
	s.task = routine.NewRecurringTask(s.generateReport, PeriodicReportInterval)
	return s.task.Start(ctx)
}

// Stop stops the node stats
func (s *NodeStats) Stop(ctx context.Context) error {
	return s.task.Stop(ctx)
}

func (s *NodeStats) generateReport() {
	stringBuilder := strings.Builder{}
	for _, stat := range s.list {
		stringBuilder.WriteString(stat.BuildReport())
		stringBuilder.WriteString("\n")
	}
	fmt.Println(stringBuilder.String())
}
