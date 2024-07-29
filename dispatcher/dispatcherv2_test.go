package dispatcher

import (
	"context"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-proto/golang/iotexrpc"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/iotexproject/iotex-core/testutil"
)

func TestDispatcherV2(t *testing.T) {
	r := require.New(t)
	t.Run("limitBlockSync", func(t *testing.T) {
		cfg := DefaultConfig
		cfg.ProcessSyncRequestInterval = time.Second
		dsp := NewDispatcherV2(cfg)
		r.NoError(dsp.Start(context.Background()))
		defer func() {
			r.NoError(dsp.Stop(context.Background()))
		}()
		peer1 := peer.AddrInfo{ID: "peer1"}
		peer2 := peer.AddrInfo{ID: "peer2"}
		count := atomic.NewInt32(0)
		ctrl := gomock.NewController(t)
		sub := NewMockSubscriber(ctrl)
		dsp.AddSubscriber(defaultChainID, sub)
		sub.EXPECT().ReportFullness(gomock.Any(), gomock.Any(), gomock.Any()).Return().AnyTimes()
		sub.EXPECT().HandleSyncRequest(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, peer peer.AddrInfo, msg *iotexrpc.BlockSync) error {
			count.Inc()
			return nil
		}).AnyTimes()
		dsp.HandleTell(context.Background(), defaultChainID, peer1, &iotexrpc.BlockSync{})
		dsp.HandleTell(context.Background(), defaultChainID, peer2, &iotexrpc.BlockSync{})
		dsp.HandleTell(context.Background(), defaultChainID, peer2, &iotexrpc.BlockSync{})
		r.NoError(testutil.WaitUntil(100*time.Millisecond, time.Second, func() (bool, error) {
			return dispatcherIsClean(dsp.(*dispatcherV2)), nil
		}))
		r.Equal(int32(2), count.Load())
	})
	t.Run("broadcast", func(t *testing.T) {
		dsp := NewDispatcherV2(DefaultConfig)
		r.NoError(dsp.Start(context.Background()))
		defer func() {
			r.NoError(dsp.Stop(context.Background()))
		}()
		// add subscriber
		ctrl := gomock.NewController(t)
		sub := NewMockSubscriber(ctrl)
		dsp.AddSubscriber(defaultChainID, sub)
		// Test handle broadcast
		cases := setTestCase()
		var (
			actionCount    atomic.Int32
			blockCount     atomic.Int32
			consensusCount atomic.Int32
			nodeInfoCount  atomic.Int32
		)
		sub.EXPECT().ReportFullness(gomock.Any(), gomock.Any(), gomock.Any()).Return().Times(len(cases))
		sub.EXPECT().HandleAction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, msg *iotextypes.Action) error {
			actionCount.Inc()
			return nil
		}).AnyTimes()
		sub.EXPECT().HandleBlock(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, hash string, msg *iotextypes.Block) error {
			blockCount.Inc()
			return nil
		}).AnyTimes()
		sub.EXPECT().HandleConsensusMsg(gomock.Any()).DoAndReturn(func(msg *iotextypes.ConsensusMessage) error {
			consensusCount.Inc()
			return nil
		}).AnyTimes()
		sub.EXPECT().HandleNodeInfo(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, hash string, msg *iotextypes.NodeInfo) error {
			nodeInfoCount.Inc()
			return nil
		})
		for _, msg := range cases {
			dsp.HandleBroadcast(context.Background(), defaultChainID, "peer1", msg)
		}
		r.NoError(testutil.WaitUntil(100*time.Millisecond, time.Second, func() (bool, error) {
			return dispatcherIsClean(dsp.(*dispatcherV2)), nil
		}))
		r.Equal(int32(2), actionCount.Load())
		r.Equal(int32(1), blockCount.Load())
		r.Equal(int32(1), consensusCount.Load())
		r.Equal(int32(1), nodeInfoCount.Load())
	})
	t.Run("unicast", func(t *testing.T) {
		dsp := NewDispatcherV2(DefaultConfig)
		r.NoError(dsp.Start(context.Background()))
		defer func() {
			r.NoError(dsp.Stop(context.Background()))
		}()
		// add subscriber
		ctrl := gomock.NewController(t)
		sub := NewMockSubscriber(ctrl)
		dsp.AddSubscriber(defaultChainID, sub)
		// Test handle broadcast
		cases := setTestCase()
		var (
			syncCount        atomic.Int32
			nodeInfoCount    atomic.Int32
			nodeInfoReqCount atomic.Int32
			blockCount       atomic.Int32
		)
		sub.EXPECT().ReportFullness(gomock.Any(), gomock.Any(), gomock.Any()).Return().Times(len(cases))
		sub.EXPECT().HandleSyncRequest(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, peer peer.AddrInfo, msg *iotexrpc.BlockSync) error {
			syncCount.Inc()
			return nil
		})
		sub.EXPECT().HandleNodeInfo(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, hash string, msg *iotextypes.NodeInfo) error {
			nodeInfoCount.Inc()
			return nil
		})
		sub.EXPECT().HandleNodeInfoRequest(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, peer peer.AddrInfo, msg *iotextypes.NodeInfoRequest) error {
			nodeInfoReqCount.Inc()
			return nil
		})
		sub.EXPECT().HandleBlock(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, hash string, msg *iotextypes.Block) error {
			blockCount.Inc()
			return nil
		}).AnyTimes()
		for _, msg := range cases {
			dsp.HandleTell(context.Background(), defaultChainID, peer.AddrInfo{}, msg)
		}
		r.NoError(testutil.WaitUntil(100*time.Millisecond, time.Second, func() (bool, error) {
			return dispatcherIsClean(dsp.(*dispatcherV2)), nil
		}))
		r.Equal(int32(1), syncCount.Load())
		r.Equal(int32(1), nodeInfoReqCount.Load())
		r.Equal(int32(1), nodeInfoCount.Load())
		r.Equal(int32(1), blockCount.Load())
	})
}

func dispatcherIsClean(dsp *dispatcherV2) bool {
	for k := range dsp.queueMgr.queues {
		if len(dsp.queueMgr.queues[k]) != 0 {
			return false
		}
	}
	return true
}
