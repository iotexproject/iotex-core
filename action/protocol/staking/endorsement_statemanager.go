package staking

import (
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/state"
)

type (
	// EndorsementStateManager defines the interface of endorsement state manager
	EndorsementStateManager struct {
		protocol.StateManager
		*EndorsementStateReader
	}
	// EndorsementStateReader defines the interface of endorsement state reader
	EndorsementStateReader struct {
		protocol.StateReader
	}
)

// NewEndorsementStateManager creates a new endorsement state manager
func NewEndorsementStateManager(sm protocol.StateManager) *EndorsementStateManager {
	return &EndorsementStateManager{
		StateManager:           sm,
		EndorsementStateReader: NewEndorsementStateReader(sm),
	}
}

// Put puts the endorsement of a bucket
func (esm *EndorsementStateManager) Put(bucketIndex uint64, endorse *Endorsement) error {
	_, err := esm.PutState(endorse, protocol.NamespaceOption(_stakingNameSpace), protocol.KeyOption(endorsementKey(bucketIndex)))
	return err
}

// NewEndorsementStateReader creates a new endorsement state reader
func NewEndorsementStateReader(sr protocol.StateReader) *EndorsementStateReader {
	return &EndorsementStateReader{StateReader: sr}
}

// Get gets the endorsement of a bucket
func (esm *EndorsementStateReader) Get(bucketIndex uint64) (*Endorsement, error) {
	value := Endorsement{}
	switch _, err := esm.State(&value, protocol.NamespaceOption(_stakingNameSpace), protocol.KeyOption(endorsementKey(bucketIndex))); errors.Cause(err) {
	case nil:
		return &value, nil
	case state.ErrStateNotExist:
		return nil, nil
	default:
		return nil, err
	}
}

func endorsementKey(bucketIndex uint64) []byte {
	key := []byte{_endorsement}
	return append(key, byteutil.Uint64ToBytesBigEndian(bucketIndex)...)
}
