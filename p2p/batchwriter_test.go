package p2p

import (
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iotexproject/iotex-proto/golang/iotexrpc"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/testutil"
)

var (
	peerAddr1, _ = peer.AddrInfoFromP2pAddr(multiaddr.StringCast(`/ip4/127.0.0.1/tcp/31954/p2p/12D3KooWJwW7pUpTkxPTMv84RPLPMQVEAjZ6fvJuX4oZrvW5DAGQ`))
	peerAddr2, _ = peer.AddrInfoFromP2pAddr(multiaddr.StringCast(`/ip4/127.0.0.1/tcp/32954/p2p/12D3KooWJwW8pUpTkxPTMv84RPLPMQVEAjZ6fvJuX4oZrvW5DAGQ`))
	_messages    = []*batchMessage{
		// broadcast Messages
		{
			MsgType: iotexrpc.MessageType_ACTIONS,
			ChainID: 1,
			Data:    []byte{0, 1},
		},
		{
			MsgType: iotexrpc.MessageType_BLOCKS,
			ChainID: 1,
			Data:    []byte{0, 1},
		},
		{
			MsgType: iotexrpc.MessageType_ACTIONS,
			ChainID: 2,
			Data:    []byte{0, 1, 2},
		},
		{
			MsgType: iotexrpc.MessageType_BLOCKS,
			ChainID: 2,
			Data:    []byte{0, 1, 2},
		},
		// unicast Messages
		{
			MsgType: iotexrpc.MessageType_ACTIONS,
			ChainID: 1,
			Target:  peerAddr1,
			Data:    []byte{0, 1},
		},
		{
			MsgType: iotexrpc.MessageType_BLOCKS,
			ChainID: 1,
			Target:  peerAddr2,
			Data:    []byte{0, 1},
		},
		{
			MsgType: iotexrpc.MessageType_ACTIONS,
			ChainID: 2,
			Target:  peerAddr1,
			Data:    []byte{0, 1, 2},
		},
		{
			MsgType: iotexrpc.MessageType_BLOCKS,
			ChainID: 2,
			Target:  peerAddr2,
			Data:    []byte{0, 1, 2, 3, 4, 5},
		},
	}
)

func TestBatchManager(t *testing.T) {
	require := require.New(t)

	var msgsSize int32 = 0
	var msgsCount int32 = 0
	callback := func(msg *batchMessage) error {
		atomic.AddInt32(&msgsSize, int32(msg.Size()))
		atomic.AddInt32(&msgsCount, 1)
		return nil
	}

	manager := newBatchManager(callback)
	manager.Start()
	defer manager.Close()

	t.Run("msgWithSizelimit", func(t *testing.T) {
		manager.Put(_messages[0], withSizeLimit(3))
		manager.Put(_messages[0], withSizeLimit(3))
		manager.Put(_messages[0], withSizeLimit(3))
		err := testutil.WaitUntil(50*time.Millisecond, 3*time.Second, func() (bool, error) {
			return atomic.LoadInt32(&msgsCount) == 2 && atomic.LoadInt32(&msgsSize) == 6, nil
		})
		require.NoError(err)

		manager.Put(_messages[2], withSizeLimit(4))
		manager.Put(_messages[1], withSizeLimit(3))
		manager.Put(_messages[2], withSizeLimit(4))
		manager.Put(_messages[1], withSizeLimit(3))
		err = testutil.WaitUntil(50*time.Millisecond, 3*time.Second, func() (bool, error) {
			return atomic.LoadInt32(&msgsCount) == 4 && atomic.LoadInt32(&msgsSize) == 16, nil
		})
		require.NoError(err)

		manager.Put(_messages[7], withSizeLimit(4))
		err = testutil.WaitUntil(50*time.Millisecond, 3*time.Second, func() (bool, error) {
			return atomic.LoadInt32(&msgsCount) == 5 && atomic.LoadInt32(&msgsSize) == 22, nil
		})
		require.NoError(err)
		require.Equal(4, len(manager.writerMap))

	})

	msgsSize = 0
	msgsCount = 0
	manager.writerMap = make(map[batchID]*batchWriter)

	t.Run("msgwithInterval", func(t *testing.T) {
		manager.Put(_messages[0], withInterval(50*time.Millisecond))
		time.Sleep(60 * time.Millisecond)
		err := testutil.WaitUntil(50*time.Millisecond, 1*time.Second, func() (bool, error) {
			return atomic.LoadInt32(&msgsCount) == 1 && atomic.LoadInt32(&msgsSize) == 2, nil
		})
		require.NoError(err)

		manager.Put(_messages[2], withInterval(50*time.Millisecond))
		manager.Put(_messages[3], withInterval(50*time.Millisecond))
		time.Sleep(60 * time.Millisecond)
		err = testutil.WaitUntil(50*time.Millisecond, 1*time.Second, func() (bool, error) {
			return atomic.LoadInt32(&msgsCount) == 3 && atomic.LoadInt32(&msgsSize) == 8, nil
		})
		require.NoError(err)
	})

	msgsSize = 0
	msgsCount = 0
	manager.writerMap = make(map[batchID]*batchWriter)

	t.Run("writerTimeout", func(t *testing.T) {
		manager.Put(_messages[0], withInterval(1*time.Minute))
		manager.Put(_messages[1], withInterval(1*time.Minute))
		manager.Put(_messages[2], withInterval(1*time.Minute))
		manager.Put(_messages[3], withInterval(1*time.Minute))
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
			return atomic.LoadInt32(&msgsCount) == 4 && atomic.LoadInt32(&msgsSize) == 10, nil
		})
		require.NoError(err)
		require.Equal(0, len(manager.writerMap))
	})
}

func TestBatchDataCorrectness(t *testing.T) {
	require := require.New(t)

	var (
		msgsSize     int32 = 0
		msgsCount    int32 = 0
		expectedData       = append(_messages[0].Data, _messages[0].Data...)
	)
	callback := func(msg *batchMessage) error {
		atomic.AddInt32(&msgsSize, int32(msg.Size()))
		atomic.AddInt32(&msgsCount, 1)
		require.Equal(expectedData, msg.Data)
		return nil
	}

	manager := newBatchManager(callback)
	manager.Start()
	defer manager.Close()

	manager.Put(_messages[0], withSizeLimit(3))
	manager.Put(_messages[0], withSizeLimit(3))
	err := testutil.WaitUntil(50*time.Millisecond, 3*time.Second, func() (bool, error) {
		return atomic.LoadInt32(&msgsCount) == 1 && atomic.LoadInt32(&msgsSize) == 4, nil
	})
	require.NoError(err)
}

func TestBatchWriter(t *testing.T) {
	require := require.New(t)

	manager := &batchManager{
		assembleQueue: make(chan *batch, _bufferLength),
	}

	var batchesSize int32 = 0
	var batchesCount int32 = 0
	go func() {
		for batch := range manager.assembleQueue {
			atomic.AddInt32(&batchesSize, int32(batch.bytes))
			atomic.AddInt32(&batchesCount, 1)
		}
	}()

	t.Run("msgWithSizelimit", func(t *testing.T) {
		writer := newBatchWriter(
			&writerConfig{
				expiredThreshold: 10,
				sizeLimit:        5,
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
		writer.Put(_messages[2])
		err = testutil.WaitUntil(50*time.Millisecond, 1*time.Second, func() (bool, error) {
			return atomic.LoadInt32(&batchesCount) == 1 && atomic.LoadInt32(&batchesSize) == 7, nil
		})
		require.NoError(err)
		require.Nil(writer.curBatch)

		writer.Put(_messages[7])
		err = testutil.WaitUntil(50*time.Millisecond, 1*time.Second, func() (bool, error) {
			return atomic.LoadInt32(&batchesCount) == 2 && atomic.LoadInt32(&batchesSize) == 13, nil
		})
		require.NoError(err)
		require.Nil(writer.curBatch)

		writer.Put(_messages[3])
		err = testutil.WaitUntil(50*time.Millisecond, 1*time.Second, func() (bool, error) {
			return atomic.LoadInt32(&batchesCount) == 2 && atomic.LoadInt32(&batchesSize) == 13, nil
		})
		require.NoError(err)
		writer.Close()
		require.Error(writer.Put(_messages[3]))
		err = testutil.WaitUntil(50*time.Millisecond, 1*time.Second, func() (bool, error) {
			return atomic.LoadInt32(&batchesCount) == 3 && atomic.LoadInt32(&batchesSize) == 16, nil
		})
		require.NoError(err)
		require.Nil(writer.curBatch)
	})

	batchesSize = 0
	batchesCount = 0

	t.Run("msgwithInterval", func(t *testing.T) {
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
			return atomic.LoadInt32(&batchesCount) == 1 && atomic.LoadInt32(&batchesSize) == 2, nil
		})
		require.NoError(err)
		require.Nil(writer.curBatch)

		writer.Put(_messages[2])
		writer.Put(_messages[3])
		time.Sleep(60 * time.Millisecond)
		err = testutil.WaitUntil(50*time.Millisecond, 1*time.Second, func() (bool, error) {
			return atomic.LoadInt32(&batchesCount) == 2 && atomic.LoadInt32(&batchesSize) == 8, nil
		})
		require.NoError(err)
		require.Nil(writer.curBatch)

		writer.Put(_messages[4])
		writer.Close()
		require.Error(writer.Put(_messages[4]))
		time.Sleep(60 * time.Millisecond)
		require.Error(writer.Put(_messages[4]))
		err = testutil.WaitUntil(50*time.Millisecond, 1*time.Second, func() (bool, error) {
			return atomic.LoadInt32(&batchesCount) == 3 && atomic.LoadInt32(&batchesSize) == 10, nil
		})
		require.NoError(err)
		require.Nil(writer.curBatch)
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
		msgs:  make([]*batchMessage, 0),
		limit: 10,
		ready: readyChan,
	}

	batch.Add(_messages[0])
	batch.Add(_messages[1])
	require.Equal(2, batch.Count())
	require.Equal(uint64(4), batch.DataSize())
	require.False(batch.Full())

	batch.Add(_messages[2])
	batch.Add(_messages[3])
	require.Equal(4, batch.Count())
	require.Equal(uint64(10), batch.DataSize())
	require.True(batch.Full())

	batch.Flush()
	err := testutil.WaitUntil(50*time.Millisecond, 1*time.Second, func() (bool, error) {
		return atomic.LoadInt32(&isReady) == 1, nil
	})
	require.NoError(err)
}

func TestMessageSize(t *testing.T) {
	require := require.New(t)
	msg := &batchMessage{
		Data: []byte{1, 2, 3},
	}
	require.Equal(uint64(3), msg.Size())
}

func TestMessageBatchID(t *testing.T) {
	require := require.New(t)

	hashSet := make(map[batchID]struct{})
	for _, msg := range _messages {
		id, err := msg.BatchID()
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

	var msgsSize int32 = 0
	var msgsCount int32 = 0
	callback := func(msg *batchMessage) error {
		atomic.AddInt32(&msgsSize, int32(msg.Size()))
		atomic.AddInt32(&msgsCount, 1)
		return nil
	}

	manager := newBatchManager(callback)
	manager.Start()
	defer manager.Close()

	b.Run("packedMsgs", func(b *testing.B) {
		// generate Messages
		messages := make([]*batchMessage, 0, numMsgsForTest)
		chainIDRange := 2
		for i, chainID := 0, 0; i < numMsgsForTest; i++ {
			data := make([]byte, msgSize)
			messages = append(messages, &batchMessage{
				MsgType: iotexrpc.MessageType_ACTIONS,
				ChainID: uint32(chainID),
				Data:    data,
			})
			chainID = (chainID + 1) % chainIDRange
		}

		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			manager.writerMap = make(map[batchID]*batchWriter) // reset batch factory manually
			for i := range messages {
				manager.Put(messages[i], withSizeLimit(10000))
			}
		}
		b.StopTimer()
	})

	runtime.GC()

	b.Run("multipleBatchWriters", func(b *testing.B) {
		// generate Messages
		messages := make([]*batchMessage, 0, numMsgsForTest)
		for i := 0; i < numMsgsForTest; i++ {
			data := make([]byte, msgSize)
			messages = append(messages, &batchMessage{
				MsgType: iotexrpc.MessageType_ACTION,
				ChainID: uint32(i),
				Data:    data,
			})
		}

		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			manager.writerMap = make(map[batchID]*batchWriter) // reset batch factory manually
			for i := range messages {
				manager.Put(messages[i], withSizeLimit(uint64(msgSize)))
			}
		}
		b.StopTimer()
	})
}
