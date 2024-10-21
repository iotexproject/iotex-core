package batch

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cespare/xxhash/v2"
	goproto "github.com/iotexproject/iotex-proto/golang"
	"github.com/iotexproject/iotex-proto/golang/iotexrpc"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
)

const (
	_bufferLength    = 500
	_maxWriters      = 5000
	_cleanupInterval = 15 * time.Minute
)

// Option sets parameter for batch
type Option func(cfg *writerConfig)

// WithInterval sets batch with time interval
func WithInterval(t time.Duration) Option {
	return func(cfg *writerConfig) {
		cfg.msgInterval = t
	}
}

// WithSizeLimit sets batch with limited size
func WithSizeLimit(limit uint64) Option {
	return func(cfg *writerConfig) {
		cfg.sizeLimit = limit
	}
}

type (
	// Manager is the manager of batch component
	Manager struct {
		mu             sync.RWMutex
		writerMap      map[batchID]*batchWriter
		outputQueue    chan *Message // assembled message queue for external reader
		assembleQueue  chan *batch   // batch queue which collects batches sent from writers
		messageHandler messageOutbound
		cancelHanlders context.CancelFunc
	}

	batchID uint64

	messageOutbound func(*Message) error
)

// NewManager creates a new Manager with callback
func NewManager(handler messageOutbound) *Manager {
	return &Manager{
		writerMap:      make(map[batchID]*batchWriter),
		outputQueue:    make(chan *Message, _bufferLength),
		assembleQueue:  make(chan *batch, _bufferLength),
		messageHandler: handler,
	}
}

// Start start the Manager
func (bm *Manager) Start() error {
	ctx, cancel := context.WithCancel(context.Background())
	go bm.assemble(ctx)
	go bm.cleanup(ctx, _cleanupInterval)
	go bm.callback(ctx)
	bm.cancelHanlders = cancel
	return nil
}

// Stop stops the Manager
func (bm *Manager) Stop() error {
	bm.cancelHanlders()
	return nil
}

// Put puts a batchmessage into the Manager
func (bm *Manager) Put(msg *Message, opts ...Option) error {
	if !bm.supported(msg.messageType()) {
		return errors.New("message is unsupported for batching")
	}
	id, err := msg.batchID()
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

func (bm *Manager) supported(msgType iotexrpc.MessageType) bool {
	return msgType == iotexrpc.MessageType_ACTION
}

func (bm *Manager) assemble(ctx context.Context) {
	for {
		select {
		case batch := <-bm.assembleQueue:
			if batch.Size() == 0 {
				continue
			}
			var (
				msg0 = batch.msgs[0]
			)
			bm.outputQueue <- &Message{
				msgType: msg0.msgType,
				ChainID: msg0.ChainID,
				Target:  msg0.Target,
				Data:    packMessageData(msg0.msgType, batch.msgs),
			}
		case <-ctx.Done():
			return
		}
	}
}

func packMessageData(msgType iotexrpc.MessageType, arr []*Message) proto.Message {
	switch msgType {
	case iotexrpc.MessageType_ACTION:
		actions := make([]*iotextypes.Action, 0, len(arr))
		for i := range arr {
			actions = append(actions, arr[i].Data.(*iotextypes.Action))
		}
		return &iotextypes.Actions{Actions: actions}
	default:
		panic(fmt.Sprintf("the message type %v is not supported", msgType))
	}
}

func (bm *Manager) cleanup(ctx context.Context, interval time.Duration) {
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

func (bm *Manager) callback(ctx context.Context) {
	for {
		select {
		case msg := <-bm.outputQueue:
			err := bm.messageHandler(msg)
			if err != nil {
				log.L().Error("fail to handle a batch message when calling back", zap.Error(err))
			}
		case <-ctx.Done():
			return
		}
	}
}

func (bm *Manager) cleanupLoop() {
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
	sizeLimit:        1000,
}

type batchWriter struct {
	lifecycle.Readiness
	mu sync.RWMutex

	manager *Manager
	cfg     writerConfig

	msgBuffer chan *Message
	curBatch  *batch

	timeoutTimes uint32
}

func newBatchWriter(cfg *writerConfig, manager *Manager) *batchWriter {
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
		msgs:      make([]*Message, 0),
		sizeLimit: limit,
		timer:     time.NewTimer(msgInterval),
		ready:     make(chan struct{}),
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
	msgs      []*Message
	sizeLimit uint64
	timer     *time.Timer
	ready     chan struct{}
}

func (b *batch) Size() int {
	return len(b.msgs)
}

func (b *batch) Add(msg *Message) {
	b.msgs = append(b.msgs, msg)
}

func (b *batch) Full() bool {
	return uint64(len(b.msgs)) >= b.sizeLimit
}

func (b *batch) Flush() {
	b.ready <- struct{}{}
	close(b.ready)
}

// Message is the message to be batching
type Message struct {
	ChainID uint32
	Data    proto.Message
	Target  *peer.AddrInfo       // target of broadcast msg is nil
	id      *batchID             // cache for Message.BatchID()
	msgType iotexrpc.MessageType // generated by MessageType()
}

func (msg *Message) batchID() (id batchID, err error) {
	if msg.id != nil {
		id = *msg.id
		return
	}
	buf := new(bytes.Buffer)
	if err = binary.Write(buf, binary.LittleEndian, msg.messageType()); err != nil {
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

func (msg *Message) messageType() iotexrpc.MessageType {
	if msg.msgType == iotexrpc.MessageType_UNKNOWN {
		var err error
		msg.msgType, err = goproto.GetTypeFromRPCMsg(msg.Data)
		if err != nil {
			return iotexrpc.MessageType_UNKNOWN
		}
	}
	return msg.msgType
}
