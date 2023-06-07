package nodestats

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/iotexproject/iotex-core/blocksync"
	"github.com/iotexproject/iotex-core/p2p"
	"github.com/iotexproject/iotex-core/pkg/routine"
)

const (
	// PeriodicReportInterval is the interval for generating periodic reports
	PeriodicReportInterval = 5 * time.Minute
)

// NodeStats is the struct for getting node stats
type NodeStats struct {
	api       *APILocalStats
	system    *systemStats
	blocksync blocksync.BlockSync
	p2pAgent  p2p.Agent
	task      *routine.RecurringTask
}

// NewNodeStats creates a new NodeStats
func NewNodeStats(apiStats *APILocalStats, bs blocksync.BlockSync, p2pAgent p2p.Agent) *NodeStats {
	return &NodeStats{
		api:       apiStats,
		blocksync: bs,
		p2pAgent:  p2pAgent,
		system:    newSystemStats(),
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
	stringBuilder.WriteString(s.api.BuildReport())
	stringBuilder.WriteString("\n")

	stringBuilder.WriteString(s.system.BuildReport())
	stringBuilder.WriteString("\n")

	startingHeight, currentHeight, targetHeight, statusString := s.blocksync.SyncStatus()
	stringBuilder.WriteString("BlockSync: ")
	stringBuilder.WriteString(statusString)
	stringBuilder.WriteString(" StartingHeight: ")
	stringBuilder.WriteString(strconv.FormatUint(startingHeight, 10))
	stringBuilder.WriteString(" CurrentHeight: ")
	stringBuilder.WriteString(strconv.FormatUint(currentHeight, 10))
	stringBuilder.WriteString(" TargetHeight: ")
	stringBuilder.WriteString(strconv.FormatUint(targetHeight, 10))
	stringBuilder.WriteString("\n")

	neighbors, err := s.p2pAgent.ConnectedPeers()
	if err == nil {
		stringBuilder.WriteString("P2P ConnectedPeers: ")
		stringBuilder.WriteString(strconv.Itoa(len(neighbors)))
		stringBuilder.WriteString("\n")
	}
	fmt.Println(stringBuilder.String())
}
