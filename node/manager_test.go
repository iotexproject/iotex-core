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

	require := require.New(t)

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
				nodeMap:     map[string]Info{},
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
				nodeMap:     map[string]Info{},
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
			require.Equal(tt.want.cfg, got.cfg)
			require.Equal(tt.want.nodeMap, got.nodeMap)
			require.Equal(tt.want.transmitter, got.transmitter)
			require.Equal(tt.want.heightable, got.heightable)
			require.Equal(tt.want.privKey, got.privKey)
			require.Equal(tt.broadcasterIsNull, got.broadcaster == nil)
		})
	}
}

func TestDelegateManager_HandleNodeInfo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	hMock := mock_node.NewMockheightable(ctrl)
	tMock := mock_node.NewMocktransmitter(ctrl)
	pMock := mock_node.NewMockprivateKey(ctrl)

	require := require.New(t)
	privKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	type fields struct {
		cfg         Config
		nodeMap     map[string]Info
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
				nodeMap:     map[string]Info{},
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
							Address:   privKey.PublicKey().Address().String(),
						},
					}
					hash := hashNodeInfo(msg)
					msg.Signature, _ = privKey.Sign(hash[:])
					return msg
				}(),
			},
			true,
		},
		{
			"invalid",
			fields{
				cfg:         Config{},
				nodeMap:     map[string]Info{},
				transmitter: tMock,
				heightable:  hMock,
				privKey:     pMock,
			},
			args{
				context.Background(),
				"abc",
				func() *iotextypes.ResponseNodeInfoMessage {
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
			addr := tt.args.node.Info.Address
			if tt.valid {
				require.Equal(tt.args.node.Info.Height, dm.nodeMap[addr].Height)
				require.Equal(tt.args.node.Info.Version, dm.nodeMap[addr].Version)
				require.Equal(tt.args.node.Info.Timestamp.AsTime().String(), dm.nodeMap[addr].Timestamp.String())

				m := dto.Metric{}
				nodeDelegateHeightGauge.WithLabelValues(addr, tt.args.node.Info.Version).Write(&m)
				require.Equal(tt.args.node.Info.Height, uint64(m.Gauge.GetValue()))
			} else {
				_, ok := dm.nodeMap[addr]
				require.False(ok)
				m := dto.Metric{}
				nodeDelegateHeightGauge.WithLabelValues(addr, tt.args.node.Info.Version).Write(&m)
				require.Equal(uint64(0), uint64(m.Gauge.GetValue()))
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

	require := require.New(t)
	type fields struct {
		cfg         Config
		nodeMap     map[string]Info
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
				map[string]Info{},
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

			require.Equal(height, dm.nodeMap[privK.PublicKey().Address().String()].Height)
		})
	}
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

func TestPubkey(t *testing.T) {
	masterKey := "96f0aa5e8523d6a28dc35c927274be4e931e74eaa720b418735debfcbfe712b8"
	sk, err := crypto.HexStringToPrivateKey(masterKey)
	if err != nil {
		t.Fatal(err)
	}
	ioPubKey := sk.PublicKey()
	ioAddr := ioPubKey.Address().String()

	hash := []byte("testtesttesttesttesttesttesttest")
	sign, err := sk.Sign(hash)
	if err != nil {
		t.Fatal(err)
	}

	recovPubKey, err := crypto.RecoverPubkey(hash, sign)
	if err != nil {
		t.Fatal(err)
	}
	recovAddr := recovPubKey.Address().String()

	t.Log("ioAddr=", ioAddr, "recovAddr", recovAddr)
}
