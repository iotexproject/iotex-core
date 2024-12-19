package api

import (
	"context"
	"encoding/hex"

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

// ActionRadio broadcasts actions to the network
type ActionRadio struct {
	broadcastHandler BroadcastOutbound
	messageBatcher   *batch.Manager
	chainID          uint32
}

// NewActionRadio creates a new ActionRadio
func NewActionRadio(broadcastHandler BroadcastOutbound, chainID uint32, opts ...ActionRadioOption) *ActionRadio {
	ar := &ActionRadio{
		broadcastHandler: broadcastHandler,
		chainID:          chainID,
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
	return nil
}

// Stop stops the action radio
func (ar *ActionRadio) Stop() error {
	if ar.messageBatcher != nil {
		return ar.messageBatcher.Stop()
	}
	return nil
}

// OnAdded broadcasts the action to the network
func (ar *ActionRadio) OnAdded(selp *action.SealedEnvelope) {
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
		log.L().Warn("Failed to broadcast SendAction request.", zap.Error(err), zap.String("actionHash", hex.EncodeToString(hash[:])))
	}
}

// OnRemoved does nothing
func (ar *ActionRadio) OnRemoved(act *action.SealedEnvelope) {}
