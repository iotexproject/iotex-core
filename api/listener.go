package api

import (
	"github.com/pkg/errors"

	"github.com/iotexproject/go-pkgs/cache"

	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/pkg/log"
)

var (
	errorResponderAdded = errors.New("Responder already added")
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
		streamMap *cache.ThreadSafeLruCache // all registered <Responder, chan error>
	}
)

// NewChainListener returns a new blockchain chainListener
func NewChainListener() Listener {
	return &chainListener{
		streamMap: cache.NewThreadSafeLruCache(0),
	}
}

// Start starts the chainListener
func (cl *chainListener) Start() error {
	return nil
}

// Stop stops the block chainListener
func (cl *chainListener) Stop() error {
	// notify all responders to exit
	cl.streamMap.Range(func(key cache.Key, _ interface{}) bool {
		r, ok := key.(Responder)
		if !ok {
			log.S().Panic("streamMap stores a key which is not a Responder")
		}
		r.Exit()
		cl.streamMap.Remove(key)
		return true
	})
	return nil
}

// ReceiveBlock handles the block
func (cl *chainListener) ReceiveBlock(blk *block.Block) error {
	// pass the block to every responder
	cl.streamMap.Range(func(key cache.Key, _ interface{}) bool {
		r, ok := key.(Responder)
		if !ok {
			log.S().Panic("streamMap stores a key which is not a Responder")
		}
		if err := r.Respond(blk); err != nil {
			cl.streamMap.Remove(key)
		}
		return true
	})
	return nil
}

// AddResponder adds a new responder
func (cl *chainListener) AddResponder(r Responder) error {
	_, loaded := cl.streamMap.Get(r)
	if loaded {
		return errorResponderAdded
	}
	cl.streamMap.Add(r, struct{}{})
	return nil
}
