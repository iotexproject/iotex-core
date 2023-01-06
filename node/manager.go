// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package node

import (
	"context"

	"github.com/iotexproject/iotex-core/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/pkg/routine"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/libp2p/go-libp2p-core/peer"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type (
	transmitter interface {
		BroadcastOutbound(context.Context, proto.Message) error
		UnicastOutbound(context.Context, peer.AddrInfo, proto.Message) error
	}

	heightable interface {
		TipHeight() uint64
	}

	// DelegateManager manage delegate node info
	DelegateManager struct {
		cfg         Config
		nodeMap     map[string]iotextypes.NodeInfo
		broadcaster lifecycle.StartStopper
		transmitter transmitter
		heightable  heightable
	}
)

// NewDelegateManager new delegate manager
func NewDelegateManager(cfg *Config, t transmitter, h heightable) *DelegateManager {
	dm := &DelegateManager{
		cfg:         *cfg,
		nodeMap:     make(map[string]iotextypes.NodeInfo),
		transmitter: t,
		heightable:  h,
	}
	dm.broadcaster = routine.NewRecurringTask(dm.requestNodeInfo, cfg.RequestNodeInfoInterval)
	return dm
}

// Start start delegate broadcast task
func (dm *DelegateManager) Start(ctx context.Context) error {
	return dm.broadcaster.Start(ctx)
}

// Stop stop delegate broadcast task
func (dm *DelegateManager) Stop(ctx context.Context) error {
	return dm.broadcaster.Stop(ctx)
}

// UpdateNode update node info
func (dm *DelegateManager) UpdateNode(addr string, node *iotextypes.NodeInfo) {
	// update dm.nodeMap
	dm.nodeMap[addr] = *node
	// update metric
	nodeDelegateHeightGauge.WithLabelValues(addr, node.Version).Set(float64(node.Height))
}

func (dm *DelegateManager) requestNodeInfo() {
	req := &iotextypes.RequestNodeInfoMessage{
		Timestamp: timestamppb.Now(),
	}
	// TODO: add sign for msg
	dm.transmitter.BroadcastOutbound(context.Background(), req)
}

// TellNodeInfo tell node info to peer
func (dm *DelegateManager) TellNodeInfo(ctx context.Context, peer peer.AddrInfo) error {
	req := &iotextypes.ResponseNodeInfoMessage{
		Info: &iotextypes.NodeInfo{
			Version:   version.PackageVersion,
			Height:    dm.heightable.TipHeight(),
			Timestamp: timestamppb.Now(),
		},
	}
	// TODO: add sign for msg
	return dm.transmitter.UnicastOutbound(ctx, peer, req)
}
