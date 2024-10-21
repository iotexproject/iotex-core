package staking

import (
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/v2/state"
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

// Delete deletes the endorsement of a bucket
func (esm *EndorsementStateManager) Delete(bucketIndex uint64) error {
	_, err := esm.DelState(protocol.NamespaceOption(_stakingNameSpace), protocol.KeyOption(endorsementKey(bucketIndex)))
	return err
}

// NewEndorsementStateReader creates a new endorsement state reader
func NewEndorsementStateReader(sr protocol.StateReader) *EndorsementStateReader {
	return &EndorsementStateReader{StateReader: sr}
}

// Get gets the endorsement of a bucket
func (esr *EndorsementStateReader) Get(bucketIndex uint64) (*Endorsement, error) {
	value := Endorsement{}
	if _, err := esr.State(&value, protocol.NamespaceOption(_stakingNameSpace), protocol.KeyOption(endorsementKey(bucketIndex))); err != nil {
		return nil, err
	}
	return &value, nil
}

// Status returns the status of the endorsement of a bucket at a certain height
// If the endorsement does not exist, it returns EndorseExpired
func (esr *EndorsementStateReader) Status(ctx protocol.FeatureCtx, bucketIndex, height uint64) (EndorsementStatus, error) {
	var status EndorsementStatus
	endorse, err := esr.Get(bucketIndex)
	switch errors.Cause(err) {
	case nil:
		if ctx.EnforceLegacyEndorsement {
			status = endorse.LegacyStatus(height)
		} else {
			status = endorse.Status(height)
		}
	case state.ErrStateNotExist:
		status = EndorseExpired
		err = nil
	default:
	}
	return status, err
}

func endorsementKey(bucketIndex uint64) []byte {
	key := []byte{_endorsement}
	return append(key, byteutil.Uint64ToBytesBigEndian(bucketIndex)...)
}
