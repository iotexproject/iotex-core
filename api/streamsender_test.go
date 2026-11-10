package api

import (
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	apitypes "github.com/iotexproject/iotex-core/v2/api/types"
	"github.com/iotexproject/iotex-core/v2/blockchain"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

func streamTestBlock(t *testing.T) *block.Block {
	blk, err := block.NewTestingBuilder().
		SetHeight(1).SetVersion(1).SetTimeStamp(time.Now()).
		SignAndBuild(identityset.PrivateKey(0))
	require.NoError(t, err)
	return &blk
}

// A stream consumer that stops reading must not apply backpressure to the block
// fan-out: Respond has to stay non-blocking and the subscription is dropped
// instead. Regression guard -- a gRPC stream write can block while the peer is
// not reading, and Respond runs on the goroutine that blockchain.commitBlock
// waits on while holding the chain lock.
func TestStreamSender_SlowConsumerIsDroppedNotTolerated(t *testing.T) {
	r := require.New(t)

	blk := streamTestBlock(t)
	ps := blockchain.NewPubSub(2)
	cl := NewChainListener(10)
	r.NoError(ps.AddBlockListener(cl))

	release := make(chan struct{})
	defer close(release)
	errChan := make(chan error, 1)
	_, err := cl.AddResponder(NewGRPCBlockListener(
		func(interface{}) (int, error) { <-release; return 0, nil }, errChan))
	r.NoError(err)

	var delivered int64
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < streamSendQueueSize*4; i++ {
			ps.SendBlockToSubscribers(blk)
			atomic.AddInt64(&delivered, 1)
		}
	}()

	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatalf("block fan-out stalled behind a stuck stream consumer (delivered=%d)",
			atomic.LoadInt64(&delivered))
	}
	r.EqualValues(streamSendQueueSize*4, atomic.LoadInt64(&delivered))

	// the stuck subscription is reported and evicted
	select {
	case err := <-errChan:
		r.ErrorIs(err, errSlowStreamConsumer)
	case <-time.After(5 * time.Second):
		t.Fatal("stuck subscription was never reported to the RPC handler")
	}
	r.Eventually(func() bool { return cl.(*chainListener).streamMap.Count() == 0 },
		5*time.Second, 20*time.Millisecond, "stuck subscription was never evicted")
}

// A stuck consumer must not starve subscribe/unsubscribe for everyone else:
// ttl.Cache.Range holds the map's write lock for the whole iteration, so
// responders have to be invoked outside it.
func TestChainListener_StuckResponderDoesNotStarveSubscribe(t *testing.T) {
	r := require.New(t)

	blk := streamTestBlock(t)
	cl := NewChainListener(10)
	release := make(chan struct{})
	defer close(release)

	blocking := make(chan struct{})
	var entered int32
	_, err := cl.AddResponder(newFuncResponder(func() {
		if atomic.AddInt32(&entered, 1) == 1 {
			close(blocking)
		}
		<-release
	}))
	r.NoError(err)

	go cl.ReceiveBlock(blk)
	<-blocking // a responder is now inside Respond

	addDone := make(chan struct{})
	go func() {
		defer close(addDone)
		cl.AddResponder(NewGRPCBlockListener(
			func(interface{}) (int, error) { return 0, nil }, make(chan error, 1)))
	}()
	select {
	case <-addDone:
	case <-time.After(5 * time.Second):
		t.Fatal("AddResponder blocked while another responder was mid-Respond")
	}
}

// Exit() and a failing write race on the same subscription must not deadlock and
// must report exactly one outcome (fail is once-guarded; errChan is never
// closed). Looped so a rare double-send would show up.
func TestStreamSender_ExitDoesNotDeadlock(t *testing.T) {
	r := require.New(t)
	for i := 0; i < 100; i++ {
		errChan := make(chan error, 1)
		bl := NewGRPCBlockListener(func(interface{}) (int, error) { return 0, errorSend }, errChan)
		blk := streamTestBlock(t)
		r.NoError(bl.Respond("", blk)) // queued; the write fails on the sender goroutine

		done := make(chan struct{})
		go func() { defer close(done); bl.Exit(); bl.Exit() }()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("Exit() deadlocked")
		}
		select {
		case <-errChan:
		case <-time.After(5 * time.Second):
			t.Fatal("subscription outcome was never reported")
		}
		// the subscription has already ended, so no second outcome may appear
		select {
		case err := <-errChan:
			r.Failf("unexpected second outcome", "iter %d: %v", i, err)
		case <-time.After(5 * time.Millisecond):
		}
	}
}

// Regression for the capacity-rejection leak: NewGRPCBlockListener starts its
// sender goroutine at construction, so a responder that is never registered
// (e.g. AddResponder hit the listener cap) must be reclaimed by Exit. Without
// the handler's Exit-on-failure, every rejected stream would leak a goroutine.
func TestStreamSender_ExitReclaimsGoroutine(t *testing.T) {
	r := require.New(t)
	before := runtime.NumGoroutine()
	for i := 0; i < 200; i++ {
		errChan := make(chan error, 1)
		resp := NewGRPCBlockListener(func(interface{}) (int, error) { return 0, nil }, errChan)
		resp.Exit() // the path a handler takes when AddResponder is rejected
	}
	r.Eventually(func() bool { return runtime.NumGoroutine() <= before+10 },
		5*time.Second, 20*time.Millisecond, "Exit did not reclaim sender goroutines")
}

// Regression for the per-block burst false-positive: one block's messages are
// enqueued as a single group occupying one queue slot, so a block with far more
// than streamSendQueueSize items (e.g. >64 matching logs) does not overflow the
// queue and drop a healthy consumer. Only a consumer that falls behind by
// streamSendQueueSize whole blocks is dropped.
func TestStreamSender_LargePerBlockBurstIsOneSlot(t *testing.T) {
	r := require.New(t)
	release := make(chan struct{})
	defer close(release)
	errChan := make(chan error, 1)
	// the handler blocks on the first message so nothing drains the queue
	s := newStreamSender("logs", func(interface{}) (int, error) {
		<-release
		return 0, nil
	}, errChan)

	// one block carrying far more than streamSendQueueSize messages: one slot
	big := make([]interface{}, streamSendQueueSize*10)
	for i := range big {
		big[i] = "msg"
	}
	r.NoError(s.enqueue(big...))

	// the queue still has room for many more per-block groups; a giant single
	// block did not consume the whole queue
	for i := 0; i < streamSendQueueSize-2; i++ {
		r.NoError(s.enqueue("x"))
	}
	select {
	case err := <-errChan:
		r.Failf("healthy consumer dropped", "only a large single-block burst was sent: %v", err)
	default:
	}
}

func newFuncResponder(fn func()) apitypes.Responder { return &funcResponder{fn: fn} }

type funcResponder struct{ fn func() }

func (f *funcResponder) Respond(_ string, _ *block.Block) error { f.fn(); return nil }
func (f *funcResponder) Exit()                                  {}
