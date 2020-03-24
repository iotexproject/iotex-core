// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"context"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

const protocolID = "rolldpos"

// Protocol defines an epoch protocol
type Protocol struct {
	numCandidateDelegates   uint64
	numDelegates            uint64
	numSubEpochs            uint64
	numSubEpochsDardanelles uint64
	dardanellesHeight       uint64
	dardanellesOn           bool
}

// FindProtocol return a registered protocol from registry
func FindProtocol(registry *protocol.Registry) *Protocol {
	if registry == nil {
		return nil
	}
	p, ok := registry.Find(protocolID)
	if !ok {
		return nil
	}
	rp, ok := p.(*Protocol)
	if !ok {
		log.S().Panic("fail to cast rolldpos protocol")
	}
	return rp
}

// MustGetProtocol return a registered protocol from registry
func MustGetProtocol(registry *protocol.Registry) *Protocol {
	if registry == nil {
		log.S().Panic("registry cannot be nil")
	}
	p, ok := registry.Find(protocolID)
	if !ok {
		log.S().Panic("rolldpos protocol is not registered")
	}
	rp, ok := p.(*Protocol)
	if !ok {
		log.S().Panic("fail to cast to rolldpos protocol")
	}
	return rp
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
func (p *Protocol) ReadState(ctx context.Context, _ protocol.StateReader, method []byte, args ...[]byte) ([]byte, error) {
	switch string(method) {
	case "NumCandidateDelegates":
		return byteutil.Uint64ToBytes(p.numCandidateDelegates), nil
	case "NumDelegates":
		return byteutil.Uint64ToBytes(p.numDelegates), nil
	case "NumSubEpochs":
		if len(args) != 1 {
			return nil, errors.Errorf("invalid number of arguments %d", len(args))
		}
		numSubEpochs := p.NumSubEpochs(byteutil.BytesToUint64(args[0]))
		return byteutil.Uint64ToBytes(numSubEpochs), nil
	case "EpochNumber":
		if len(args) != 1 {
			return nil, errors.Errorf("invalid number of arguments %d", len(args))
		}
		epochNumber := p.GetEpochNum(byteutil.BytesToUint64(args[0]))
		return byteutil.Uint64ToBytes(epochNumber), nil
	case "EpochHeight":
		if len(args) != 1 {
			return nil, errors.Errorf("invalid number of arguments %d", len(args))
		}
		epochHeight := p.GetEpochHeight(byteutil.BytesToUint64(args[0]))
		return byteutil.Uint64ToBytes(epochHeight), nil
	case "EpochLastHeight":
		if len(args) != 1 {
			return nil, errors.Errorf("invalid number of arguments %d", len(args))
		}
		epochLastHeight := p.GetEpochLastBlockHeight(byteutil.BytesToUint64(args[0]))
		return byteutil.Uint64ToBytes(epochLastHeight), nil
	case "SubEpochNumber":
		if len(args) != 1 {
			return nil, errors.Errorf("invalid number of arguments %d", len(args))
		}
		subEpochNumber := p.GetSubEpochNum(byteutil.BytesToUint64(args[0]))
		return byteutil.Uint64ToBytes(subEpochNumber), nil
	default:
		return nil, errors.New("corresponding method isn't found")
	}
}

// Register registers the protocol with a unique ID
func (p *Protocol) Register(r *protocol.Registry) error {
	return r.Register(protocolID, p)
}

// ForceRegister registers the protocol with a unique ID and force replacing the previous protocol if it exists
func (p *Protocol) ForceRegister(r *protocol.Registry) error {
	return r.ForceRegister(protocolID, p)
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

// NumSubEpochs returns the number of subEpochs given a block height
func (p *Protocol) NumSubEpochs(height uint64) uint64 {
	if !p.dardanellesOn || height < p.dardanellesHeight {
		return p.numSubEpochs
	}
	return p.numSubEpochsDardanelles
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

// ProductivityByEpoch read the productivity in an epoch
func (p *Protocol) ProductivityByEpoch(
	epochNum uint64,
	tipHeight uint64,
	productivity func(uint64, uint64) (map[string]uint64, error),
) (uint64, map[string]uint64, error) {
	if tipHeight == 0 {
		return 0, map[string]uint64{}, nil
	}
	currentEpochNum := p.GetEpochNum(tipHeight)
	if epochNum > currentEpochNum {
		return 0, nil, errors.Errorf("epoch number %d is larger than current epoch number %d", epochNum, currentEpochNum)
	}
	epochStartHeight := p.GetEpochHeight(epochNum)
	var epochEndHeight uint64
	if epochNum == currentEpochNum {
		epochEndHeight = tipHeight
	} else {
		epochEndHeight = p.GetEpochLastBlockHeight(epochNum)
	}
	produce, err := productivity(epochStartHeight, epochEndHeight)
	return epochEndHeight - epochStartHeight + 1, produce, err
}
