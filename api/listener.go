package api

import (
	"encoding/hex"
	"sync"

	"github.com/iotexproject/go-pkgs/cache/ttl"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	apitypes "github.com/iotexproject/iotex-core/v2/api/types"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/pkg/fastrand"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
)

const (
	_idSize  = 16
	_idRetry = 1e6
)

var (
	errorUnsupportedType = errors.New("type is unsupported")
	errorCapacityReached = errors.New("capacity has been reached")
	errListenerNotFound  = errors.New("subscription not found")
)

type (
	// chainListener implements the Listener interface
	chainListener struct {
		maxCapacity int
		streamMap   *ttl.Cache // all registered <Responder, chan error>
		idGenerator *randID
		mu          sync.Mutex
	}
)

// NewChainListener returns a new blockchain chainListener
func NewChainListener(c int) apitypes.Listener {
	s, _ := ttl.NewCache(ttl.EvictOnErrorOption())
	return &chainListener{
		maxCapacity: c,
		streamMap:   s,
		idGenerator: newIDGenerator(_idSize),
	}
}

// Start starts the chainListener
func (cl *chainListener) Start() error {
	return nil
}

// Stop stops the block chainListener
func (cl *chainListener) Stop() error {
	// notify all responders to exit
	cl.streamMap.Range(func(_, value interface{}) error {
		r, ok := value.(apitypes.Responder)
		if !ok {
			log.L().Error("streamMap stores a value which is not a Responder")
			return errorUnsupportedType
		}
		r.Exit()
		return nil
	})
	cl.streamMap.Reset()
	apiLimitMtcs.WithLabelValues("listener").Set(float64(cl.streamMap.Count()))
	return nil
}

// ReceiveBlock handles the block
func (cl *chainListener) ReceiveBlock(blk *block.Block) error {
	// pass the block to every responder
	cl.streamMap.Range(func(key, value interface{}) error {
		r, ok := value.(apitypes.Responder)
		if !ok {
			log.L().Error("streamMap stores a value which is not a Responder")
			return errorUnsupportedType
		}
		err := r.Respond(key.(string), blk)
		if err != nil {
			log.L().Error("responder failed to process block", zap.Error(err))
		}
		return err
	})
	return nil
}

// AddResponder adds a new responder
func (cl *chainListener) AddResponder(responder apitypes.Responder) (string, error) {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	if cl.streamMap.Count() >= cl.maxCapacity {
		return "", errorCapacityReached
	}

	listenerID, i := "", 0
	// An new id is assumed to be found, because combinations (2^62) is far larger than capacity
	for ; i < _idRetry; i++ {
		listenerID = cl.idGenerator.newID()
		if _, exist := cl.streamMap.Get(listenerID); !exist {
			break
		}
	}
	if i == _idRetry {
		return "", errors.New("No peer id is available")
	}

	cl.streamMap.Set(listenerID, responder)
	apiLimitMtcs.WithLabelValues("listener").Set(float64(cl.streamMap.Count()))
	return listenerID, nil
}

// RemoveResponder delete the responder
func (cl *chainListener) RemoveResponder(listenerID string) (bool, error) {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	value, exist := cl.streamMap.Get(listenerID)
	if !exist {
		return false, errListenerNotFound
	}
	r, ok := value.(apitypes.Responder)
	if !ok {
		log.L().Error("streamMap stores a value which is not a Responder")
		return false, errListenerNotFound
	}
	r.Exit()
	apiLimitMtcs.WithLabelValues("listener").Set(float64(cl.streamMap.Count() - 1))
	return cl.streamMap.Delete(listenerID), nil
}

type randID struct {
	length uint8
}

func newIDGenerator(length uint8) *randID {
	return &randID{length: length}
}

func (id *randID) newID() string {
	token := make([]byte, id.length)
	fastrand.Read(token)
	return "0x" + hex.EncodeToString(token)
}
