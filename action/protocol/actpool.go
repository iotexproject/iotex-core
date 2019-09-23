package protocol

import (
	"sync"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
)

// ActPoolInterface is the interface for actpool
type ActPoolInterface struct {
	mu      sync.RWMutex
	actpool ActPoolManager
}

// NewActPoolInterface constructs a new ActPoolInterface
func NewActPoolInterface(actpool ActPoolManager) *ActPoolInterface {
	return &ActPoolInterface{
		actpool: actpool,
	}
}

// GetActionByHash returns the pending action in pool given action's hash
func (v *ActPoolInterface) GetActionByHash(hash hash.Hash256) (action.SealedEnvelope, error) {
	if v.actpool != nil {
		return v.actpool.GetActionByHash(hash)
	}
	return action.SealedEnvelope{}, errors.Wrapf(action.ErrActPool, "actpool is nil")
}
