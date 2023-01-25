// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package node

import (
	"context"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-core/test/mock/mock_node"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/libp2p/go-libp2p-core/peer"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestNewDelegateManager(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	hMock := mock_node.NewMockchain(ctrl)
	tMock := mock_node.NewMocktransmitter(ctrl)
	privK, err := crypto.GenerateKey()
	require.NoError(err)

	t.Run("disable_broadcast", func(t *testing.T) {
		cfg := Config{true, 100 * time.Millisecond}
		dm := NewDelegateManager(&cfg, tMock, hMock, privK)
		require.Equal(cfg, dm.cfg)
		require.Equal(map[string]Info{}, dm.nodeMap)
		require.Equal(tMock, dm.transmitter)
		require.Equal(hMock, dm.chain)
		require.Equal(privK, dm.privKey)
		require.Equal(true, dm.broadcastTask != nil)
		tMock.EXPECT().BroadcastOutbound(gomock.Any(), gomock.Any()).Times(0)
		err := dm.Start(context.Background())
		require.NoError(err)
		defer dm.Stop(context.Background())
		time.Sleep(time.Second)
	})

	t.Run("enable_broadcast", func(t *testing.T) {
		cfg := Config{false, 100 * time.Millisecond}
		dm := NewDelegateManager(&cfg, tMock, hMock, privK)
		require.Equal(cfg, dm.cfg)
		require.Equal(map[string]Info{}, dm.nodeMap)
		require.Equal(tMock, dm.transmitter)
		require.Equal(hMock, dm.chain)
		require.Equal(privK, dm.privKey)
		require.Equal(true, dm.broadcastTask != nil)
		tMock.EXPECT().Info().Return(peer.AddrInfo{}, nil).MinTimes(1)
		hMock.EXPECT().TipHeight().Return(uint64(10)).MinTimes(1)
		tMock.EXPECT().BroadcastOutbound(gomock.Any(), gomock.Any()).Return(nil).MinTimes(1)
		err := dm.Start(context.Background())
		require.NoError(err)
		defer dm.Stop(context.Background())
		time.Sleep(time.Second)
	})
}

func TestDelegateManager_HandleNodeInfo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	hMock := mock_node.NewMockchain(ctrl)
	tMock := mock_node.NewMocktransmitter(ctrl)

	require := require.New(t)
	privKey, err := crypto.GenerateKey()
	require.NoError(err)

	t.Run("verify_pass", func(t *testing.T) {
		msg := &iotextypes.ResponseNodeInfoMessage{
			Info: &iotextypes.NodeInfo{
				Version:   "v1.8.0",
				Height:    200,
				Timestamp: timestamppb.Now(),
				Address:   privKey.PublicKey().Address().String(),
			},
		}
		hash := hashNodeInfo(msg.Info)
		msg.Signature, _ = privKey.Sign(hash[:])
		dm := NewDelegateManager(&DefaultConfig, tMock, hMock, privKey)
		dm.HandleNodeInfo(context.Background(), "abc", msg)
		addr := msg.Info.Address
		nodeGot := dm.nodeMap[addr]
		require.Equal(msg.Info.Height, nodeGot.Height)
		require.Equal(msg.Info.Version, nodeGot.Version)
		require.Equal(msg.Info.Timestamp.AsTime().String(), nodeGot.Timestamp.String())
		require.Equal("abc", nodeGot.PeerID)
		m := dto.Metric{}
		nodeDelegateHeightGauge.WithLabelValues(addr, msg.Info.Version).Write(&m)
		require.Equal(msg.Info.Height, uint64(m.Gauge.GetValue()))
	})

	t.Run("verify_fail", func(t *testing.T) {
		privKey2, _ := crypto.GenerateKey()
		msg := &iotextypes.ResponseNodeInfoMessage{
			Info: &iotextypes.NodeInfo{
				Version:   "v1.8.0",
				Height:    200,
				Timestamp: timestamppb.Now(),
				Address:   privKey2.PublicKey().Address().String(),
			},
			Signature: []byte("xxxx"),
		}
		dm := NewDelegateManager(&DefaultConfig, tMock, hMock, privKey)
		dm.HandleNodeInfo(context.Background(), "abc", msg)
		addr := msg.Info.Address
		_, ok := dm.nodeMap[addr]
		require.False(ok)
		m := dto.Metric{}
		nodeDelegateHeightGauge.WithLabelValues(addr, msg.Info.Version).Write(&m)
		require.Equal(uint64(0), uint64(m.Gauge.GetValue()))
	})
}

func TestDelegateManager_BroadcastNodeInfo(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	hMock := mock_node.NewMockchain(ctrl)
	tMock := mock_node.NewMocktransmitter(ctrl)
	privKey, err := crypto.GenerateKey()
	require.NoError(err)

	t.Run("update_self", func(t *testing.T) {
		dm := NewDelegateManager(&DefaultConfig, tMock, hMock, privKey)
		height := uint64(200)
		peerID, err := peer.IDFromString("12D3KooWF2fns5ZWKbPfx2U1wQDdxoTK2D6HC3ortbSAQYR4BQp4")
		require.NoError(err)
		hMock.EXPECT().TipHeight().Return(height).Times(1)
		tMock.EXPECT().Info().Return(peer.AddrInfo{ID: peerID}, nil).Times(1)
		tMock.EXPECT().BroadcastOutbound(gomock.Any(), gomock.Any()).Return(nil).Times(1)
		err = dm.BroadcastNodeInfo(context.Background())
		require.NoError(err)
		addr := privKey.PublicKey().Address().String()
		nodeGot := dm.nodeMap[addr]
		require.Equal(height, nodeGot.Height)
		require.Equal(dm.myVersion, nodeGot.Version)
		require.Equal(addr, nodeGot.Address)
		require.Equal(peerID.Pretty(), nodeGot.PeerID)
	})
}

func TestDelegateManager_HandleNodeInfoRequest(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	hMock := mock_node.NewMockchain(ctrl)
	tMock := mock_node.NewMocktransmitter(ctrl)
	privKey, err := crypto.GenerateKey()
	require.NoError(err)

	t.Run("unicast", func(t *testing.T) {
		dm := NewDelegateManager(&DefaultConfig, tMock, hMock, privKey)
		height := uint64(200)
		var sig []byte
		message := &iotextypes.ResponseNodeInfoMessage{}
		hMock.EXPECT().TipHeight().Return(height).Times(1)
		tMock.EXPECT().UnicastOutbound(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, peerInfo peer.AddrInfo, msg proto.Message) error {
			*message = *msg.(*iotextypes.ResponseNodeInfoMessage)
			hash := hashNodeInfo(message.Info)
			sig, _ = dm.privKey.Sign(hash[:])
			return nil
		}).Times(1)
		err := dm.HandleNodeInfoRequest(context.Background(), peer.AddrInfo{})
		require.NoError(err)
		require.Equal(message.Info.Height, height)
		require.Equal(message.Info.Version, dm.myVersion)
		require.Equal(message.Info.Address, dm.myAddress)
		require.Equal(message.Signature, sig)
	})
}

func TestDelegateManager_RequestSingleNodeInfoAsync(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	hMock := mock_node.NewMockchain(ctrl)
	tMock := mock_node.NewMocktransmitter(ctrl)
	privKey, err := crypto.GenerateKey()
	require.NoError(err)

	t.Run("request_single", func(t *testing.T) {
		dm := NewDelegateManager(&DefaultConfig, tMock, hMock, privKey)
		var paramPeer peer.AddrInfo
		var paramMsg iotextypes.RequestNodeInfoMessage
		peerID, err := peer.IDFromString("12D3KooWF2fns5ZWKbPfx2U1wQDdxoTK2D6HC3ortbSAQYR4BQp4")
		require.NoError(err)
		targetPeer := peer.AddrInfo{ID: peerID}
		tMock.EXPECT().UnicastOutbound(gomock.Any(), gomock.Any(), gomock.Any()).Do(func(_ context.Context, p peer.AddrInfo, msg proto.Message) {
			paramPeer = p
			paramMsg = *msg.(*iotextypes.RequestNodeInfoMessage)
		}).Times(1)
		dm.RequestSingleNodeInfoAsync(context.Background(), targetPeer)
		require.Equal(targetPeer, paramPeer)
		require.Equal(iotextypes.RequestNodeInfoMessage{}, paramMsg)
	})

}
