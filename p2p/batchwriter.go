package p2p

import (
	"bytes"
	"encoding/binary"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/iotexproject/iotex-proto/golang/iotexrpc"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/pkg/lifecycle"
)

// TODO: add stat

const (
	_bufferLength    = 500
	_maxWriters      = 5000
	_cleanupInterval = 15 * time.Minute
)

// BatchOption is for testing
type BatchOption func(cfg *writerConfig)

// WithInterval is for testing
func WithInterval(t time.Duration) BatchOption {
	return func(cfg *writerConfig) {
		cfg.msgInterval = t
	}
}

// WithSizeLimit is for testing
func WithSizeLimit(limit uint64) BatchOption {
	return func(cfg *writerConfig) {
		cfg.sizeLimit = limit
	}
}

type (
	batchManager struct {
		mu            sync.RWMutex
		writerMap     map[BatchID]*batchWriter
		outputQueue   chan Message // assembled message queue for external reader
		assembleQueue chan *batch  // batch queue which collects batches sent from writers
	}

	// BatchID is for testing
	BatchID uint64
)

func newBatchManager() *batchManager {
	bm := &batchManager{
		writerMap:     make(map[BatchID]*batchWriter),
		outputQueue:   make(chan Message, _bufferLength),
		assembleQueue: make(chan *batch, _bufferLength),
	}
	go bm.assemble()
	go bm.cleanup(_cleanupInterval)
	return bm
}

func (bm *batchManager) Put(msg *Message, opts ...BatchOption) error {
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
	if msgType == iotexrpc.MessageType_ACTIONS ||
		msgType == iotexrpc.MessageType_BLOCKS {
		return true
	}
	return false
}

func (bm *batchManager) OutputChannel() chan Message {
	return bm.outputQueue
}

func (bm *batchManager) assemble() {
	for batch := range bm.assembleQueue {
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
		bm.outputQueue <- Message{
			MsgType: msg0.MsgType,
			ChainID: msg0.ChainID,
			Target:  msg0.Target,
			Data:    data,
		}
	}
}

func (bm *batchManager) cleanup(interval time.Duration) {
	ticker := time.NewTicker(interval)
	for range ticker.C {
		bm.cleanupLoop()
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

	msgBuffer chan *Message
	curBatch  *batch

	timeoutTimes uint32
}

func newBatchWriter(cfg *writerConfig, manager *batchManager) *batchWriter {
	bw := &batchWriter{
		manager:   manager,
		cfg:       *cfg,
		msgBuffer: make(chan *Message, _bufferLength),
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

func (bw *batchWriter) addBatch(msg *Message, more bool) {
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
		msgs:  make([]*Message, 0),
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

func (bw *batchWriter) Put(msg *Message) error {
	if !bw.IsReady() {
		return errors.New("writer hasn't started yet")
	}
	bw.msgBuffer <- msg
	return nil
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
	msgs  []*Message
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

func (b *batch) Add(msg *Message) {
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

// Message is for testing
type Message struct {
	id      *BatchID // cache for Message.BatchID()
	MsgType iotexrpc.MessageType
	ChainID uint32
	Target  *peer.AddrInfo // target of broadcast msg is nil
	Data    []byte
}

// Size is for testing
func (msg *Message) Size() uint64 {
	return uint64(len(msg.Data))
}

// BatchID is for testing
func (msg *Message) BatchID() (id BatchID, err error) {
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
	id = BatchID(h)
	msg.id = &id
	return
}
