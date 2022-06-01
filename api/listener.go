package api

import (
	"encoding/hex"
	"math/rand"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/go-pkgs/cache/ttl"

	apitypes "github.com/iotexproject/iotex-core/api/types"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/pkg/log"
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

// TODO: add lock to enable thread safety
// AddResponder adds a new responder
func (cl *chainListener) AddResponder(responder apitypes.Responder) (string, error) {
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
	return listenerID, nil
}

// RemoveResponder delete the responder
func (cl *chainListener) RemoveResponder(listenerID string) (bool, error) {
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
	return cl.streamMap.Delete(listenerID), nil
}

type randID struct {
	length uint8
}

func newIDGenerator(length uint8) *randID {
	rand.Seed(time.Now().UnixNano())
	return &randID{length: length}
}

func (id *randID) newID() string {
	token := make([]byte, id.length)
	rand.Read(token)
	return "0x" + hex.EncodeToString(token)
}
