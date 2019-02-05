// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package registry

import (
	"context"
	"sync"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	prtcl "github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/log"
)

// Protocol is the registry service to bookkeep and discover installed protocols. This is a special protocol to
// bootstrap the system, which should be installed by hard code.
type Protocol struct {
	addressToProtocol sync.Map
}

// NewRegistry instantiates a registry struct
func NewProtocol() *Protocol { return &Protocol{} }

// Address is the protocol address
func (p *Protocol) Address() address.Address { return address.New(hash.ZeroPKHash[:]) }

// Tags are the tags to describe the protocol
func (p *Protocol) Tags() [][]byte { return [][]byte{[]byte("registry")} }

// Handle handles an account
func (p *Protocol) Handle(ctx context.Context, act action.Action, sm prtcl.StateManager) (*action.Receipt, error) {
	return nil, nil
}

// Validate validates an account
func (p *Protocol) Validate(ctx context.Context, act action.Action) error {
	return nil
}

// Register registers a protocol. If the same address has been used by other protocol already, an error will be returned.
func (p *Protocol) Register(protocol prtcl.Protocol) error {
	if _, loaded := p.addressToProtocol.LoadOrStore(protocol.Address().Bech32(), protocol); loaded {
		return errors.Errorf(
			"address %s is already registered with some other protocol",
			protocol.Address().Bech32(),
		)
	}
	return nil
}

// Get gets a protocol by its address.
func (p *Protocol) Get(addr address.Address) (prtcl.Protocol, bool) {
	value, ok := p.addressToProtocol.Load(addr.Bech32())
	if !ok {
		return nil, false
	}
	protocol, ok := value.(prtcl.Protocol)
	if !ok {
		log.S().Panic("Protocol registry has store some struct that is not a protocol")
	}
	return protocol, true
}

// FindFirst finds the first protocol that meets the criteria specified by the callback function. Order is not
// guaranteed
func (p *Protocol) FindFirst(f func(prtcl.Protocol) bool) (prtcl.Protocol, bool) {
	var protocolToReturn prtcl.Protocol
	var found bool
	p.addressToProtocol.Range(func(_, value interface{}) bool {
		protocol, ok := value.(prtcl.Protocol)
		if !ok {
			log.S().Panic("Protocol registry has store some struct that is not a protocol")
		}
		if ok := f(protocol); ok {
			protocolToReturn = protocol
			found = true
			return false
		}
		return true
	})
	return protocolToReturn, found
}
