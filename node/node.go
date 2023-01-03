// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package node

import (
	"context"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/consensus"
	"github.com/iotexproject/iotex-core/p2p"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/pkg/routine"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

type Node struct {
	Addr    string
	Height  uint64
	Version string
}

// NodeManager manage nodes on the blockchain
type NodeManager interface {
	lifecycle.StartStopper
	UpdateNode(*Node)
	GetNode(addr string) *Node
}

type delegateManager struct {
	cfg         Config
	nodeMap     map[string]*Node
	consensus   consensus.Consensus
	broadcaster lifecycle.StartStopper
	p2pAgent    p2p.Agent
	bc          blockchain.Blockchain
}

func NewDelegateManager(cfg *Config, consensus consensus.Consensus, p2pAgent p2p.Agent, bc blockchain.Blockchain) NodeManager {
	dm := &delegateManager{
		cfg:       *cfg,
		nodeMap:   make(map[string]*Node),
		consensus: consensus,
		p2pAgent:  p2pAgent,
		bc:        bc,
	}
	dm.broadcaster = routine.NewRecurringTask(dm.broadcast, cfg.NodeInfoBroadcastInterval)
	return dm
}

func (dm *delegateManager) Start(ctx context.Context) error {
	return dm.broadcaster.Start(ctx)
}
func (dm *delegateManager) Stop(ctx context.Context) error {
	return dm.broadcaster.Stop(ctx)
}
func (dm *delegateManager) UpdateNode(node *Node) {
	// update dm.nodeMap
	n := *node
	dm.nodeMap[node.Addr] = &n
	// update metric
	nodeDelegateHeightGauge.WithLabelValues("address", node.Addr, "version", node.Version).Set(float64(node.Height))
}
func (dm *delegateManager) GetNode(addr string) *Node {
	return dm.nodeMap[addr]
}
func (dm *delegateManager) broadcast() {
	dm.p2pAgent.BroadcastOutbound(context.Background(), &iotextypes.NodeInfo{
		Height:  dm.bc.TipHeight(),
		Version: version.PackageVersion,
	})
}
