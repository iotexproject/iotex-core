package api

import (
	"context"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	batch "github.com/iotexproject/iotex-core/v2/pkg/messagebatcher"
)

// ActionRadioOption is the option to create ActionRadio
type ActionRadioOption func(*ActionRadio)

// WithMessageBatch enables message batching
func WithMessageBatch() ActionRadioOption {
	return func(ar *ActionRadio) {
		ar.messageBatcher = batch.NewManager(func(msg *batch.Message) error {
			return ar.broadcastHandler(context.Background(), ar.chainID, msg.Data)
		})
	}
}

// WithRetry enables retry for action broadcast
func WithRetry(fetchFn func() chan *action.SealedEnvelope, max int, interval time.Duration) ActionRadioOption {
	return func(ar *ActionRadio) {
		ar.fetchFn = fetchFn
		ar.retryMax = max
		ar.retryInterval = interval
		ar.tickInterval = time.Second * 10
	}
}

// ActionRadio broadcasts actions to the network
type ActionRadio struct {
	broadcastHandler BroadcastOutbound
	messageBatcher   *batch.Manager
	chainID          uint32
	unconfirmedActs  map[hash.Hash256]*radioAction
	mutex            sync.Mutex
	quit             chan struct{}
	fetchFn          func() chan *action.SealedEnvelope
	retryMax         int
	retryInterval    time.Duration
	tickInterval     time.Duration
}

type radioAction struct {
	act           *action.SealedEnvelope
	lastRadioTime time.Time
	retry         int
}

// NewActionRadio creates a new ActionRadio
func NewActionRadio(broadcastHandler BroadcastOutbound, chainID uint32, opts ...ActionRadioOption) *ActionRadio {
	ar := &ActionRadio{
		broadcastHandler: broadcastHandler,
		chainID:          chainID,
		unconfirmedActs:  make(map[hash.Hash256]*radioAction),
		quit:             make(chan struct{}),
	}
	for _, opt := range opts {
		opt(ar)
	}
	return ar
}

// Start starts the action radio
func (ar *ActionRadio) Start() error {
	if ar.messageBatcher != nil {
		return ar.messageBatcher.Start()
	}
	if ar.tickInterval > 0 {
		go func() {
			ticker := time.NewTicker(ar.tickInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					ar.mutex.Lock()
					ar.autoRadio()
					ar.mutex.Unlock()
				case <-ar.quit:
					break
				}
			}
		}()
	}
	return nil
}

// Stop stops the action radio
func (ar *ActionRadio) Stop() error {
	close(ar.quit)
	if ar.messageBatcher != nil {
		return ar.messageBatcher.Stop()
	}
	return nil
}

// OnAdded broadcasts the action to the network
func (ar *ActionRadio) OnAdded(selp *action.SealedEnvelope) {
	ar.mutex.Lock()
	defer ar.mutex.Unlock()
	ar.radio(selp)
}

// OnRemoved does nothing
func (ar *ActionRadio) OnRemoved(selp *action.SealedEnvelope) {
	ar.mutex.Lock()
	defer ar.mutex.Unlock()
	hash, _ := selp.Hash()
	delete(ar.unconfirmedActs, hash)
}

func (ar *ActionRadio) OnRejected(ctx context.Context, selp *action.SealedEnvelope, err error) {
	if !errors.Is(err, action.ErrExistedInPool) {
		return
	}
	if _, fromAPI := GetAPIContext(ctx); fromAPI {
		// ignore action rejected from API
		return
	}
	// retry+1 for action broadcast from other nodes, alleviate the network congestion
	hash, _ := selp.Hash()
	ar.mutex.Lock()
	defer ar.mutex.Unlock()
	if radioAct, ok := ar.unconfirmedActs[hash]; ok {
		radioAct.retry++
		radioAct.lastRadioTime = time.Now()
	} else {
		log.L().Warn("Found rejected action not in unconfirmedActs", zap.String("actionHash", hex.EncodeToString(hash[:])))
	}
}

// autoRadio broadcasts long time pending actions periodically
func (ar *ActionRadio) autoRadio() {
	now := time.Now()
	for pending := range ar.fetchFn() {
		hash, _ := pending.Hash()
		if radioAct, ok := ar.unconfirmedActs[hash]; ok {
			if radioAct.retry < ar.retryMax && now.Sub(radioAct.lastRadioTime) > ar.retryInterval {
				ar.radio(radioAct.act)
			}
			continue
		}
		// wired case, add it to unconfirmedActs and broadcast it
		log.L().Warn("Found missing pending action", zap.String("actionHash", hex.EncodeToString(hash[:])))
		ar.radio(pending)
	}
}

func (ar *ActionRadio) radio(selp *action.SealedEnvelope) {
	// broadcast action
	var (
		hasSidecar = selp.BlobTxSidecar() != nil
		hash, _    = selp.Hash()
		out        proto.Message
		err        error
	)
	if hasSidecar {
		out = &iotextypes.ActionHash{
			Hash: hash[:],
		}
	} else {
		out = selp.Proto()
	}
	if ar.messageBatcher != nil && !hasSidecar { // TODO: batch blobTx
		err = ar.messageBatcher.Put(&batch.Message{
			ChainID: ar.chainID,
			Target:  nil,
			Data:    out,
		})
	} else {
		err = ar.broadcastHandler(context.Background(), ar.chainID, out)
	}
	if err != nil {
		log.L().Warn("Failed to broadcast action.", zap.Error(err), zap.String("actionHash", hex.EncodeToString(hash[:])))
	}
	// update unconfirmed action
	if radio, ok := ar.unconfirmedActs[hash]; ok {
		radio.lastRadioTime = time.Now()
		radio.retry++
	} else {
		ar.unconfirmedActs[hash] = &radioAction{
			act:           selp,
			lastRadioTime: time.Now(),
		}
	}
}
