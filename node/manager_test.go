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
	"github.com/iotexproject/iotex-core/pkg/routine"
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
	sMock := mock_node.NewMocksigner(ctrl)

	type args struct {
		cfg *Config
		t   transmitter
		h   heightable
		s   signer
	}
	tests := []struct {
		name              string
		args              args
		want              *DelegateManager
		broadcasterIsNull bool
	}{
		{
			"disable_broadcast",
			args{&Config{}, tMock, hMock, sMock},
			&DelegateManager{
				cfg:         Config{},
				nodeMap:     map[string]iotextypes.NodeInfo{},
				transmitter: tMock,
				heightable:  hMock,
				signer:      sMock,
			},
			true,
		},
		{
			"enable_broadcast",
			args{&Config{time.Second * 3}, tMock, hMock, sMock},
			&DelegateManager{
				cfg:         Config{time.Second * 3},
				nodeMap:     map[string]iotextypes.NodeInfo{},
				transmitter: tMock,
				heightable:  hMock,
				signer:      sMock,
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewDelegateManager(tt.args.cfg, tt.args.t, tt.args.h, tt.args.s)
			require.Equal(t, tt.want.cfg, got.cfg)
			require.Equal(t, tt.want.nodeMap, got.nodeMap)
			require.Equal(t, tt.want.transmitter, got.transmitter)
			require.Equal(t, tt.want.heightable, got.heightable)
			require.Equal(t, tt.want.signer, got.signer)
			require.Equal(t, tt.broadcasterIsNull, got.broadcaster == nil)
		})
	}
}

func TestDelegateManager_HandleNodeInfo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	hMock := mock_node.NewMockheightable(ctrl)
	tMock := mock_node.NewMocktransmitter(ctrl)
	sMock := mock_node.NewMocksigner(ctrl)

	type fields struct {
		cfg         Config
		nodeMap     map[string]iotextypes.NodeInfo
		broadcaster *routine.RecurringTask
		transmitter transmitter
		heightable  heightable
		signer      signer
	}
	type args struct {
		ctx  context.Context
		addr string
		node *iotextypes.ResponseNodeInfoMessage
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			"",
			fields{
				cfg:         Config{},
				nodeMap:     map[string]iotextypes.NodeInfo{},
				transmitter: tMock,
				heightable:  hMock,
				signer:      sMock,
			},
			args{
				context.Background(),
				"abc",
				&iotextypes.ResponseNodeInfoMessage{
					Info: &iotextypes.NodeInfo{
						Version:   "v1.8.0",
						Height:    200,
						Timestamp: timestamppb.Now(),
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dm := &DelegateManager{
				cfg:         tt.fields.cfg,
				nodeMap:     tt.fields.nodeMap,
				broadcaster: tt.fields.broadcaster,
				transmitter: tt.fields.transmitter,
				heightable:  tt.fields.heightable,
				signer:      tt.fields.signer,
			}
			dm.HandleNodeInfo(tt.args.ctx, tt.args.addr, tt.args.node)

			require.Equal(t, tt.args.node.Info.Height, dm.nodeMap[tt.args.addr].Height)
			require.Equal(t, tt.args.node.Info.Version, dm.nodeMap[tt.args.addr].Version)
			require.Equal(t, tt.args.node.Info.Timestamp.String(), dm.nodeMap[tt.args.addr].Timestamp.String())

			m := dto.Metric{}
			nodeDelegateHeightGauge.WithLabelValues(tt.args.addr, tt.args.node.Info.Version).Write(&m)
			require.Equal(t, tt.args.node.Info.Height, uint64(m.Gauge.GetValue()))
		})
	}
}

func TestDelegateManager_RequestNodeInfo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	hMock := mock_node.NewMockheightable(ctrl)
	tMock := mock_node.NewMocktransmitter(ctrl)
	sMock := mock_node.NewMocksigner(ctrl)

	type fields struct {
		cfg         Config
		nodeMap     map[string]iotextypes.NodeInfo
		broadcaster *routine.RecurringTask
		transmitter transmitter
		heightable  heightable
		signer      signer
	}
	tests := []struct {
		name   string
		fields fields
	}{
		{
			"update_self",
			fields{
				Config{},
				map[string]iotextypes.NodeInfo{},
				nil,
				tMock,
				hMock,
				sMock,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dm := &DelegateManager{
				cfg:         tt.fields.cfg,
				nodeMap:     tt.fields.nodeMap,
				broadcaster: tt.fields.broadcaster,
				transmitter: tt.fields.transmitter,
				heightable:  tt.fields.heightable,
				signer:      tt.fields.signer,
			}
			peerID := peer.AddrInfo{ID: "abcd"}
			height := uint64(200)
			tMock.EXPECT().BroadcastOutbound(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			tMock.EXPECT().Info().Return(peerID, nil).Times(1)
			hMock.EXPECT().TipHeight().Return(height).Times(1)
			dm.RequestNodeInfo()

			require.Equal(t, height, dm.nodeMap[peerID.ID.Pretty()].Height)
		})
	}
}

func TestDelegateManager_TellNodeInfo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	hMock := mock_node.NewMockheightable(ctrl)
	tMock := mock_node.NewMocktransmitter(ctrl)
	sMock := mock_node.NewMocksigner(ctrl)

	type fields struct {
		cfg         Config
		nodeMap     map[string]iotextypes.NodeInfo
		broadcaster *routine.RecurringTask
		transmitter transmitter
		heightable  heightable
		signer      signer
	}
	type args struct {
		ctx    context.Context
		peer   peer.AddrInfo
		height uint64
		sign   []byte
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			"",
			fields{
				Config{},
				map[string]iotextypes.NodeInfo{},
				nil,
				tMock,
				hMock,
				sMock,
			},
			args{
				context.Background(),
				peer.AddrInfo{ID: "abcd"},
				200,
				[]byte("xxxxxx"),
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dm := &DelegateManager{
				cfg:         tt.fields.cfg,
				nodeMap:     tt.fields.nodeMap,
				broadcaster: tt.fields.broadcaster,
				transmitter: tt.fields.transmitter,
				heightable:  tt.fields.heightable,
				signer:      tt.fields.signer,
			}
			message := &iotextypes.ResponseNodeInfoMessage{}
			hMock.EXPECT().TipHeight().Return(tt.args.height).Times(1)
			tMock.EXPECT().UnicastOutbound(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, peerInfo peer.AddrInfo, msg proto.Message) error {
				*message = *msg.(*iotextypes.ResponseNodeInfoMessage)
				return nil
			}).Times(1)
			sMock.EXPECT().Sign(gomock.Any()).Return(tt.args.sign, nil).Times(1)
			if err := dm.TellNodeInfo(tt.args.ctx, tt.args.peer); (err != nil) != tt.wantErr {
				t.Errorf("DelegateManager.TellNodeInfo() error = %v, wantErr %v", err, tt.wantErr)
			}

			require.Equal(t, message.Info.Height, tt.args.height)
			require.Equal(t, message.Signature, tt.args.sign)
		})
	}
}
