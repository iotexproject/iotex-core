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
	pMock := mock_node.NewMockprivateKey(ctrl)

	type args struct {
		cfg *Config
		t   transmitter
		h   heightable
		p   privateKey
	}
	tests := []struct {
		name              string
		args              args
		want              *DelegateManager
		broadcasterIsNull bool
	}{
		{
			"disable_broadcast",
			args{&Config{}, tMock, hMock, pMock},
			&DelegateManager{
				cfg:         Config{},
				nodeMap:     map[string]iotextypes.NodeInfo{},
				transmitter: tMock,
				heightable:  hMock,
				privKey:     pMock,
			},
			true,
		},
		{
			"enable_broadcast",
			args{&Config{time.Second * 3}, tMock, hMock, pMock},
			&DelegateManager{
				cfg:         Config{time.Second * 3},
				nodeMap:     map[string]iotextypes.NodeInfo{},
				transmitter: tMock,
				heightable:  hMock,
				privKey:     pMock,
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewDelegateManager(tt.args.cfg, tt.args.t, tt.args.h, tt.args.p)
			require.Equal(t, tt.want.cfg, got.cfg)
			require.Equal(t, tt.want.nodeMap, got.nodeMap)
			require.Equal(t, tt.want.transmitter, got.transmitter)
			require.Equal(t, tt.want.heightable, got.heightable)
			require.Equal(t, tt.want.privKey, got.privKey)
			require.Equal(t, tt.broadcasterIsNull, got.broadcaster == nil)
		})
	}
}

func TestDelegateManager_HandleNodeInfo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	hMock := mock_node.NewMockheightable(ctrl)
	tMock := mock_node.NewMocktransmitter(ctrl)
	pMock := mock_node.NewMockprivateKey(ctrl)

	privKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	type fields struct {
		cfg         Config
		nodeMap     map[string]iotextypes.NodeInfo
		broadcaster *routine.RecurringTask
		transmitter transmitter
		heightable  heightable
		privKey     privateKey
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
		valid  bool
	}{
		{
			"valid",
			fields{
				cfg:         Config{},
				nodeMap:     map[string]iotextypes.NodeInfo{},
				transmitter: tMock,
				heightable:  hMock,
				privKey:     pMock,
			},
			args{
				context.Background(),
				"abc",
				func() *iotextypes.ResponseNodeInfoMessage {
					msg := &iotextypes.ResponseNodeInfoMessage{
						Info: &iotextypes.NodeInfo{
							Version:   "v1.8.0",
							Height:    200,
							Timestamp: timestamppb.Now(),
						},
					}
					hash := hashNodeInfo(msg)
					msg.Signature, _ = privKey.Sign(hash[:])
					msg.Pubkey = privKey.PublicKey().Bytes()
					return msg
				}(),
			},
			true,
		},
		{
			"invalid",
			fields{
				cfg:         Config{},
				nodeMap:     map[string]iotextypes.NodeInfo{},
				transmitter: tMock,
				heightable:  hMock,
				privKey:     pMock,
			},
			args{
				context.Background(),
				"abc",
				func() *iotextypes.ResponseNodeInfoMessage {
					msg := &iotextypes.ResponseNodeInfoMessage{
						Info: &iotextypes.NodeInfo{
							Version:   "v1.8.0",
							Height:    200,
							Timestamp: timestamppb.Now(),
						},
					}
					hash := hashNodeInfo(msg)
					msg.Signature, _ = privKey.Sign(hash[:])
					privKey2, _ := crypto.GenerateKey()
					msg.Pubkey = privKey2.PublicKey().Bytes()
					return msg
				}(),
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
				privKey:     tt.fields.privKey,
			}
			dm.HandleNodeInfo(tt.args.ctx, tt.args.addr, tt.args.node)
			pubKey, err := crypto.BytesToPublicKey(tt.args.node.Pubkey)
			if err != nil {
				t.Fatal(err)
			}
			addr := pubKey.Address().String()
			if tt.valid {
				require.Equal(t, tt.args.node.Info.Height, dm.nodeMap[addr].Height)
				require.Equal(t, tt.args.node.Info.Version, dm.nodeMap[addr].Version)
				require.Equal(t, tt.args.node.Info.Timestamp.String(), dm.nodeMap[addr].Timestamp.String())

				m := dto.Metric{}
				nodeDelegateHeightGauge.WithLabelValues(addr, tt.args.node.Info.Version).Write(&m)
				require.Equal(t, tt.args.node.Info.Height, uint64(m.Gauge.GetValue()))
			} else {
				_, ok := dm.nodeMap[addr]
				require.False(t, ok)
				m := dto.Metric{}
				nodeDelegateHeightGauge.WithLabelValues(addr, tt.args.node.Info.Version).Write(&m)
				require.Equal(t, uint64(0), uint64(m.Gauge.GetValue()))
			}
		})
	}
}

func TestDelegateManager_RequestNodeInfo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	hMock := mock_node.NewMockheightable(ctrl)
	tMock := mock_node.NewMocktransmitter(ctrl)
	pMock := mock_node.NewMockprivateKey(ctrl)

	type fields struct {
		cfg         Config
		nodeMap     map[string]iotextypes.NodeInfo
		broadcaster *routine.RecurringTask
		transmitter transmitter
		heightable  heightable
		privateKey  privateKey
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
				pMock,
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
				privKey:     tt.fields.privateKey,
			}

			height := uint64(200)
			privK, _ := crypto.GenerateKey()
			tMock.EXPECT().BroadcastOutbound(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			hMock.EXPECT().TipHeight().Return(height).Times(1)
			pMock.EXPECT().PublicKey().Return(privK.PublicKey()).Times(1)
			dm.RequestNodeInfo()

			require.Equal(t, height, dm.nodeMap[privK.PublicKey().Address().String()].Height)
		})
	}
}

func TestDelegateManager_TellNodeInfo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	hMock := mock_node.NewMockheightable(ctrl)
	tMock := mock_node.NewMocktransmitter(ctrl)
	pMock := mock_node.NewMockprivateKey(ctrl)

	type fields struct {
		cfg         Config
		nodeMap     map[string]iotextypes.NodeInfo
		broadcaster *routine.RecurringTask
		transmitter transmitter
		heightable  heightable
		privateKey  privateKey
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
				pMock,
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
				privKey:     tt.fields.privateKey,
			}
			privK, _ := crypto.GenerateKey()
			message := &iotextypes.ResponseNodeInfoMessage{}
			hMock.EXPECT().TipHeight().Return(tt.args.height).Times(1)
			tMock.EXPECT().UnicastOutbound(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, peerInfo peer.AddrInfo, msg proto.Message) error {
				*message = *msg.(*iotextypes.ResponseNodeInfoMessage)
				return nil
			}).Times(1)
			pMock.EXPECT().PublicKey().Return(privK.PublicKey()).Times(1)
			pMock.EXPECT().Sign(gomock.Any()).Return(tt.args.sign, nil).Times(1)
			if err := dm.TellNodeInfo(tt.args.ctx, tt.args.peer); (err != nil) != tt.wantErr {
				t.Errorf("DelegateManager.TellNodeInfo() error = %v, wantErr %v", err, tt.wantErr)
			}

			require.Equal(t, message.Info.Height, tt.args.height)
			require.Equal(t, message.Signature, tt.args.sign)
		})
	}
}
