package batch

import (
	"math/big"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

var (
	peerAddr1, _ = peer.AddrInfoFromP2pAddr(multiaddr.StringCast(`/ip4/127.0.0.1/tcp/31954/p2p/12D3KooWJwW7pUpTkxPTMv84RPLPMQVEAjZ6fvJuX4oZrvW5DAGQ`))
	peerAddr2, _ = peer.AddrInfoFromP2pAddr(multiaddr.StringCast(`/ip4/127.0.0.1/tcp/32954/p2p/12D3KooWJwW8pUpTkxPTMv84RPLPMQVEAjZ6fvJuX4oZrvW5DAGQ`))

	tx1, _ = action.SignedTransfer(identityset.Address(28).String(),
		identityset.PrivateKey(28), 3, big.NewInt(10), []byte{}, testutil.TestGasLimit,
		big.NewInt(testutil.TestGasPriceInt64))
	txProto1 = tx1.Proto()
	tx2, _   = action.SignedExecution(identityset.Address(29).String(),
		identityset.PrivateKey(29), 1, big.NewInt(0), testutil.TestGasLimit,
		big.NewInt(testutil.TestGasPriceInt64), []byte{})
	txProto2  = tx2.Proto()
	_messages = []*Message{
		// broadcast Messages
		{
			// MsgType: iotexrpc.MessageType_ACTIONS,
			ChainID: 1,
			Data:    txProto1,
			Target:  nil,
		},
		{
			ChainID: 3,
			Target:  nil,
			Data:    &iotextypes.Action{},
		},
		{
			// MsgType: iotexrpc.MessageType_ACTIONS,
			ChainID: 2,
			Data:    txProto2,
			Target:  nil,
		},
		{
			ChainID: 4,
			Target:  nil,
			Data:    &iotextypes.Action{},
		},
		// unicast Messages
		{
			// MsgType: iotexrpc.MessageType_ACTIONS,
			ChainID: 1,
			Data:    txProto1,
			Target:  peerAddr1,
		},
		{
			// MsgType: iotexrpc.MessageType_ACTIONS,
			ChainID: 2,
			Data:    txProto2,
			Target:  peerAddr1,
		},
	}
)

func TestBatchManager(t *testing.T) {
	require := require.New(t)

	var msgsCount int32 = 0
	callback := func(msg *Message) error {
		atomic.AddInt32(&msgsCount, 1)
		return nil
	}

	manager := NewManager(callback)
	manager.Start()
	defer manager.Stop()

	t.Run("msgWithSizelimit", func(t *testing.T) {
		manager.Put(_messages[0], WithSizeLimit(2))
		manager.Put(_messages[0], WithSizeLimit(2))
		manager.Put(_messages[0], WithSizeLimit(2))
		err := testutil.WaitUntil(50*time.Millisecond, 3*time.Second, func() (bool, error) {
			return atomic.LoadInt32(&msgsCount) == 2, nil
		})
		require.NoError(err)

		manager.Put(_messages[2], WithSizeLimit(2))
		manager.Put(_messages[1], WithSizeLimit(2))
		manager.Put(_messages[2], WithSizeLimit(2))
		manager.Put(_messages[1], WithSizeLimit(2))
		err = testutil.WaitUntil(50*time.Millisecond, 3*time.Second, func() (bool, error) {
			return atomic.LoadInt32(&msgsCount) == 4, nil
		})
		require.NoError(err)

		manager.Put(_messages[5], WithSizeLimit(1))
		err = testutil.WaitUntil(50*time.Millisecond, 3*time.Second, func() (bool, error) {
			return atomic.LoadInt32(&msgsCount) == 5, nil
		})
		require.NoError(err)
		require.Equal(4, len(manager.writerMap))
	})

	msgsCount = 0
	manager.writerMap = make(map[batchID]*batchWriter)

	t.Run("msgWithInterval", func(t *testing.T) {
		manager.Put(_messages[0], WithInterval(50*time.Millisecond))
		time.Sleep(60 * time.Millisecond)
		err := testutil.WaitUntil(50*time.Millisecond, 1*time.Second, func() (bool, error) {
			return atomic.LoadInt32(&msgsCount) == 1, nil
		})
		require.NoError(err)

		manager.Put(_messages[2], WithInterval(50*time.Millisecond))
		manager.Put(_messages[3], WithInterval(50*time.Millisecond))
		time.Sleep(60 * time.Millisecond)
		err = testutil.WaitUntil(50*time.Millisecond, 1*time.Second, func() (bool, error) {
			return atomic.LoadInt32(&msgsCount) == 3, nil
		})
		require.NoError(err)
	})

	msgsCount = 0
	manager.writerMap = make(map[batchID]*batchWriter)

	t.Run("writerTimeout", func(t *testing.T) {
		manager.Put(_messages[0], WithInterval(1*time.Minute))
		manager.Put(_messages[1], WithInterval(1*time.Minute))
		manager.Put(_messages[2], WithInterval(1*time.Minute))
		manager.Put(_messages[3], WithInterval(1*time.Minute))
		require.Equal(4, len(manager.writerMap))
		err := testutil.WaitUntil(50*time.Millisecond, 1*time.Second, func() (bool, error) {
			queuedItems := 0
			for _, writer := range manager.writerMap {
				queuedItems += len(writer.msgBuffer)
			}

			return queuedItems == 0, nil
		})
		require.NoError(err)
		manager.cleanupLoop()
		manager.cleanupLoop()
		err = testutil.WaitUntil(50*time.Millisecond, 1*time.Second, func() (bool, error) {
			return atomic.LoadInt32(&msgsCount) == 4, nil
		})
		require.NoError(err)
		require.Equal(0, len(manager.writerMap))
	})
}

func TestBatchDataCorrectness(t *testing.T) {
	require := require.New(t)

	var (
		msgsCount    int32 = 0
		expectedData       = iotextypes.Actions{Actions: []*iotextypes.Action{txProto1, txProto1}}
	)
	callback := func(msg *Message) error {
		atomic.AddInt32(&msgsCount, 1)
		binary1, _ := proto.Marshal(&expectedData)
		binary2, _ := proto.Marshal(msg.Data)
		require.Equal(binary1, binary2)
		return nil
	}

	manager := NewManager(callback)
	manager.Start()
	defer manager.Stop()

	manager.Put(_messages[0], WithSizeLimit(2))
	manager.Put(_messages[0], WithSizeLimit(2))
	err := testutil.WaitUntil(50*time.Millisecond, 3*time.Second, func() (bool, error) {
		return atomic.LoadInt32(&msgsCount) == 1, nil
	})
	require.NoError(err)
}

func TestBatchWriter(t *testing.T) {
	require := require.New(t)

	manager := &Manager{
		assembleQueue: make(chan *batch, _bufferLength),
	}

	var batchesCount int32 = 0
	go func() {
		for range manager.assembleQueue {
			atomic.AddInt32(&batchesCount, 1)
		}
	}()

	isBatchNil := func(writer *batchWriter) bool {
		writer.mu.Lock()
		ret := writer.curBatch == nil
		writer.mu.Unlock()
		return ret
	}

	t.Run("msgWithSizelimit", func(t *testing.T) {
		writer := newBatchWriter(
			&writerConfig{
				expiredThreshold: 10,
				sizeLimit:        2,
				msgInterval:      10 * time.Minute,
			},
			manager,
		)

		// wait until writer is ready
		err := testutil.WaitUntil(10*time.Millisecond, 10*time.Second, func() (bool, error) {
			return writer.IsReady(), nil
		})
		require.NoError(err)

		writer.Put(_messages[0])
		writer.Put(_messages[1])
		err = testutil.WaitUntil(50*time.Millisecond, 1*time.Second, func() (bool, error) {
			return atomic.LoadInt32(&batchesCount) == 1, nil
		})
		require.NoError(err)
		require.True(isBatchNil(writer))

		writer.Put(_messages[5])
		err = testutil.WaitUntil(50*time.Millisecond, 1*time.Second, func() (bool, error) {
			return atomic.LoadInt32(&batchesCount) == 1, nil
		})
		require.NoError(err)
		require.False(isBatchNil(writer))

		writer.Put(_messages[3])
		err = testutil.WaitUntil(50*time.Millisecond, 1*time.Second, func() (bool, error) {
			return atomic.LoadInt32(&batchesCount) == 2, nil
		})
		require.NoError(err)
		require.True(isBatchNil(writer))

		writer.Put(_messages[3])
		err = testutil.WaitUntil(50*time.Millisecond, 1*time.Second, func() (bool, error) {
			return atomic.LoadInt32(&batchesCount) == 2, nil
		})
		require.NoError(err)
		require.False(isBatchNil(writer))
		writer.Close()
		require.Error(writer.Put(_messages[3]))
		err = testutil.WaitUntil(50*time.Millisecond, 1*time.Second, func() (bool, error) {
			return atomic.LoadInt32(&batchesCount) == 3, nil
		})
		require.NoError(err)
		require.Nil(writer.curBatch)
	})

	batchesCount = 0

	t.Run("msgWithInterval", func(t *testing.T) {
		writer := newBatchWriter(
			&writerConfig{
				expiredThreshold: 10,
				sizeLimit:        100,
				msgInterval:      50 * time.Millisecond,
			},
			manager,
		)

		// wait until writer is ready
		err := testutil.WaitUntil(10*time.Millisecond, 10*time.Second, func() (bool, error) {
			return writer.IsReady(), nil
		})
		require.NoError(err)

		writer.Put(_messages[0])
		time.Sleep(60 * time.Millisecond)
		err = testutil.WaitUntil(50*time.Millisecond, 1*time.Second, func() (bool, error) {
			return atomic.LoadInt32(&batchesCount) == 1, nil
		})
		require.NoError(err)
		require.True(isBatchNil(writer))

		writer.Put(_messages[2])
		writer.Put(_messages[3])
		time.Sleep(60 * time.Millisecond)
		err = testutil.WaitUntil(50*time.Millisecond, 1*time.Second, func() (bool, error) {
			return atomic.LoadInt32(&batchesCount) == 2, nil
		})
		require.NoError(err)
		require.True(isBatchNil(writer))

		writer.Put(_messages[4])
		writer.Close()
		require.Error(writer.Put(_messages[4]))
		time.Sleep(60 * time.Millisecond)
		require.Error(writer.Put(_messages[4]))
		err = testutil.WaitUntil(50*time.Millisecond, 1*time.Second, func() (bool, error) {
			return atomic.LoadInt32(&batchesCount) == 3, nil
		})
		require.NoError(err)
		require.True(isBatchNil(writer))
	})
}

func TestBatch(t *testing.T) {
	require := require.New(t)

	readyChan := make(chan struct{})
	var isReady int32 = 0
	go func() {
		for range readyChan {
			atomic.AddInt32(&isReady, 1)
			return
		}
	}()
	batch := &batch{
		msgs:      make([]*Message, 0),
		sizeLimit: 4,
		ready:     readyChan,
	}

	batch.Add(_messages[0])
	batch.Add(_messages[1])
	require.Equal(2, batch.Size())
	require.False(batch.Full())

	batch.Add(_messages[2])
	batch.Add(_messages[3])
	require.Equal(4, batch.Size())
	require.True(batch.Full())

	batch.Flush()
	err := testutil.WaitUntil(50*time.Millisecond, 1*time.Second, func() (bool, error) {
		return atomic.LoadInt32(&isReady) == 1, nil
	})
	require.NoError(err)
}

func TestMessageBatchID(t *testing.T) {
	require := require.New(t)

	hashSet := make(map[batchID]struct{})
	for _, msg := range _messages {
		id, err := msg.batchID()
		require.NoError(err)
		hashSet[id] = struct{}{}
	}
	require.Less(0, len(_messages))
	require.Equal(len(_messages), len(hashSet))
}

func BenchmarkBatchManager(b *testing.B) {
	var (
		numMsgsForTest = 1000
		msgSize        = 500
	)

	var msgsCount int32 = 0
	callback := func(msg *Message) error {
		atomic.AddInt32(&msgsCount, 1)
		return nil
	}

	manager := NewManager(callback)
	manager.Start()
	defer manager.Stop()

	b.Run("packedMsgs", func(b *testing.B) {
		// generate Messages
		messages := make([]*Message, 0, numMsgsForTest)
		chainIDRange := 2
		for i, chainID := 0, 0; i < numMsgsForTest; i++ {
			messages = append(messages, &Message{
				ChainID: uint32(chainID),
				Data:    txProto1,
			})
			chainID = (chainID + 1) % chainIDRange
		}

		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			manager.writerMap = make(map[batchID]*batchWriter) // reset batch factory manually
			for i := range messages {
				manager.Put(messages[i], WithSizeLimit(10000))
			}
		}
		b.StopTimer()
	})

	runtime.GC()

	b.Run("multipleBatchWriters", func(b *testing.B) {
		// generate Messages
		messages := make([]*Message, 0, numMsgsForTest)
		for i := 0; i < numMsgsForTest; i++ {
			messages = append(messages, &Message{
				ChainID: uint32(i),
				Data:    txProto1,
			})
		}

		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			manager.writerMap = make(map[batchID]*batchWriter) // reset batch factory manually
			for i := range messages {
				manager.Put(messages[i], WithSizeLimit(uint64(msgSize)))
			}
		}
		b.StopTimer()
	})
}
