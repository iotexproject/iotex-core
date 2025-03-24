// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package nodeinfo

import (
	"context"
	"slices"
	"sync/atomic"
	"time"

	"github.com/iotexproject/go-pkgs/cache/lru"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/iotexproject/iotex-core/v2/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/pkg/routine"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/v2/pkg/version"
)

type (
	transmitter interface {
		BroadcastOutbound(context.Context, proto.Message) error
		UnicastOutbound(context.Context, peer.AddrInfo, proto.Message) error
		Info() (peer.AddrInfo, error)
	}

	chain interface {
		TipHeight() uint64
	}

	// Info node infomation
	Info struct {
		Version   string
		Height    uint64
		Timestamp time.Time
		Address   string
		PeerID    string
	}

	// InfoManager manage delegate node info
	InfoManager struct {
		lifecycle.Lifecycle
		version              string
		broadcastList        atomic.Value // []string, whitelist to force enable broadcast
		nodeMap              *lru.Cache
		transmitter          transmitter
		chain                chain
		privKeys             map[string]crypto.PrivateKey
		addrs                []string
		getBroadcastListFunc getBroadcastListFunc
	}

	getBroadcastListFunc func() []string
)

var _nodeInfoHeightGauge = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "iotex_node_info_height_gauge",
		Help: "height info of node",
	},
	[]string{"address", "version"},
)

func init() {
	prometheus.MustRegister(_nodeInfoHeightGauge)
}

// NewInfoManager new info manager
func NewInfoManager(cfg *Config, t transmitter, ch chain, broadcastListFunc getBroadcastListFunc, privKeys ...crypto.PrivateKey) *InfoManager {
	addrs := make([]string, 0, len(privKeys))
	keyMaps := make(map[string]crypto.PrivateKey)
	for _, privKey := range privKeys {
		addr := privKey.PublicKey().Address().String()
		addrs = append(addrs, addr)
		keyMaps[addr] = privKey
	}
	dm := &InfoManager{
		nodeMap:              lru.New(cfg.NodeMapSize),
		transmitter:          t,
		chain:                ch,
		addrs:                addrs,
		privKeys:             keyMaps,
		version:              version.PackageVersion,
		getBroadcastListFunc: broadcastListFunc,
	}
	dm.broadcastList.Store([]string{})
	// init recurring tasks
	broadcastTask := routine.NewRecurringTask(func() {
		addrs := dm.addrs
		if !cfg.EnableBroadcastNodeInfo {
			addrs = dm.inBroadcastList()
		}
		// broadcastlist or nodes who are turned on will broadcast
		if len(addrs) > 0 {
			if err := dm.BroadcastNodeInfo(context.Background(), addrs); err != nil {
				log.L().Error("nodeinfo manager broadcast node info failed", zap.Error(err))
			}
		} else {
			log.L().Debug("nodeinfo manager general node disabled node info broadcast")
		}
	}, cfg.BroadcastNodeInfoInterval)
	updateBroadcastListTask := routine.NewRecurringTask(func() {
		dm.updateBroadcastList()
	}, cfg.BroadcastListTTL)
	dm.AddModels(updateBroadcastListTask, broadcastTask)
	return dm
}

// Start start delegate broadcast task
func (dm *InfoManager) Start(ctx context.Context) error {
	dm.updateBroadcastList()
	return dm.OnStart(ctx)
}

// Stop stop delegate broadcast task
func (dm *InfoManager) Stop(ctx context.Context) error {
	return dm.OnStop(ctx)
}

// HandleNodeInfo handle node info message
func (dm *InfoManager) HandleNodeInfo(ctx context.Context, peerID string, msg *iotextypes.NodeInfo) {
	log.L().Debug("nodeinfo manager handle node info")
	// recover pubkey
	hash := hashNodeInfo(msg.Info)
	pubKey, err := crypto.RecoverPubkey(hash[:], msg.Signature)
	if err != nil {
		log.L().Warn("nodeinfo manager recover pubkey failed", zap.Error(err))
		return
	}
	// verify signature
	if addr := pubKey.Address().String(); addr != msg.Info.Address {
		log.L().Warn("nodeinfo manager node info message verify failed", zap.String("expected", addr), zap.String("recieved", msg.Info.Address))
		return
	}

	dm.updateNode(&Info{
		Version:   msg.Info.Version,
		Height:    msg.Info.Height,
		Timestamp: msg.Info.Timestamp.AsTime(),
		Address:   msg.Info.Address,
		PeerID:    peerID,
	})
}

// updateNode update node info
func (dm *InfoManager) updateNode(node *Info) {
	addr := node.Address
	// update dm.nodeMap
	dm.nodeMap.Add(addr, *node)
	// update metric
	_nodeInfoHeightGauge.WithLabelValues(addr, node.Version).Set(float64(node.Height))
}

// GetNodeInfo get node info by address
func (dm *InfoManager) GetNodeInfo(addr string) (Info, bool) {
	info, ok := dm.nodeMap.Get(addr)
	if !ok {
		return Info{}, false
	}
	return info.(Info), true
}

// BroadcastNodeInfo broadcast request node info message
func (dm *InfoManager) BroadcastNodeInfo(ctx context.Context, addrs []string) error {
	log.L().Debug("nodeinfo manager broadcast node info")
	infos, err := dm.genNodeInfoMsg(addrs)
	if err != nil {
		return err
	}
	for _, info := range infos {
		// broadcast request meesage
		if err := dm.transmitter.BroadcastOutbound(ctx, info); err != nil {
			return err
		}
		// manually update self node info for broadcast message to myself will be ignored
		peer, err := dm.transmitter.Info()
		if err != nil {
			return err
		}
		dm.updateNode(&Info{
			Version:   info.Info.Version,
			Height:    info.Info.Height,
			Timestamp: info.Info.Timestamp.AsTime(),
			Address:   info.Info.Address,
			PeerID:    peer.ID.String(),
		})
	}
	return nil
}

// RequestSingleNodeInfoAsync unicast request node info message
func (dm *InfoManager) RequestSingleNodeInfoAsync(ctx context.Context, peer peer.AddrInfo) error {
	log.L().Debug("nodeinfo manager request one node info", zap.String("peer", peer.ID.String()))
	return dm.transmitter.UnicastOutbound(ctx, peer, &iotextypes.NodeInfoRequest{})
}

// HandleNodeInfoRequest tell node info to peer
func (dm *InfoManager) HandleNodeInfoRequest(ctx context.Context, peer peer.AddrInfo) error {
	log.L().Debug("nodeinfo manager tell node info", zap.Any("peer", peer.ID.String()))
	infos, err := dm.genNodeInfoMsg(dm.addrs)
	if err != nil {
		return err
	}
	for _, info := range infos {
		if err := dm.transmitter.UnicastOutbound(ctx, peer, info); err != nil {
			return err
		}
	}
	return nil
}

func (dm *InfoManager) genNodeInfoMsg(addrs []string) ([]*iotextypes.NodeInfo, error) {
	infos := make([]*iotextypes.NodeInfo, 0, len(addrs))
	tip := dm.chain.TipHeight()
	ts := timestamppb.Now()

	for _, addr := range addrs {
		privKey, ok := dm.privKeys[addr]
		if !ok {
			return nil, errors.Errorf("private key not found for address %s", addr)
		}
		core := &iotextypes.NodeInfoCore{
			Version:   dm.version,
			Height:    tip,
			Timestamp: ts,
			Address:   addr,
		}
		// add sig for msg
		h := hashNodeInfo(core)
		sig, err := privKey.Sign(h[:])
		if err != nil {
			return nil, errors.Wrap(err, "sign node info message failed")
		}
		infos = append(infos, &iotextypes.NodeInfo{
			Info:      core,
			Signature: sig,
		})
	}
	return infos, nil
}

func (dm *InfoManager) inBroadcastList() []string {
	list := dm.broadcastList.Load().([]string)
	inList := make([]string, 0, len(dm.addrs))
	for _, a := range dm.addrs {
		if slices.Contains(list, a) {
			inList = append(inList, a)
		}
	}
	return inList
}

func (dm *InfoManager) updateBroadcastList() {
	if dm.getBroadcastListFunc != nil {
		list := dm.getBroadcastListFunc()
		dm.broadcastList.Store(list)
		log.L().Debug("nodeinfo manaager updateBroadcastList", zap.Strings("list", list))
	}
}

func hashNodeInfo(msg *iotextypes.NodeInfoCore) hash.Hash256 {
	return hash.Hash256b(byteutil.Must(proto.Marshal(msg)))
}
