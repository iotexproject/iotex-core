// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package nodeinfo

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
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
		cfg := Config{true, 100 * time.Millisecond, 1000}
		dm := NewInfoManager(&cfg, tMock, hMock, 2500*time.Millisecond, privK)
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
		cfg := Config{false, 100 * time.Millisecond, 1000}
		dm := NewInfoManager(&cfg, tMock, hMock, 2500*time.Millisecond, privK)
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
		cfg := Config{false, 100 * time.Millisecond, 1000}
		dm := NewInfoManager(&cfg, tMock, hMock, 2500*time.Millisecond, privK)
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
		dm := NewInfoManager(&DefaultConfig, tMock, hMock, 2500*time.Millisecond, privKey)
		dm.HandleNodeInfo(context.Background(), "abc", msg)
		addr := msg.Info.Address
		nodeGot, ok := dm.nodeMap.Get("abc")
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

	t.Run("nil_msg_and_nil_info", func(t *testing.T) {
		hMock := mock_nodeinfo.NewMockchain(ctrl)
		tMock := mock_nodeinfo.NewMocktransmitter(ctrl)
		dm := NewInfoManager(&DefaultConfig, tMock, hMock, 2500*time.Millisecond, privKey)
		// Both nil msg and msg with nil Info must not panic on field deref / hash.
		require.NotPanics(func() { dm.HandleNodeInfo(context.Background(), "abc", nil) })
		require.NotPanics(func() {
			dm.HandleNodeInfo(context.Background(), "abc", &iotextypes.NodeInfo{Signature: []byte("sig")})
		})
		_, ok := dm.nodeMap.Get("abc")
		require.False(ok)
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
		dm := NewInfoManager(&DefaultConfig, tMock, hMock, 2500*time.Millisecond, privKey)
		dm.HandleNodeInfo(context.Background(), "abc", msg)
		addr := msg.Info.Address
		_, ok := dm.nodeMap.Get(addr)
		require.False(ok)
		m := dto.Metric{}
		_nodeInfoHeightGauge.WithLabelValues(addr, msg.Info.Version).Write(&m)
		require.Equal(uint64(0), uint64(m.Gauge.GetValue()))
	})
}

func signedNodeInfo(t *testing.T, privKey crypto.PrivateKey, ver string, height uint64) *iotextypes.NodeInfo {
	msg := &iotextypes.NodeInfo{
		Info: &iotextypes.NodeInfoCore{
			Version:   ver,
			Height:    height,
			Timestamp: timestamppb.Now(),
			Address:   privKey.PublicKey().Address().String(),
		},
	}
	h := hashNodeInfo(msg.Info)
	sig, err := privKey.Sign(h[:])
	require.NoError(t, err)
	msg.Signature = sig
	return msg
}

// heightSeries returns the version label values exported for each address
func heightSeries(t *testing.T) map[string][]string {
	ch := make(chan prometheus.Metric, 1024)
	go func() {
		_nodeInfoHeightGauge.Collect(ch)
		close(ch)
	}()
	series := make(map[string][]string)
	for m := range ch {
		pb := dto.Metric{}
		require.NoError(t, m.Write(&pb))
		var addr, ver string
		for _, l := range pb.GetLabel() {
			switch l.GetName() {
			case "address":
				addr = l.GetValue()
			case "version":
				ver = l.GetValue()
			}
		}
		series[addr] = append(series[addr], ver)
	}
	return series
}

func gaugeValue(t *testing.T, addr, ver string) float64 {
	m := dto.Metric{}
	require.NoError(t, _nodeInfoHeightGauge.WithLabelValues(addr, ver).Write(&m))
	return m.GetGauge().GetValue()
}

func TestDelegateManager_HeightSeriesBound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	hMock := mock_nodeinfo.NewMockchain(ctrl)
	tMock := mock_nodeinfo.NewMocktransmitter(ctrl)

	t.Run("series_count_capped_and_version_sanitized", func(t *testing.T) {
		require := require.New(t)
		_nodeInfoHeightGauge.Reset()
		defer _nodeInfoHeightGauge.Reset()
		const limit = 8
		cfg := Config{true, time.Minute, limit}
		dm := NewInfoManager(&cfg, tMock, hMock, 2500*time.Millisecond)
		longVersion := strings.Repeat("v1.0.0\n{}\"é", 400)
		for i := 0; i < 10*limit; i++ {
			privKey, err := crypto.GenerateKey()
			require.NoError(err)
			ver := fmt.Sprintf("%d-%s", i, longVersion)
			dm.HandleNodeInfo(context.Background(), "peer", signedNodeInfo(t, privKey, ver, uint64(i+1)))
		}
		series := heightSeries(t)
		require.Len(series, limit)
		for _, vers := range series {
			require.Len(vers, 1)
			require.LessOrEqual(len(vers[0]), _maxVersionLabelLen)
			require.Regexp(`^[0-9A-Za-z._+-]+$`, vers[0])
		}
		info, ok := dm.GetNodeInfo("peer")
		require.True(ok)
		require.LessOrEqual(len(info.Version), _maxVersionLabelLen)
	})

	t.Run("one_series_per_address_with_latest_version", func(t *testing.T) {
		require := require.New(t)
		_nodeInfoHeightGauge.Reset()
		defer _nodeInfoHeightGauge.Reset()
		dm := NewInfoManager(&DefaultConfig, tMock, hMock, 2500*time.Millisecond)
		privKey, err := crypto.GenerateKey()
		require.NoError(err)
		addr := privKey.PublicKey().Address().String()

		dm.HandleNodeInfo(context.Background(), "p1", signedNodeInfo(t, privKey, "v2.5.1", 100))
		dm.HandleNodeInfo(context.Background(), "p1", signedNodeInfo(t, privKey, "v2.5.2", 110))
		require.Equal(map[string][]string{addr: {"v2.5.2"}}, heightSeries(t))
		require.Equal(float64(110), gaugeValue(t, addr, "v2.5.2"))

		// a message reporting a lower height does not replace the exported version
		dm.HandleNodeInfo(context.Background(), "p2", signedNodeInfo(t, privKey, "v2.5.1", 105))
		require.Equal(map[string][]string{addr: {"v2.5.2"}}, heightSeries(t))
		require.Equal(float64(110), gaugeValue(t, addr, "v2.5.2"))

		// the same address keeps a single series as it reports from several peers
		dm.HandleNodeInfo(context.Background(), "p2", signedNodeInfo(t, privKey, "v2.5.2", 120))
		require.Equal(map[string][]string{addr: {"v2.5.2"}}, heightSeries(t))
		require.Equal(float64(120), gaugeValue(t, addr, "v2.5.2"))
	})

	t.Run("least_recently_updated_address_removed", func(t *testing.T) {
		require := require.New(t)
		_nodeInfoHeightGauge.Reset()
		defer _nodeInfoHeightGauge.Reset()
		cfg := Config{true, time.Minute, 2}
		dm := NewInfoManager(&cfg, tMock, hMock, 2500*time.Millisecond)
		keys := make([]crypto.PrivateKey, 3)
		for i := range keys {
			var err error
			keys[i], err = crypto.GenerateKey()
			require.NoError(err)
		}
		dm.HandleNodeInfo(context.Background(), "p0", signedNodeInfo(t, keys[0], "v1", 1))
		dm.HandleNodeInfo(context.Background(), "p1", signedNodeInfo(t, keys[1], "v1", 1))
		// refresh keys[0] so keys[1] is the least recently updated
		dm.HandleNodeInfo(context.Background(), "p0", signedNodeInfo(t, keys[0], "v1", 2))
		dm.HandleNodeInfo(context.Background(), "p2", signedNodeInfo(t, keys[2], "v1", 1))
		series := heightSeries(t)
		require.Len(series, 2)
		require.Contains(series, keys[0].PublicKey().Address().String())
		require.Contains(series, keys[2].PublicKey().Address().String())
		require.NotContains(series, keys[1].PublicKey().Address().String())
	})
}

func TestSanitizeVersion(t *testing.T) {
	require := require.New(t)
	require.Equal("v2.5.1.2-3-gabc1234", sanitizeVersion("v2.5.1.2-3-gabc1234"))
	require.Equal("NoBuildInfo", sanitizeVersion("NoBuildInfo"))
	require.Equal("v1_0__", sanitizeVersion("v1 0\n\""))
	require.Equal("v__", sanitizeVersion("vé"))
	require.Equal(strings.Repeat("a", _maxVersionLabelLen), sanitizeVersion(strings.Repeat("a", 2*_maxVersionLabelLen)))
}

func TestDelegateManager_BroadcastNodeInfo(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	privKey, err := crypto.GenerateKey()
	require.NoError(err)

	t.Run("update_self", func(t *testing.T) {
		hMock := mock_nodeinfo.NewMockchain(ctrl)
		tMock := mock_nodeinfo.NewMocktransmitter(ctrl)
		dm := NewInfoManager(&DefaultConfig, tMock, hMock, 2500*time.Millisecond, privKey)
		height := uint64(200)
		peerID, err := peer.IDFromBytes([]byte("12D3KooWF2fns5ZWKbPfx2U1wQDdxoTK2D6HC3ortbSAQYR4BQp4"))
		require.NoError(err)
		hMock.EXPECT().TipHeight().Return(height).Times(1)
		tMock.EXPECT().Info().Return(peer.AddrInfo{ID: peerID}, nil).Times(1)
		tMock.EXPECT().BroadcastOutbound(gomock.Any(), gomock.Any()).Return(nil).Times(1)
		err = dm.BroadcastNodeInfo(context.Background(), []crypto.PrivateKey{privKey})
		require.NoError(err)
		addr := privKey.PublicKey().Address().String()
		nodeGot, ok := dm.nodeMap.Get(peerID.String())
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
		dm := NewInfoManager(&DefaultConfig, tMock, hMock, 2500*time.Millisecond, privKey)
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
		dm := NewInfoManager(&DefaultConfig, tMock, hMock, 2500*time.Millisecond, privKey)
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

	dm := NewInfoManager(&DefaultConfig, tMock, hMock, 2500*time.Millisecond, privKey)
	dm.updateNode(&Info{PeerID: "1"})
	dm.updateNode(&Info{PeerID: "2"})

	t.Run("exist", func(t *testing.T) {
		info, ok := dm.GetNodeInfo("1")
		require.True(ok)
		require.Equal(Info{PeerID: "1"}, info)
		info, ok = dm.GetNodeInfo("2")
		require.True(ok)
		require.Equal(Info{PeerID: "2"}, info)
	})
	t.Run("not_exist", func(t *testing.T) {
		_, ok := dm.GetNodeInfo("3")
		require.False(ok)
	})

}
