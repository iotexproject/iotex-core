package api

import (
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/go-pkgs/cache/ttl"

	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/pkg/log"
)

var (
	errorResponderAdded    = errors.New("Responder already added")
	errorKeyIsNotResponder = errors.New("key does not implement Responder interface")
	errorCapacityReached   = errors.New("capacity has been reached")
)

type (
	// Listener pass new block to all responders
	Listener interface {
		Start() error
		Stop() error
		ReceiveBlock(*block.Block) error
		AddResponder(Responder) error
	}

	// chainListener implements the Listener interface
	chainListener struct {
		capacity  int
		streamMap *ttl.Cache // all registered <Responder, chan error>
	}
)

// NewChainListener returns a new blockchain chainListener
func NewChainListener(c int) Listener {
	s, _ := ttl.NewCache(ttl.EvictOnErrorOption())
	return &chainListener{
		capacity:  c,
		streamMap: s,
	}
}

// Start starts the chainListener
func (cl *chainListener) Start() error {
	return nil
}

// Stop stops the block chainListener
func (cl *chainListener) Stop() error {
	// notify all responders to exit
	cl.streamMap.Range(func(key, _ interface{}) error {
		r, ok := key.(Responder)
		if !ok {
			log.L().Error("streamMap stores a key which is not a Responder")
			return errorKeyIsNotResponder
		}
		r.Exit()
		return nil
	})
	cl.streamMap.Reset()
	return nil
}

// ReceiveBlock handles the block
func (cl *chainListener) ReceiveBlock(blk *block.Block) error {
	// pass the block to every responder
	cl.streamMap.Range(func(key, _ interface{}) error {
		r, ok := key.(Responder)
		if !ok {
			log.L().Error("streamMap stores a key which is not a Responder")
			return errorKeyIsNotResponder
		}
		err := r.Respond(blk)
		if err != nil {
			log.L().Error("responder failed to process block", zap.Error(err))

		}
		return err
	})
	return nil
}

// AddResponder adds a new responder
func (cl *chainListener) AddResponder(r Responder) error {
	_, loaded := cl.streamMap.Get(r)
	if loaded {
		return errorResponderAdded
	}
	if cl.streamMap.Count() >= cl.capacity {
		return errorCapacityReached
	}
	cl.streamMap.Set(r, struct{}{})
	return nil
}
