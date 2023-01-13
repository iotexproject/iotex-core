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
	}

	heightable interface {
		TipHeight() uint64
	}

	privateKey interface {
		PublicKey() crypto.PublicKey
		Sign([]byte) ([]byte, error)
	}

	// DelegateManager manage delegate node info
	DelegateManager struct {
		cfg         Config
		nodeMap     map[string]iotextypes.NodeInfo
		broadcaster *routine.RecurringTask
		transmitter transmitter
		heightable  heightable
		privKey     privateKey
	}
)

// NewDelegateManager new delegate manager
func NewDelegateManager(cfg *Config, t transmitter, h heightable, privKey privateKey) *DelegateManager {
	dm := &DelegateManager{
		cfg:         *cfg,
		nodeMap:     make(map[string]iotextypes.NodeInfo),
		transmitter: t,
		heightable:  h,
		privKey:     privKey,
	}
	// disable broadcast if RequestNodeInfoInterval == 0
	if cfg.RequestNodeInfoInterval > 0 {
		dm.broadcaster = routine.NewRecurringTask(dm.RequestNodeInfo, cfg.RequestNodeInfoInterval)
	}

	return dm
}

// Start start delegate broadcast task
func (dm *DelegateManager) Start(ctx context.Context) error {
	if dm.broadcaster != nil {
		return dm.broadcaster.Start(ctx)
	}
	return nil
}

// Stop stop delegate broadcast task
func (dm *DelegateManager) Stop(ctx context.Context) error {
	if dm.broadcaster != nil {
		return dm.broadcaster.Stop(ctx)
	}
	return nil
}

// HandleNodeInfo handle node info message
func (dm *DelegateManager) HandleNodeInfo(ctx context.Context, peerID string, msg *iotextypes.ResponseNodeInfoMessage) {
	// recover pubkey
	hash := hashNodeInfo(msg)
	pubKey, err := crypto.RecoverPubkey(hash[:], msg.Signature)
	if err != nil {
		log.L().Warn("delegate manager recover pubkey failed", zap.Error(err))
		return
	}
	// verify signature
	if pubKey.Address().String() != msg.Info.Address {
		log.L().Warn("delegate manager node info message verify failed", zap.String("address", msg.Info.Address))
		return
	}

	dm.updateNode(msg.Info.Address, msg.Info)
}

// updateNode update node info
func (dm *DelegateManager) updateNode(addr string, node *iotextypes.NodeInfo) {
	// update dm.nodeMap
	dm.nodeMap[addr] = *node
	// update metric
	nodeDelegateHeightGauge.WithLabelValues(addr, node.Version).Set(float64(node.Height))
}

// RequestNodeInfo broadcast request node info message
func (dm *DelegateManager) RequestNodeInfo() {
	log.L().Info("delegate manager request node info")

	// broadcast request meesage
	if err := dm.transmitter.BroadcastOutbound(context.Background(), &iotextypes.RequestNodeInfoMessage{}); err != nil {
		log.L().Error("delegate manager request node info failed", zap.Error(err))
	}

	// manually update self node info for broadcast message to myself will be ignored
	dm.updateNode(dm.privKey.PublicKey().Address().String(), &iotextypes.NodeInfo{
		Version:   version.PackageVersion,
		Height:    dm.heightable.TipHeight(),
		Timestamp: timestamppb.Now(),
	})
}

// TellNodeInfo tell node info to peer
func (dm *DelegateManager) TellNodeInfo(ctx context.Context, peer peer.AddrInfo) error {
	log.L().Info("delegate manager tell node info", zap.Any("peer", peer.ID.Pretty()))
	req := &iotextypes.ResponseNodeInfoMessage{
		Info: &iotextypes.NodeInfo{
			Version:   version.PackageVersion,
			Height:    dm.heightable.TipHeight(),
			Timestamp: timestamppb.Now(),
			Address:   dm.privKey.PublicKey().Address().String(),
		},
	}
	// add sign for msg
	h := hashNodeInfo(req)
	sign, err := dm.privKey.Sign(h[:])
	if err != nil {
		return err
	}
	req.Signature = sign

	return dm.transmitter.UnicastOutbound(ctx, peer, req)
}

func hashNodeInfo(msg *iotextypes.ResponseNodeInfoMessage) hash.Hash256 {
	return hash.Hash256b(byteutil.Must(proto.Marshal(msg.Info)))
}
