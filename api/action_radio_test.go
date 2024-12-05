package api

import (
	"context"
	"math/big"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

func TestActionRadio(t *testing.T) {
	r := require.New(t)
	broadcastCount := uint64(0)
	radio := NewActionRadio(
		func(_ context.Context, _ uint32, _ proto.Message) error {
			atomic.AddUint64(&broadcastCount, 1)
			return nil
		},
		0)
	r.NoError(radio.Start())
	defer func() {
		r.NoError(radio.Stop())
	}()

	gas := uint64(100000)
	gasPrice := big.NewInt(10)
	selp, err := action.SignedTransfer(identityset.Address(1).String(), identityset.PrivateKey(1), 1, big.NewInt(1), nil, gas, gasPrice)
	r.NoError(err)

	radio.OnAdded(selp)
	r.Equal(uint64(1), atomic.LoadUint64(&broadcastCount))
}

func TestActionRadioRetry(t *testing.T) {
	r := require.New(t)
	broadcastCnt := uint64(0)
	pendings := make([]*action.SealedEnvelope, 0)
	mutex := sync.Mutex{}
	ar := NewActionRadio(func(ctx context.Context, chainID uint32, msg proto.Message) error {
		atomic.AddUint64(&broadcastCnt, 1)
		return nil
	}, 0, WithRetry(func() chan *action.SealedEnvelope {
		ch := make(chan *action.SealedEnvelope, 1)
		go func() {
			mutex.Lock()
			for _, p := range pendings {
				ch <- p
			}
			mutex.Unlock()
			close(ch)
		}()
		return ch
	}, 3, 20*time.Millisecond))
	ar.tickInterval = 10 * time.Millisecond

	r.NoError(ar.Start())
	defer ar.Stop()

	setPending := func(acts ...*action.SealedEnvelope) {
		mutex.Lock()
		pendings = acts
		mutex.Unlock()
	}

	tsf1, err := action.SignedTransfer(identityset.Address(1).String(), identityset.PrivateKey(1), 1, big.NewInt(1), nil, 10000, big.NewInt(1))
	r.NoError(err)
	tsf2, err := action.SignedTransfer(identityset.Address(1).String(), identityset.PrivateKey(1), 2, big.NewInt(1), nil, 10000, big.NewInt(1))
	r.NoError(err)
	tsf3, err := action.SignedTransfer(identityset.Address(1).String(), identityset.PrivateKey(1), 3, big.NewInt(1), nil, 10000, big.NewInt(1))
	r.NoError(err)

	// -- case 1: retry pending actions at most 3 times
	r.Equal(uint64(0), atomic.LoadUint64(&broadcastCnt))
	// add first action
	ar.OnAdded(tsf1)
	r.Equal(uint64(1), atomic.LoadUint64(&broadcastCnt))
	// add second action
	ar.OnAdded(tsf2)
	r.Equal(uint64(2), atomic.LoadUint64(&broadcastCnt))
	// set tsf1 as pending
	time.Sleep(ar.retryInterval)
	setPending(tsf1)
	// first retry after interval
	time.Sleep(ar.retryInterval)
	r.Equal(uint64(3), atomic.LoadUint64(&broadcastCnt))
	// retry 3 at most
	time.Sleep(ar.retryInterval * 10)
	r.Equal(uint64(5), atomic.LoadUint64(&broadcastCnt))
	// tsf1 confirmed
	setPending()
	ar.OnRemoved(tsf1)

	// -- case 2: retry + 1 if receive again
	setPending(tsf2)
	// first retry after interval
	time.Sleep(ar.retryInterval)
	r.Equal(uint64(6), atomic.LoadUint64(&broadcastCnt))
	// receive tsf2 again and retry+1
	ar.OnRejected(context.Background(), tsf2, action.ErrExistedInPool)
	time.Sleep(ar.retryInterval * 10)
	r.Equal(uint64(7), atomic.LoadUint64(&broadcastCnt))

	// -- case 3: ignore if receive again from API
	ar.OnAdded(tsf3)
	r.Equal(uint64(8), atomic.LoadUint64(&broadcastCnt))
	time.Sleep(ar.retryInterval)
	setPending(tsf3)
	// first retry after interval
	time.Sleep(ar.retryInterval)
	r.Equal(uint64(9), atomic.LoadUint64(&broadcastCnt))
	// receive tsf3 again from API
	ar.OnRejected(WithAPIContext(context.Background()), tsf3, action.ErrExistedInPool)
	time.Sleep(ar.retryInterval * 10)
	r.Equal(uint64(11), atomic.LoadUint64(&broadcastCnt))
}
