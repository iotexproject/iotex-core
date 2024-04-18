package staking

import (
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/staking/stakingpb"
	"github.com/iotexproject/iotex-core/state"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"
)

const (
	_candidateTransferOwnershipProtoVersion float32 = 1.0
)

var (
	_candidateTransferOwnship = []byte("candidateTransferOwnership")
)

type CandidateTransferOwnership struct {
	NameToOwner map[string]address.Address
}

func newCandidateTransferOwnership() *CandidateTransferOwnership {
	return &CandidateTransferOwnership{
		NameToOwner: make(map[string]address.Address),
	}
}

// Update updates the candidate transfer ownership
func (c *CandidateTransferOwnership) Update(name string, newOwner address.Address) {
	c.NameToOwner[name] = newOwner
}

// Serialize serializes CandidateTransferOwnership to bytes
func (c *CandidateTransferOwnership) Serialize() ([]byte, error) {
	pb := &stakingpb.CandidateTransferOwnership{
		Version:     _candidateTransferOwnershipProtoVersion,
		NameToOwner: make(map[string][]byte),
	}
	for k, v := range c.NameToOwner {
		pb.NameToOwner[k] = v.Bytes()
	}
	return proto.Marshal(pb)
}

// Deserialize deserializes bytes to CandidateTransferOwnership
func (c *CandidateTransferOwnership) Deserialize(data []byte) error {
	pb := &stakingpb.CandidateTransferOwnership{}
	if err := proto.Unmarshal(data, pb); err != nil {
		return err
	}
	c.NameToOwner = make(map[string]address.Address, len(pb.NameToOwner))
	for k, v := range pb.NameToOwner {
		addr, err := address.FromBytes(v)
		if err != nil {
			return err
		}
		c.NameToOwner[k] = addr
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
