package p2p

import (
	"runtime"
	"sync/atomic"
	"testing"

	"github.com/iotexproject/iotex-proto/golang/iotexrpc"
)

func BenchmarkBatchManager(b *testing.B) {
	var (
		numMsgsForTest = 1000
		msgSize        = 500
	)

	manager := newBatchManager()

	var msgsSize int32 = 0
	var msgsCount int32 = 0
	go func() {
		for msg := range manager.OutputChannel() {
			atomic.AddInt32(&msgsSize, int32(msg.Size()))
			atomic.AddInt32(&msgsCount, 1)
			// log.L().Info("packed msg received", zap.Uint64("data size", msg.Size()), zap.Int32("total msgs", atomic.LoadInt32(&msgsCount)))
		}
	}()

	b.Run("packedMsgs", func(b *testing.B) {
		// generate Messages
		messages := make([]*Message, 0, numMsgsForTest)
		chainIDRange := 2
		for i, chainID := 0, 0; i < numMsgsForTest; i++ {
			data := make([]byte, msgSize)
			messages = append(messages, &Message{
				MsgType: iotexrpc.MessageType_ACTIONS,
				ChainID: uint32(chainID),
				Data:    data,
			})
			chainID = (chainID + 1) % chainIDRange
		}

		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			manager.writerMap = make(map[BatchID]*batchWriter)
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
			data := make([]byte, msgSize)
			messages = append(messages, &Message{
				MsgType: iotexrpc.MessageType_ACTION,
				ChainID: uint32(i),
				Data:    data,
			})
		}

		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			manager.writerMap = make(map[BatchID]*batchWriter)
			for i := range messages {
				manager.Put(messages[i], WithSizeLimit(uint64(msgSize)))
			}
		}
		b.StopTimer()
	})

}
