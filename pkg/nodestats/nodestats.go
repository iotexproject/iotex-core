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

type nodeStats struct {
	rpc       IRPCLocalStats
	system    SytemStats
	blocksync blocksync.BlockSync
	p2pAgent  p2p.Agent
	task      *routine.RecurringTask
}

func NewNodeStats(rpc IRPCLocalStats, bs blocksync.BlockSync, p2pAgent p2p.Agent) NodeStats {
	return &nodeStats{
		rpc:       rpc,
		blocksync: bs,
		p2pAgent:  p2pAgent,
		system:    newSystemStats(),
	}
}

func (s *nodeStats) Start(ctx context.Context) error {
	s.task = routine.NewRecurringTask(s.generateReport, time.Second*10)
	return s.task.Start(ctx)
}

func (s *nodeStats) Stop(ctx context.Context) error {
	return s.task.Stop(ctx)
}

func (s *nodeStats) generateReport() {
	stringBuilder := strings.Builder{}
	stringBuilder.WriteString(s.rpc.BuildReport())
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
