// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package protocol

import (
	"sync"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/pkg/log"
)

const (
	// AccountProtocolID is account protocol ID
	AccountProtocolID = "account"
	// ExecutionProtocolID is execution protocol ID
	ExecutionProtocolID = "execution"
	// MainChainProtocolID is main-chain protocol ID
	MainChainProtocolID = "multi-chain_main-chain"
	// SubChainProtocolID is sub-chain protocol ID
	SubChainProtocolID = "multi-chain_sub-chain"
	// PollProtocolID is poll protocol ID
	PollProtocolID = "poll"
	// RewardingProtocolID is rewarding protocol ID
	RewardingProtocolID = "rewarding"
	// RollDPoSProtocolID is roll protocol ID
	RollDPoSProtocolID = "rolldpos"
)

// TODO: avoid global variable
var activeProtocols = make(map[string]interface{})

// ActiveProtocols returns the active protocol IDs
func ActiveProtocols(_ uint64) map[string]interface{} {
	if len(activeProtocols) > 0 {
		return activeProtocols
	}
	// The following is not set in the default genesis config to prevent breaking the genesis hash for mainnet.
	return map[string]interface{}{
		AccountProtocolID:   nil,
		ExecutionProtocolID: nil,
		MainChainProtocolID: nil,
		SubChainProtocolID:  nil,
		PollProtocolID:      nil,
		RewardingProtocolID: nil,
		RollDPoSProtocolID:  nil,
	}
}

// OverrideLifeLongActiveProtocols overrides the default active protocols
func OverrideLifeLongActiveProtocols(activeProtocolsSlice []string) {
	for _, activeProtocol := range activeProtocolsSlice {
		activeProtocols[activeProtocol] = nil
	}
}

// Registry is the hub of all protocols deployed on the chain
type Registry struct {
	protocols sync.Map
}

// Register registers the protocol with a unique ID
func (r *Registry) Register(id string, p Protocol) error {
	_, loaded := r.protocols.LoadOrStore(id, p)
	if loaded {
		return errors.Errorf("Protocol with ID %s is already registered", id)
	}
	return nil
}

// ForceRegister registers the protocol with a unique ID and force replacing the previous protocol if it exists
func (r *Registry) ForceRegister(id string, p Protocol) error {
	r.protocols.Store(id, p)
	return nil
}

// Find finds a protocol by ID
func (r *Registry) Find(id string) (Protocol, bool) {
	value, ok := r.protocols.Load(id)
	if !ok {
		return nil, false
	}
	p, ok := value.(Protocol)
	if !ok {
		log.S().Panic("Registry stores the item which is not a protocol")
	}
	return p, true
}

// All returns all protocols
func (r *Registry) All() []Protocol {
	all := make([]Protocol, 0)
	r.protocols.Range(func(_, value interface{}) bool {
		p, ok := value.(Protocol)
		if !ok {
			log.S().Panic("Registry stores the item which is not a protocol")
		}
		all = append(all, p)
		return true
	})
	return all
}
