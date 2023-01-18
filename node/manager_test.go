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
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	hMock := mock_node.NewMockheightable(ctrl)
	tMock := mock_node.NewMocktransmitter(ctrl)
	pMock := mock_node.NewMockprivateKey(ctrl)

	require := require.New(t)

	t.Run("disable_broadcast", func(t *testing.T) {
		cfg := Config{true, 100 * time.Millisecond}
		got := NewDelegateManager(&cfg, tMock, hMock, pMock)
		require.Equal(cfg, got.cfg)
		require.Equal(map[string]Info{}, got.nodeMap)
		require.Equal(tMock, got.transmitter)
		require.Equal(hMock, got.heightable)
		require.Equal(pMock, got.privKey)
		require.Equal(true, got.broadcaster != nil)
		tMock.EXPECT().BroadcastOutbound(gomock.Any(), gomock.Any()).Times(0)
		err := got.Start(context.Background())
		require.NoError(err)
		time.Sleep(time.Second)
	})

	t.Run("enable_broadcast", func(t *testing.T) {
		cfg := Config{false, 100 * time.Millisecond}
		got := NewDelegateManager(&cfg, tMock, hMock, pMock)
		require.Equal(cfg, got.cfg)
		require.Equal(map[string]Info{}, got.nodeMap)
		require.Equal(tMock, got.transmitter)
		require.Equal(hMock, got.heightable)
		require.Equal(pMock, got.privKey)
		require.Equal(true, got.broadcaster != nil)

		privK, _ := crypto.GenerateKey()
		hMock.EXPECT().TipHeight().Return(uint64(10)).AnyTimes()
		pMock.EXPECT().PublicKey().Return(privK.PublicKey()).AnyTimes()
		pMock.EXPECT().Sign(gomock.Any()).Return([]byte("xxxxxx"), nil).AnyTimes()
		tMock.EXPECT().BroadcastOutbound(gomock.Any(), gomock.Any()).Return(nil).MinTimes(1)
		err := got.Start(context.Background())
		require.NoError(err)
		time.Sleep(time.Second)
	})
}

func TestDelegateManager_HandleNodeInfo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	hMock := mock_node.NewMockheightable(ctrl)
	tMock := mock_node.NewMocktransmitter(ctrl)
	pMock := mock_node.NewMockprivateKey(ctrl)

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
		hash := hashNodeInfo(msg)
		msg.Signature, _ = privKey.Sign(hash[:])
		dm := NewDelegateManager(&DefaultConfig, tMock, hMock, pMock)
		dm.HandleNodeInfo(context.Background(), "abc", msg)
		addr := msg.Info.Address
		require.Equal(msg.Info.Height, dm.nodeMap[addr].Height)
		require.Equal(msg.Info.Version, dm.nodeMap[addr].Version)
		require.Equal(msg.Info.Timestamp.AsTime().String(), dm.nodeMap[addr].Timestamp.String())
		m := dto.Metric{}
		nodeDelegateHeightGauge.WithLabelValues(addr, msg.Info.Version).Write(&m)
		require.Equal(msg.Info.Height, uint64(m.Gauge.GetValue()))
	})

	t.Run("verify_unpass", func(t *testing.T) {
		privKey2, _ := crypto.GenerateKey()
		msg := &iotextypes.ResponseNodeInfoMessage{
			Info: &iotextypes.NodeInfo{
				Version:   "v1.8.0",
				Height:    200,
				Timestamp: timestamppb.Now(),
				Address:   privKey2.PublicKey().Address().String(),
			},
		}
		hash := hashNodeInfo(msg)
		msg.Signature, _ = privKey.Sign(hash[:])
		dm := NewDelegateManager(&DefaultConfig, tMock, hMock, pMock)
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
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	hMock := mock_node.NewMockheightable(ctrl)
	tMock := mock_node.NewMocktransmitter(ctrl)
	pMock := mock_node.NewMockprivateKey(ctrl)

	require := require.New(t)

	t.Run("update_self", func(t *testing.T) {
		dm := NewDelegateManager(&DefaultConfig, tMock, hMock, pMock)

		height := uint64(200)
		privK, _ := crypto.GenerateKey()
		tMock.EXPECT().BroadcastOutbound(gomock.Any(), gomock.Any()).Return(nil).Times(1)
		hMock.EXPECT().TipHeight().Return(height).Times(1)
		pMock.EXPECT().PublicKey().Return(privK.PublicKey()).Times(1)
		pMock.EXPECT().Sign(gomock.Any()).Return([]byte(""), nil).Times(1)
		err := dm.BroadcastNodeInfo(context.Background())

		require.NoError(err)
		require.Equal(height, dm.nodeMap[privK.PublicKey().Address().String()].Height)
	})
}

func TestDelegateManager_TellNodeInfo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	hMock := mock_node.NewMockheightable(ctrl)
	tMock := mock_node.NewMocktransmitter(ctrl)
	pMock := mock_node.NewMockprivateKey(ctrl)

	require := require.New(t)

	t.Run("tell", func(t *testing.T) {
		dm := &DelegateManager{
			cfg:         Config{},
			nodeMap:     map[string]Info{},
			broadcaster: nil,
			transmitter: tMock,
			heightable:  hMock,
			privKey:     pMock,
		}
		height := uint64(200)
		sign := []byte("xxxxxx")
		privK, _ := crypto.GenerateKey()
		message := &iotextypes.ResponseNodeInfoMessage{}

		hMock.EXPECT().TipHeight().Return(height).Times(1)
		tMock.EXPECT().UnicastOutbound(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, peerInfo peer.AddrInfo, msg proto.Message) error {
			*message = *msg.(*iotextypes.ResponseNodeInfoMessage)
			return nil
		}).Times(1)
		pMock.EXPECT().PublicKey().Return(privK.PublicKey()).Times(1)
		pMock.EXPECT().Sign(gomock.Any()).Return(sign, nil).Times(1)

		err := dm.TellNodeInfo(context.Background(), peer.AddrInfo{})

		require.NoError(err)
		require.Equal(message.Info.Height, height)
		require.Equal(message.Signature, sign)
	})
}

func TestDelegateManager_RequestSingleNodeInfoAsync(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	hMock := mock_node.NewMockheightable(ctrl)
	tMock := mock_node.NewMocktransmitter(ctrl)
	pMock := mock_node.NewMockprivateKey(ctrl)

	require := require.New(t)
	t.Run("request_single", func(t *testing.T) {
		dm := &DelegateManager{
			cfg:         Config{},
			nodeMap:     map[string]Info{},
			transmitter: tMock,
			heightable:  hMock,
			privKey:     pMock,
		}
		var paramPeer peer.AddrInfo
		var paramMsg iotextypes.RequestNodeInfoMessage
		targetPeer := peer.AddrInfo{ID: peer.ID("xxxx")}
		tMock.EXPECT().UnicastOutbound(gomock.Any(), gomock.Any(), gomock.Any()).Do(func(_ context.Context, p peer.AddrInfo, msg proto.Message) {
			paramPeer = p
			paramMsg = *msg.(*iotextypes.RequestNodeInfoMessage)
		}).Times(1)
		dm.RequestSingleNodeInfoAsync(context.Background(), targetPeer)

		require.Equal(targetPeer, paramPeer)
		require.Equal(iotextypes.RequestNodeInfoMessage{}, paramMsg)
	})

}
