package staking

import (
	"sync"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/staking/stakingpb"
	"github.com/iotexproject/iotex-core/state"
)

const (
	_candidateTransferOwnershipProtoVersion float32 = 1.0
)

var (
	_candidateTransferOwnship = []byte("candidateTransferOwnership")
)

type CandidateTransferOwnership struct {
	NameToOwner map[address.Address]address.Address //newOwner -> oldOwner
	mu          sync.RWMutex
}

func newCandidateTransferOwnership() *CandidateTransferOwnership {
	return &CandidateTransferOwnership{
		NameToOwner: make(map[address.Address]address.Address),
		mu:          sync.RWMutex{},
	}
}

// Update updates the candidate transfer ownership
func (c *CandidateTransferOwnership) Update(oldOwner address.Address, newOwner address.Address) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.NameToOwner[newOwner] = oldOwner
}

// Serialize serializes CandidateTransferOwnership to bytes
func (c *CandidateTransferOwnership) Serialize() ([]byte, error) {
	pb := &stakingpb.CandidateTransferOwnership{
		Version:     _candidateTransferOwnershipProtoVersion,
		NameToOwner: make(map[string]string),
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	for k, v := range c.NameToOwner {
		pb.NameToOwner[k.String()] = v.String()
	}
	return proto.Marshal(pb)
}

// Deserialize deserializes bytes to CandidateTransferOwnership
func (c *CandidateTransferOwnership) Deserialize(data []byte) error {
	pb := &stakingpb.CandidateTransferOwnership{}
	if err := proto.Unmarshal(data, pb); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.NameToOwner = make(map[address.Address]address.Address, len(pb.NameToOwner))
	for k, v := range pb.NameToOwner {
		newAddr, err := address.FromString(k)
		if err != nil {
			return err
		}
		oldAddr, err := address.FromString(v)
		if err != nil {
			return err
		}
		c.NameToOwner[newAddr] = oldAddr
	}
	return nil
}

// Load loads the candidate transfer ownership from state manager
func (c *CandidateTransferOwnership) LoadFromStateManager(sm protocol.StateManager) error {
	if _, err := sm.State(c, protocol.NamespaceOption(_stakingNameSpace), protocol.KeyOption(_candidateTransferOwnship)); errors.Cause(err) != state.ErrStateNotExist {
		return err
	}
	return nil
}

// Store stores the candidate transfer ownership to state manager
func (c *CandidateTransferOwnership) StoreToStateManager(sm protocol.StateManager) error {
	_, err := sm.PutState(c, protocol.NamespaceOption(_stakingNameSpace), protocol.KeyOption(_candidateTransferOwnship))
	return err
}
