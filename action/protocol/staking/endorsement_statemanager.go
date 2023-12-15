package staking

import (
	"errors"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/state"
)

type (
	// EndorsementStateManager defines the interface of endorsement state manager
	EndorsementStateManager struct {
		protocol.StateManager
	}
)

// NewEndorsementStateManager creates a new endorsement state manager
func NewEndorsementStateManager(sm protocol.StateManager) *EndorsementStateManager {
	return &EndorsementStateManager{StateManager: sm}
}

// Put puts the endorsement of a bucket
func (esm *EndorsementStateManager) Put(bucketIndex uint64, endorse *Endorsement) error {
	key := endorsementKey(bucketIndex)
	if _, err := esm.PutState(endorse, protocol.NamespaceOption(_stakingNameSpace), protocol.KeyOption(key)); err != nil {
		return err
	}
	return nil
}

// Get gets the endorsement of a bucket
func (esm *EndorsementStateManager) Get(bucketIndex uint64) (*Endorsement, error) {
	key := endorsementKey(bucketIndex)
	value := Endorsement{}
	if _, err := esm.State(&value, protocol.NamespaceOption(_stakingNameSpace), protocol.KeyOption(key)); err != nil {
		if errors.Is(err, state.ErrStateNotExist) {
			return nil, nil
		}
		return nil, err
	}
	return &value, nil
}

func endorsementKey(bucketIndex uint64) []byte {
	key := []byte{_endorsement}
	return append(key, byteutil.Uint64ToBytesBigEndian(bucketIndex)...)
}
