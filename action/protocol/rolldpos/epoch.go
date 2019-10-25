// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"context"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/pkg/log"
)

// ProtocolID is the identity of this protocol
const ProtocolID = "rolldpos"

// Protocol defines an epoch protocol
type Protocol struct {
	numCandidateDelegates   uint64
	numDelegates            uint64
	numSubEpochs            uint64
	numSubEpochsDardanelles uint64
	dardanellesHeight       uint64
	dardanellesOn           bool
}

// Option is optional setting for epoch protocol
type Option func(*Protocol) error

// EnableDardanellesSubEpoch will set give numSubEpochs at give height.
func EnableDardanellesSubEpoch(height, numSubEpochs uint64) Option {
	return func(p *Protocol) error {
		p.dardanellesOn = true
		p.numSubEpochsDardanelles = numSubEpochs
		p.dardanellesHeight = height
		return nil
	}
}

// NewProtocol returns a new rolldpos protocol
func NewProtocol(numCandidateDelegates, numDelegates, numSubEpochs uint64, opts ...Option) *Protocol {
	if numCandidateDelegates < numDelegates {
		numCandidateDelegates = numDelegates
	}
	p := &Protocol{
		numCandidateDelegates: numCandidateDelegates,
		numDelegates:          numDelegates,
		numSubEpochs:          numSubEpochs,
	}
	for _, opt := range opts {
		if err := opt(p); err != nil {
			log.S().Panicf("Failed to execute epoch protocol creation option %p: %v", opt, err)
		}
	}
	return p
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

// GetEpochNum returns the number of the epoch for a given height
func (p *Protocol) GetEpochNum(height uint64) uint64 {
	if height == 0 {
		return 0
	}
	if !p.dardanellesOn || height <= p.dardanellesHeight {
		return (height-1)/p.numDelegates/p.numSubEpochs + 1
	}
	dardanellesEpoch := p.GetEpochNum(p.dardanellesHeight)
	dardanellesEpochHeight := p.GetEpochHeight(dardanellesEpoch)
	return dardanellesEpoch + (height-dardanellesEpochHeight)/p.numDelegates/p.numSubEpochsDardanelles
}

// GetEpochHeight returns the start height of an epoch
func (p *Protocol) GetEpochHeight(epochNum uint64) uint64 {
	if epochNum == 0 {
		return 0
	}
	dardanellesEpoch := p.GetEpochNum(p.dardanellesHeight)
	if !p.dardanellesOn || epochNum <= dardanellesEpoch {
		return (epochNum-1)*p.numDelegates*p.numSubEpochs + 1
	}
	dardanellesEpochHeight := p.GetEpochHeight(dardanellesEpoch)
	return dardanellesEpochHeight + (epochNum-dardanellesEpoch)*p.numDelegates*p.numSubEpochsDardanelles
}

// GetEpochLastBlockHeight returns the last height of an epoch
func (p *Protocol) GetEpochLastBlockHeight(epochNum uint64) uint64 {
	return p.GetEpochHeight(epochNum+1) - 1
}

// GetSubEpochNum returns the sub epoch number of a block height
func (p *Protocol) GetSubEpochNum(height uint64) uint64 {
	return (height - p.GetEpochHeight(p.GetEpochNum(height))) / p.numDelegates
}
