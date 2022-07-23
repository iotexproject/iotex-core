package p2p

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iotexproject/go-p2p"
	"github.com/iotexproject/iotex-proto/golang/iotexrpc"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/testutil"
)

var _messages []*Message

func init() {
	node1, err := p2p.NewHost(context.Background(), p2p.DHTProtocolID(1), p2p.Port(testutil.RandomPort()), p2p.SecureIO(), p2p.MasterKey("bootnode"))
	if err != nil {
		panic(err)
	}
	defer node1.Close()
	target1 := node1.Info()

	node2, err := p2p.NewHost(context.Background(), p2p.DHTProtocolID(1), p2p.Port(testutil.RandomPort()), p2p.SecureIO(), p2p.MasterKey("bootnode"))
	if err != nil {
		panic(err)
	}
	defer node2.Close()
	target2 := node2.Info()

	_messages = []*Message{
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
			Target:  &target1,
			Data:    []byte{0, 1},
		},
		{
			MsgType: iotexrpc.MessageType_BLOCKS,
			ChainID: 1,
			Target:  &target2,
			Data:    []byte{0, 1},
		},
		{
			MsgType: iotexrpc.MessageType_ACTIONS,
			ChainID: 2,
			Target:  &target1,
			Data:    []byte{0, 1, 2},
		},
		{
			MsgType: iotexrpc.MessageType_BLOCKS,
			ChainID: 2,
			Target:  &target2,
			Data:    []byte{0, 1, 2, 3, 4, 5},
		},
	}
}

// TODO: add data validation

func TestBatchManager(t *testing.T) {
	require := require.New(t)

	manager := newBatchManager()

	var msgsSize int32 = 0
	var msgsCount int32 = 0
	go func() {
		for msg := range manager.OutputChannel() {
			atomic.AddInt32(&msgsSize, int32(msg.Size()))
			atomic.AddInt32(&msgsCount, 1)
		}
	}()

	t.Run("msgWithSizelimit", func(t *testing.T) {
		manager.Put(_messages[0], WithSizeLimit(3))
		manager.Put(_messages[0], WithSizeLimit(3))
		manager.Put(_messages[0], WithSizeLimit(3))
		err := testutil.WaitUntil(50*time.Millisecond, 3*time.Second, func() (bool, error) {
			return atomic.LoadInt32(&msgsCount) == 2 && atomic.LoadInt32(&msgsSize) == 6, nil
		})
		require.NoError(err)

		manager.Put(_messages[2], WithSizeLimit(4))
		manager.Put(_messages[1], WithSizeLimit(3))
		manager.Put(_messages[2], WithSizeLimit(4))
		manager.Put(_messages[1], WithSizeLimit(3))
		err = testutil.WaitUntil(50*time.Millisecond, 3*time.Second, func() (bool, error) {
			return atomic.LoadInt32(&msgsCount) == 4 && atomic.LoadInt32(&msgsSize) == 16, nil
		})
		require.NoError(err)

		manager.Put(_messages[7], WithSizeLimit(4))
		err = testutil.WaitUntil(50*time.Millisecond, 3*time.Second, func() (bool, error) {
			return atomic.LoadInt32(&msgsCount) == 5 && atomic.LoadInt32(&msgsSize) == 22, nil
		})
		require.NoError(err)
		require.Equal(4, len(manager.writerMap))

	})

	msgsSize = 0
	msgsCount = 0
	manager.writerMap = make(map[BatchID]*batchWriter)

	t.Run("msgWithInterval", func(t *testing.T) {
		manager.Put(_messages[0], WithInterval(50*time.Millisecond))
		time.Sleep(60 * time.Millisecond)
		err := testutil.WaitUntil(50*time.Millisecond, 1*time.Second, func() (bool, error) {
			return atomic.LoadInt32(&msgsCount) == 1 && atomic.LoadInt32(&msgsSize) == 2, nil
		})
		require.NoError(err)

		manager.Put(_messages[2], WithInterval(50*time.Millisecond))
		manager.Put(_messages[3], WithInterval(50*time.Millisecond))
		time.Sleep(60 * time.Millisecond)
		err = testutil.WaitUntil(50*time.Millisecond, 1*time.Second, func() (bool, error) {
			return atomic.LoadInt32(&msgsCount) == 3 && atomic.LoadInt32(&msgsSize) == 8, nil
		})
		require.NoError(err)
	})

	msgsSize = 0
	msgsCount = 0
	manager.writerMap = make(map[BatchID]*batchWriter)

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
			return atomic.LoadInt32(&msgsCount) == 4 && atomic.LoadInt32(&msgsSize) == 10, nil
		})
		require.NoError(err)
		require.Equal(0, len(manager.writerMap))
	})
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
		msgs:  make([]*Message, 0),
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
	msg := &Message{
		Data: []byte{1, 2, 3},
	}
	require.Equal(uint64(3), msg.Size())
}

func TestMessageBatchID(t *testing.T) {
	require := require.New(t)

	hashSet := make(map[BatchID]struct{})
	for _, msg := range _messages {
		id, err := msg.BatchID()
		require.NoError(err)
		hashSet[id] = struct{}{}
	}
	require.Less(0, len(_messages))
	require.Equal(len(_messages), len(hashSet))
}
