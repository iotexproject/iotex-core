// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package nodeinfo

import (
	"context"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/libp2p/go-libp2p/core/peer"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/iotexproject/iotex-core/v2/test/mock/mock_nodeinfo"
)

func getEmptyWhiteList() []string {
	return []string{}
}

func TestNewDelegateManager(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	privK, err := crypto.GenerateKey()
	nodeAddr := privK.PublicKey().Address().String()
	require.NoError(err)

	t.Run("disable_broadcast", func(t *testing.T) {
		hMock := mock_nodeinfo.NewMockchain(ctrl)
		tMock := mock_nodeinfo.NewMocktransmitter(ctrl)
		cfg := Config{false, 100 * time.Millisecond, 100 * time.Millisecond, 1000}
		dm := NewInfoManager(&cfg, tMock, hMock, getEmptyWhiteList, privK)
		require.NotNil(dm.nodeMap)
		require.Equal(tMock, dm.transmitter)
		require.Equal(hMock, dm.chain)
		require.Equal(privK, dm.privKeys[nodeAddr])
		tMock.EXPECT().BroadcastOutbound(gomock.Any(), gomock.Any()).Times(0)
		hMock.EXPECT().TipHeight().Return(uint64(2)).Times(0)
		err := dm.Start(context.Background())
		require.NoError(err)
		defer dm.Stop(context.Background())
		time.Sleep(time.Second)
	})

	t.Run("enable_broadcast", func(t *testing.T) {
		hMock := mock_nodeinfo.NewMockchain(ctrl)
		tMock := mock_nodeinfo.NewMocktransmitter(ctrl)
		cfg := Config{true, 100 * time.Millisecond, 100 * time.Millisecond, 1000}
		dm := NewInfoManager(&cfg, tMock, hMock, getEmptyWhiteList, privK)
		require.NotNil(dm.nodeMap)
		require.Equal(tMock, dm.transmitter)
		require.Equal(hMock, dm.chain)
		require.Equal(privK, dm.privKeys[nodeAddr])
		tMock.EXPECT().Info().Return(peer.AddrInfo{}, nil).MinTimes(1)
		hMock.EXPECT().TipHeight().Return(uint64(10)).MinTimes(1)
		tMock.EXPECT().BroadcastOutbound(gomock.Any(), gomock.Any()).Return(nil).MinTimes(1)
		err := dm.Start(context.Background())
		require.NoError(err)
		defer dm.Stop(context.Background())
		time.Sleep(time.Second)
	})
	t.Run("delegate_broadcast", func(t *testing.T) {
		hMock := mock_nodeinfo.NewMockchain(ctrl)
		tMock := mock_nodeinfo.NewMocktransmitter(ctrl)
		cfg := Config{true, 100 * time.Millisecond, 100 * time.Millisecond, 1000}
		dm := NewInfoManager(&cfg, tMock, hMock, getEmptyWhiteList, privK)
		require.NotNil(dm.nodeMap)
		require.Equal(tMock, dm.transmitter)
		require.Equal(hMock, dm.chain)
		require.Equal(privK, dm.privKeys[nodeAddr])
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

	require := require.New(t)
	privKey, err := crypto.GenerateKey()
	require.NoError(err)

	t.Run("verify_pass", func(t *testing.T) {
		hMock := mock_nodeinfo.NewMockchain(ctrl)
		tMock := mock_nodeinfo.NewMocktransmitter(ctrl)
		msg := &iotextypes.NodeInfo{
			Info: &iotextypes.NodeInfoCore{
				Version:   "v1.8.0",
				Height:    200,
				Timestamp: timestamppb.Now(),
				Address:   privKey.PublicKey().Address().String(),
			},
		}
		hash := hashNodeInfo(msg.Info)
		msg.Signature, _ = privKey.Sign(hash[:])
		dm := NewInfoManager(&DefaultConfig, tMock, hMock, getEmptyWhiteList, privKey)
		dm.HandleNodeInfo(context.Background(), "abc", msg)
		addr := msg.Info.Address
		nodeGot, ok := dm.nodeMap.Get(addr)
		require.True(ok)
		nodeInfo := nodeGot.(Info)
		require.Equal(msg.Info.Height, nodeInfo.Height)
		require.Equal(msg.Info.Version, nodeInfo.Version)
		require.Equal(msg.Info.Timestamp.AsTime().String(), nodeInfo.Timestamp.String())
		require.Equal("abc", nodeInfo.PeerID)
		m := dto.Metric{}
		_nodeInfoHeightGauge.WithLabelValues(addr, msg.Info.Version).Write(&m)
		require.Equal(msg.Info.Height, uint64(m.Gauge.GetValue()))
	})

	t.Run("verify_fail", func(t *testing.T) {
		hMock := mock_nodeinfo.NewMockchain(ctrl)
		tMock := mock_nodeinfo.NewMocktransmitter(ctrl)
		privKey2, _ := crypto.GenerateKey()
		msg := &iotextypes.NodeInfo{
			Info: &iotextypes.NodeInfoCore{
				Version:   "v1.8.0",
				Height:    200,
				Timestamp: timestamppb.Now(),
				Address:   privKey2.PublicKey().Address().String(),
			},
			Signature: []byte("xxxx"),
		}
		dm := NewInfoManager(&DefaultConfig, tMock, hMock, getEmptyWhiteList, privKey)
		dm.HandleNodeInfo(context.Background(), "abc", msg)
		addr := msg.Info.Address
		_, ok := dm.nodeMap.Get(addr)
		require.False(ok)
		m := dto.Metric{}
		_nodeInfoHeightGauge.WithLabelValues(addr, msg.Info.Version).Write(&m)
		require.Equal(uint64(0), uint64(m.Gauge.GetValue()))
	})
}

func TestDelegateManager_BroadcastNodeInfo(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	privKey, err := crypto.GenerateKey()
	nodeAddr := privKey.PublicKey().Address().String()
	require.NoError(err)

	t.Run("update_self", func(t *testing.T) {
		hMock := mock_nodeinfo.NewMockchain(ctrl)
		tMock := mock_nodeinfo.NewMocktransmitter(ctrl)
		dm := NewInfoManager(&DefaultConfig, tMock, hMock, getEmptyWhiteList, privKey)
		height := uint64(200)
		peerID, err := peer.IDFromBytes([]byte("12D3KooWF2fns5ZWKbPfx2U1wQDdxoTK2D6HC3ortbSAQYR4BQp4"))
		require.NoError(err)
		hMock.EXPECT().TipHeight().Return(height).Times(1)
		tMock.EXPECT().Info().Return(peer.AddrInfo{ID: peerID}, nil).Times(1)
		tMock.EXPECT().BroadcastOutbound(gomock.Any(), gomock.Any()).Return(nil).Times(1)
		err = dm.BroadcastNodeInfo(context.Background(), []string{nodeAddr})
		require.NoError(err)
		addr := privKey.PublicKey().Address().String()
		nodeGot, ok := dm.nodeMap.Get(addr)
		require.True(ok)
		nodeInfo := nodeGot.(Info)
		require.Equal(height, nodeInfo.Height)
		require.Equal(dm.version, nodeInfo.Version)
		require.Equal(addr, nodeInfo.Address)
		require.Equal(peerID.String(), nodeInfo.PeerID)
	})
}

func TestDelegateManager_HandleNodeInfoRequest(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	privKey, err := crypto.GenerateKey()
	nodeAddr := privKey.PublicKey().Address().String()
	require.NoError(err)

	t.Run("unicast", func(t *testing.T) {
		hMock := mock_nodeinfo.NewMockchain(ctrl)
		tMock := mock_nodeinfo.NewMocktransmitter(ctrl)
		dm := NewInfoManager(&DefaultConfig, tMock, hMock, getEmptyWhiteList, privKey)
		height := uint64(200)
		var sig []byte
		message := &iotextypes.NodeInfo{}
		hMock.EXPECT().TipHeight().Return(height).Times(1)
		tMock.EXPECT().UnicastOutbound(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, peerInfo peer.AddrInfo, msg proto.Message) error {
			message = msg.(*iotextypes.NodeInfo)
			hash := hashNodeInfo(message.Info)
			sig, _ = dm.privKeys[nodeAddr].Sign(hash[:])
			return nil
		}).Times(1)
		err := dm.HandleNodeInfoRequest(context.Background(), peer.AddrInfo{})
		require.NoError(err)
		require.Equal(message.Info.Height, height)
		require.Equal(message.Info.Version, dm.version)
		require.Equal(message.Info.Address, nodeAddr)
		require.Equal(message.Signature, sig)
	})
}

func TestDelegateManager_RequestSingleNodeInfoAsync(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	privKey, err := crypto.GenerateKey()
	require.NoError(err)

	t.Run("request_single", func(t *testing.T) {
		hMock := mock_nodeinfo.NewMockchain(ctrl)
		tMock := mock_nodeinfo.NewMocktransmitter(ctrl)
		dm := NewInfoManager(&DefaultConfig, tMock, hMock, getEmptyWhiteList, privKey)
		var paramPeer peer.AddrInfo
		var paramMsg *iotextypes.NodeInfoRequest
		peerID, err := peer.IDFromBytes([]byte("12D3KooWF2fns5ZWKbPfx2U1wQDdxoTK2D6HC3ortbSAQYR4BQp4"))
		require.NoError(err)
		targetPeer := peer.AddrInfo{ID: peerID}
		tMock.EXPECT().UnicastOutbound(gomock.Any(), gomock.Any(), gomock.Any()).Do(func(_ context.Context, p peer.AddrInfo, msg proto.Message) {
			paramPeer = p
			paramMsg = msg.(*iotextypes.NodeInfoRequest)
		}).Times(1)
		dm.RequestSingleNodeInfoAsync(context.Background(), targetPeer)
		require.Equal(targetPeer, paramPeer)
		request := iotextypes.NodeInfoRequest{}
		require.Equal(request.String(), paramMsg.String())
	})
}

func TestDelegateManager_GetNodeByAddr(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	hMock := mock_nodeinfo.NewMockchain(ctrl)
	tMock := mock_nodeinfo.NewMocktransmitter(ctrl)
	privKey, err := crypto.GenerateKey()
	require.NoError(err)

	dm := NewInfoManager(&DefaultConfig, tMock, hMock, getEmptyWhiteList, privKey)
	dm.updateNode(&Info{Address: "1"})
	dm.updateNode(&Info{Address: "2"})

	t.Run("exist", func(t *testing.T) {
		info, ok := dm.GetNodeInfo("1")
		require.True(ok)
		require.Equal(Info{Address: "1"}, info)
		info, ok = dm.GetNodeInfo("2")
		require.True(ok)
		require.Equal(Info{Address: "2"}, info)
	})
	t.Run("not_exist", func(t *testing.T) {
		_, ok := dm.GetNodeInfo("3")
		require.False(ok)
	})

}
