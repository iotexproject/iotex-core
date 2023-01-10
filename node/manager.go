// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package node

import (
	"context"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/routine"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/libp2p/go-libp2p-core/peer"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type (
	transmitter interface {
		BroadcastOutbound(context.Context, proto.Message) error
		UnicastOutbound(context.Context, peer.AddrInfo, proto.Message) error
		Info() (peer.AddrInfo, error)
	}

	heightable interface {
		TipHeight() uint64
	}

	signer interface {
		Sign([]byte) ([]byte, error)
	}

	// DelegateManager manage delegate node info
	DelegateManager struct {
		cfg         Config
		nodeMap     map[string]iotextypes.NodeInfo
		broadcaster *routine.RecurringTask
		transmitter transmitter
		heightable  heightable
		signer      signer
	}
)

// NewDelegateManager new delegate manager
func NewDelegateManager(cfg *Config, t transmitter, h heightable, s signer) *DelegateManager {
	dm := &DelegateManager{
		cfg:         *cfg,
		nodeMap:     make(map[string]iotextypes.NodeInfo),
		transmitter: t,
		heightable:  h,
		signer:      s,
	}
	dm.broadcaster = routine.NewRecurringTask(dm.RequestNodeInfo, cfg.RequestNodeInfoInterval)
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

// HandleNodeInfo handle node info message
func (dm *DelegateManager) HandleNodeInfo(ctx context.Context, addr string, node *iotextypes.ResponseNodeInfoMessage) {
	// TODO sign verify
	// if !dm.verifyNodeInfo(addr, node) {
	// 	log.L().Warn("node info message verify failed", zap.String("addr", addr))
	// 	return
	// }
	dm.UpdateNode(addr, node.Info)
}

// UpdateNode update node info
func (dm *DelegateManager) UpdateNode(addr string, node *iotextypes.NodeInfo) {
	// update dm.nodeMap
	dm.nodeMap[addr] = *node
	// update metric
	nodeDelegateHeightGauge.WithLabelValues(addr, node.Version).Set(float64(node.Height))
}

// RequestNodeInfo broadcast request node info message
func (dm *DelegateManager) RequestNodeInfo() {
	log.L().Info("delegateManager request node info")
	if err := dm.transmitter.BroadcastOutbound(context.Background(), &iotextypes.RequestNodeInfoMessage{}); err != nil {
		log.L().Error("delegateManager request node info failed", zap.Error(err))
	}

	// manually update self node info for broadcast message to myself will be ignored
	peer, err := dm.transmitter.Info()
	if err != nil {
		log.L().Error("delegateManager get self info failed", zap.Error(err))
		return
	}
	msg := &iotextypes.NodeInfo{
		Version:   version.PackageVersion,
		Height:    dm.heightable.TipHeight(),
		Timestamp: timestamppb.Now(),
	}
	dm.UpdateNode(peer.ID.Pretty(), msg)
}

// TellNodeInfo tell node info to peer
func (dm *DelegateManager) TellNodeInfo(ctx context.Context, peer peer.AddrInfo) error {
	log.L().Info("delegateManager tell node info", zap.Any("peer", peer.ID.Pretty()))
	req := &iotextypes.ResponseNodeInfoMessage{
		Info: &iotextypes.NodeInfo{
			Version:   version.PackageVersion,
			Height:    dm.heightable.TipHeight(),
			Timestamp: timestamppb.Now(),
		},
	}
	// add sign for msg
	sign, err := dm.signNodeInfo(req)
	if err != nil {
		return err
	}
	req.Signature = sign

	return dm.transmitter.UnicastOutbound(ctx, peer, req)
}

func (dm *DelegateManager) signNodeInfo(msg *iotextypes.ResponseNodeInfoMessage) ([]byte, error) {
	h := dm.hashNodeInfo(msg)
	sign, err := dm.signer.Sign(h[:])
	if err != nil {
		return nil, err
	}
	return sign, nil
}

func (dm *DelegateManager) hashNodeInfo(msg *iotextypes.ResponseNodeInfoMessage) hash.Hash256 {
	return hash.Hash256b(byteutil.Must(proto.Marshal(msg.Info)))
}

func (dm *DelegateManager) verifyNodeInfo(peer string, msg *iotextypes.ResponseNodeInfoMessage) bool {
	pk := []byte{}
	hash := dm.hashNodeInfo(msg)
	pubK, err := crypto.BytesToPublicKey(pk)
	if err != nil {
		log.L().Warn("convert bytes to publickey failed", zap.Error(err))
		return false
	}

	return pubK.Verify(hash[:], msg.Signature)
}