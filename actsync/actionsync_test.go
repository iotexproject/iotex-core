package actsync

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/testutil"
)

func TestActionSyncStartStop(t *testing.T) {
	r := require.New(t)
	as := NewActionSync(DefaultConfig, &Helper{
		P2PNeighbor: func() ([]peer.AddrInfo, error) {
			return nil, nil
		},
		UnicastOutbound: func(context.Context, peer.AddrInfo, proto.Message) error {
			return nil
		},
	})
	// start
	r.False(as.IsReady(), "not be ready before started")
	r.NoError(as.Start(context.Background()))
	r.True(as.IsReady(), "be ready after started")
	// stop
	r.NoError(as.Stop(context.Background()))
	r.False(as.IsReady(), "not be ready after stopped")
}

func TestActionSync(t *testing.T) {
	r := require.New(t)
	neighbors := []peer.AddrInfo{
		{ID: peer.ID("peer1")},
		{ID: peer.ID("peer2")},
		{ID: peer.ID("peer3")},
	}
	count := atomic.Int32{}
	as := NewActionSync(Config{
		Size:     1000,
		Interval: 10 * time.Millisecond,
	}, &Helper{
		P2PNeighbor: func() ([]peer.AddrInfo, error) {
			return neighbors, nil
		},
		UnicastOutbound: func(_ context.Context, p peer.AddrInfo, msg proto.Message) error {
			count.Add(1)
			return nil
		},
	})
	r.NoError(as.Start(context.Background()))
	defer func() {
		r.NoError(as.Stop(context.Background()))
	}()

	t.Run("requestingAction", func(t *testing.T) {
		actHash := hash.BytesToHash256([]byte("test"))
		as.RequestAction(context.Background(), actHash)
		// keep requesting actions until received the action
		testutil.WaitUntil(100*time.Millisecond, time.Second, func() (bool, error) {
			return count.Load() > 1000, nil
		})
		as.ReceiveAction(context.Background(), actHash)
		_, ok := as.actions.Load(actHash)
		r.False(ok, "action should be removed after received")
		// request same action again
		as.RequestAction(context.Background(), actHash)
		as.RequestAction(context.Background(), actHash)
		as.RequestAction(context.Background(), actHash)
		// receive same action again
		as.ReceiveAction(context.Background(), actHash)
		as.ReceiveAction(context.Background(), actHash)
		as.ReceiveAction(context.Background(), actHash)
	})
	t.Run("concurrency", func(t *testing.T) {
		acts := []hash.Hash256{}
		for i := 0; i < 100; i++ {
			acts = append(acts, hash.Hash256b([]byte{byte(i)}))
		}
		// request actions concurrently
		wg := sync.WaitGroup{}
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				for k := 0; k <= 10; k++ {
					idx := i*10 + k
					if idx >= len(acts) {
						break
					}
					as.RequestAction(context.Background(), acts[idx])
				}
			}(i)
		}
		wg.Wait()
		// check actions
		for _, act := range acts {
			_, ok := as.actions.Load(act)
			r.True(ok, "action should be requested")
		}
		// receive actions concurrently
		wg = sync.WaitGroup{}
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				for k := 0; k <= 10; k++ {
					idx := i*10 + k
					if idx >= len(acts) {
						break
					}
					as.ReceiveAction(context.Background(), acts[idx])
				}
			}(i)
		}
		wg.Wait()
		// check actions
		for _, act := range acts {
			_, ok := as.actions.Load(act)
			r.False(ok, "action should be removed after received")
		}
	})
	t.Run("requestWhenStopping", func(t *testing.T) {
		count := atomic.Int32{}
		as := NewActionSync(Config{
			Size:     1000,
			Interval: 10 * time.Millisecond,
		}, &Helper{
			P2PNeighbor: func() ([]peer.AddrInfo, error) {
				return neighbors, nil
			},
			UnicastOutbound: func(_ context.Context, p peer.AddrInfo, msg proto.Message) error {
				count.Add(1)
				return nil
			},
		})
		r.NoError(as.Start(context.Background()))
		acts := []hash.Hash256{}
		for i := 0; i < 100; i++ {
			acts = append(acts, hash.Hash256b([]byte{byte(i)}))
		}
		wg := sync.WaitGroup{}
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				for k := 0; k <= 10; k++ {
					idx := i*10 + k
					if idx >= len(acts) {
						break
					}
					as.RequestAction(context.Background(), acts[idx])
				}
			}(i)
		}
		r.NoError(as.Stop(context.Background()))
		wg.Wait()
	})
}
