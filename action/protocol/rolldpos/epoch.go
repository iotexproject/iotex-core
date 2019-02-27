// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"context"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
)

// ProtocolID is the identity of this protocol
const ProtocolID = "rolldpos"

// Protocol defines an epoch protocol
type Protocol struct {
	numCandidateDelegates uint64
	numDelegates          uint64
	numSubEpochs          uint64
}

// NewProtocol returns a new rolldpos protocol
func NewProtocol(numCandidateDelegates uint64, numDelegates uint64, numSubEpochs uint64) *Protocol {
	if numCandidateDelegates < numDelegates {
		numCandidateDelegates = numDelegates
	}
	return &Protocol{
		numCandidateDelegates: numCandidateDelegates,
		numDelegates:          numDelegates,
		numSubEpochs:          numSubEpochs,
	}
}

// Handle handles a modification
func (p *Protocol) Handle(context.Context, action.Action, protocol.StateManager) (*action.Receipt, error) {
	return nil, nil
}

// ReadState read the state on blockchain via protocol
func (p *Protocol) ReadState(context.Context, protocol.StateManager, []byte, ...[]byte) ([]byte, error) {
	return nil, protocol.ErrUnimplemented
}

// Validate validates a modification
func (p *Protocol) Validate(context.Context, action.Action) error {
	return nil
}

// NumCandidateDelegates returns the number of delegate candidates for an epoch
func (p *Protocol) NumCandidateDelegates() uint64 {
	return p.numCandidateDelegates
}

// NumDelegates returns the number of delegates in an epoch
func (p *Protocol) NumDelegates() uint64 {
	return p.numDelegates
}

// NumSubEpochs returns the number of sub-epochs in an epoch
func (p *Protocol) NumSubEpochs() uint64 {
	return p.numSubEpochs
}

// GetEpochNum returns the number of the epoch for a given height
func (p *Protocol) GetEpochNum(height uint64) uint64 {
	if height == 0 {
		return 0
	}
	return (height-1)/p.numDelegates/p.numSubEpochs + 1
}

// GetEpochHeight returns the start height of an epoch
func (p *Protocol) GetEpochHeight(epochNum uint64) uint64 {
	if epochNum == 0 {
		return 0
	}
	return (epochNum-1)*p.numDelegates*p.numSubEpochs + 1
}

// GetEpochLastBlockHeight returns the last height of an epoch
func (p *Protocol) GetEpochLastBlockHeight(epochNum uint64) uint64 {
	if epochNum == 0 {
		return 0
	}
	return epochNum * p.numDelegates * p.numSubEpochs
}

// GetSubEpochNum returns the sub epoch number of a block height
func (p *Protocol) GetSubEpochNum(height uint64) uint64 {
	if height == 0 {
		return 0
	}
	return (height - 1) % (p.numDelegates * p.numSubEpochs) / p.numDelegates
}
