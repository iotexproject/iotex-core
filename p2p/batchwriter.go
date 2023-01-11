package p2p

import (
	"bytes"
	"context"
	"encoding/binary"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/iotexproject/iotex-proto/golang/iotexrpc"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/pkg/log"
)

const (
	_bufferLength    = 500
	_maxWriters      = 5000
	_cleanupInterval = 15 * time.Minute
)

type batchOption func(cfg *writerConfig)

func withInterval(t time.Duration) batchOption {
	return func(cfg *writerConfig) {
		cfg.msgInterval = t
	}
}

func withSizeLimit(limit uint64) batchOption {
	return func(cfg *writerConfig) {
		cfg.sizeLimit = limit
	}
}

type (
	batchManager struct {
		mu                  sync.RWMutex
		writerMap           map[batchID]*batchWriter
		outputQueue         chan *batchMessage // assembled message queue for external reader
		assembleQueue       chan *batch        // batch queue which collects batches sent from writers
		batchMessageHandler messageOutbound
		cancelHanlder       context.CancelFunc
	}

	batchID uint64

	messageOutbound func(*batchMessage) error
)

// TODO: move BatchManager outside of p2p after serialized messages
// could be passed directly to p2p module
func newBatchManager(handler messageOutbound) *batchManager {
	return &batchManager{
		writerMap:           make(map[batchID]*batchWriter),
		outputQueue:         make(chan *batchMessage, _bufferLength),
		assembleQueue:       make(chan *batch, _bufferLength),
		batchMessageHandler: handler,
	}
}

func (bm *batchManager) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	go bm.assemble(ctx)
	go bm.cleanup(ctx, _cleanupInterval)
	go bm.callback(ctx)
	bm.cancelHanlder = cancel
}

func (bm *batchManager) Close() {
	bm.cancelHanlder()
}

func (bm *batchManager) Put(msg *batchMessage, opts ...batchOption) error {
	if !bm.supported(msg.MsgType) {
		return errors.New("message is unsupported for batching")
	}
	id, err := msg.BatchID()
	if err != nil {
		return err
	}
	bm.mu.RLock()
	writer, exist := bm.writerMap[id]
	bm.mu.RUnlock()
	if !exist {
		cfg := _defaultWriterConfig
		for _, opt := range opts {
			opt(cfg)
		}
		bm.mu.Lock()
		if len(bm.writerMap) > _maxWriters {
			bm.mu.Unlock()
			return errors.New("the batch is full")
		}
		writer = newBatchWriter(cfg, bm)
		bm.writerMap[id] = writer
		bm.mu.Unlock()
	}
	return writer.Put(msg)
}

func (bm *batchManager) supported(msgType iotexrpc.MessageType) bool {
	return msgType == iotexrpc.MessageType_ACTIONS || msgType == iotexrpc.MessageType_BLOCKS
}

func (bm *batchManager) assemble(ctx context.Context) {
	for {
		select {
		case batch := <-bm.assembleQueue:
			if batch.Count() == 0 {
				continue
			}
			var (
				msg0 = batch.msgs[0]
				data = make([]byte, 0, batch.DataSize())
			)
			for i := range batch.msgs {
				data = append(data, batch.msgs[i].Data...)
			}
			bm.outputQueue <- &batchMessage{
				MsgType: msg0.MsgType,
				ChainID: msg0.ChainID,
				Target:  msg0.Target,
				Data:    data,
			}
		case <-ctx.Done():
			return
		}
	}
}

func (bm *batchManager) cleanup(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	for {
		select {
		case <-ticker.C:
			bm.cleanupLoop()
		case <-ctx.Done():
			ticker.Stop()
			return
		}
	}
}

func (bm *batchManager) callback(ctx context.Context) {
	for {
		select {
		case msg := <-bm.outputQueue:
			err := bm.batchMessageHandler(msg)
			if err != nil {
				log.L().Error("fail to handle a batch message when calling back", zap.Error(err))
			}
		case <-ctx.Done():
			return
		}
	}
}

func (bm *batchManager) cleanupLoop() {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	for k, v := range bm.writerMap {
		v.AddTimeoutTimes()
		if v.Expired() {
			v.Close()
			delete(bm.writerMap, k)
		}
	}
}

type writerConfig struct {
	expiredThreshold uint32
	sizeLimit        uint64
	msgInterval      time.Duration
}

var _defaultWriterConfig = &writerConfig{
	expiredThreshold: 2,
	msgInterval:      100 * time.Millisecond,
	sizeLimit:        1e6,
}

type batchWriter struct {
	lifecycle.Readiness
	mu sync.RWMutex

	manager *batchManager
	cfg     writerConfig

	msgBuffer chan *batchMessage
	curBatch  *batch

	timeoutTimes uint32
}

func newBatchWriter(cfg *writerConfig, manager *batchManager) *batchWriter {
	bw := &batchWriter{
		manager:   manager,
		cfg:       *cfg,
		msgBuffer: make(chan *batchMessage, _bufferLength),
	}
	go bw.handleMsg()
	bw.TurnOn()
	return bw
}

func (bw *batchWriter) handleMsg() {
	for {
		select {
		case msg, more := <-bw.msgBuffer:
			if !more {
				bw.closeCurBatch()
				return
			}
			bw.addBatch(msg, more)
		}
	}
}

func (bw *batchWriter) closeCurBatch() {
	bw.mu.Lock()
	defer bw.mu.Unlock()
	if bw.curBatch != nil {
		bw.curBatch.ready <- struct{}{}
		close(bw.curBatch.ready)
	}
}

func (bw *batchWriter) addBatch(msg *batchMessage, more bool) {
	bw.mu.Lock()
	defer bw.mu.Unlock()
	if bw.curBatch == nil {
		bw.curBatch = bw.newBatch(bw.cfg.msgInterval, bw.cfg.sizeLimit)
	}
	bw.extendTimeout()
	bw.curBatch.Add(msg)
	if bw.curBatch.Full() {
		bw.curBatch.Flush()
		bw.curBatch = nil
	}
}

func (bw *batchWriter) newBatch(msgInterval time.Duration, limit uint64) *batch {
	batch := &batch{
		msgs:  make([]*batchMessage, 0),
		bytes: 0,
		limit: limit,
		timer: time.NewTimer(msgInterval),
		ready: make(chan struct{}),
	}
	go bw.awaitBatch(batch)
	return batch
}

func (bw *batchWriter) awaitBatch(b *batch) {
	select {
	case <-b.timer.C:
	case <-b.ready:
	}
	bw.mu.Lock()
	if bw.curBatch == b {
		bw.curBatch = nil
	}
	bw.mu.Unlock()
	bw.manager.assembleQueue <- b
	b.timer.Stop()
}

func (bw *batchWriter) extendTimeout() {
	atomic.SwapUint32(&bw.timeoutTimes, 0)
}

func (bw *batchWriter) Put(msg *batchMessage) error {
	if !bw.IsReady() {
		return errors.New("writer hasn't started yet")
	}
	select {
	case bw.msgBuffer <- msg:
		return nil
	default:
		return errors.New("the msg buffer of writer is full")
	}
}

func (bw *batchWriter) AddTimeoutTimes() {
	atomic.AddUint32(&bw.timeoutTimes, 1)
}

func (bw *batchWriter) Expired() bool {
	return atomic.LoadUint32(&bw.timeoutTimes) >= bw.cfg.expiredThreshold
}

func (bw *batchWriter) Close() {
	bw.TurnOff()
	close(bw.msgBuffer)
}

type batch struct {
	msgs  []*batchMessage
	bytes uint64
	limit uint64
	timer *time.Timer
	ready chan struct{}
}

func (b *batch) Count() int {
	return len(b.msgs)
}

func (b *batch) DataSize() uint64 {
	return b.bytes
}

func (b *batch) Add(msg *batchMessage) {
	b.bytes += msg.Size()
	b.msgs = append(b.msgs, msg)
}

func (b *batch) Full() bool {
	return b.bytes >= b.limit
}

func (b *batch) Flush() {
	b.ready <- struct{}{}
	close(b.ready)
}

type batchMessage struct {
	id      *batchID // cache for Message.BatchID()
	MsgType iotexrpc.MessageType
	ChainID uint32
	Target  *peer.AddrInfo // target of broadcast msg is nil
	Data    []byte
}

func (msg *batchMessage) Size() uint64 {
	return uint64(len(msg.Data))
}

func (msg *batchMessage) BatchID() (id batchID, err error) {
	if msg.id != nil {
		id = *msg.id
		return
	}
	buf := new(bytes.Buffer)
	if err = binary.Write(buf, binary.LittleEndian, msg.MsgType); err != nil {
		return
	}
	if err = binary.Write(buf, binary.LittleEndian, msg.ChainID); err != nil {
		return
	}
	var idInBytes []byte
	if msg.Target != nil {
		idInBytes, err = msg.Target.ID.Marshal()
		if err != nil {
			return
		}
	}
	// non-cryptographic fast hash algorithm xxhash is used for generating batchID
	h := xxhash.Sum64(append(buf.Bytes(), idInBytes...))
	id = batchID(h)
	msg.id = &id
	return
}
