// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package node

import (
	"context"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/p2p"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/pkg/routine"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/libp2p/go-libp2p-core/peer"
	"google.golang.org/protobuf/types/known/timestamppb"
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
	GetNode(addr string) Node
	TellNodeInfo(ctx context.Context, peer peer.AddrInfo) error
}

type delegateManager struct {
	cfg         Config
	nodeMap     map[string]*Node
	broadcaster lifecycle.StartStopper
	p2pAgent    p2p.Agent
	bc          blockchain.Blockchain
}

// NewDelegateManager new delegate manager
func NewDelegateManager(cfg *Config, p2pAgent p2p.Agent, bc blockchain.Blockchain) NodeManager {
	dm := &delegateManager{
		cfg:      *cfg,
		nodeMap:  make(map[string]*Node),
		p2pAgent: p2pAgent,
		bc:       bc,
	}
	dm.broadcaster = routine.NewRecurringTask(dm.requestNodeInfo, cfg.RequestNodeInfoInterval)
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

func (dm *delegateManager) GetNode(addr string) Node {
	return *dm.nodeMap[addr]
}

func (dm *delegateManager) requestNodeInfo() {
	req := &iotextypes.RequestNodeInfoMessage{
		Timestamp: timestamppb.Now(),
	}
	// TODO: add sign for msg
	dm.p2pAgent.BroadcastOutbound(context.Background(), req)
}

func (dm *delegateManager) TellNodeInfo(ctx context.Context, peer peer.AddrInfo) error {
	req := &iotextypes.ResponseNodeInfoMessage{
		Info: &iotextypes.NodeInfo{
			Version:   version.PackageVersion,
			Height:    dm.bc.TipHeight(),
			Timestamp: timestamppb.Now(),
		},
	}
	// TODO: add sign for msg
	return dm.p2pAgent.UnicastOutbound(ctx, peer, req)
}
