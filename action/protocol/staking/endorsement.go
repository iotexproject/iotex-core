package staking

import (
	"math"

	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/stakingpb"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/systemcontracts"
)

// EndorsementStatus
const (
	// EndorseExpired means the endorsement is expired
	EndorseExpired = EndorsementStatus(iota)
	// UnEndorsing means the endorser has submitted unendorsement, but it is not expired yet
	UnEndorsing
	// Endorsed means the endorsement is valid
	Endorsed
)

const (
	endorsementNotExpireHeight = math.MaxUint64
)

type (
	// EndorsementStatus is a uint8 that represents the status of the endorsement
	EndorsementStatus uint8

	// Endorsement is a struct that contains the expire height of the Endorsement
	Endorsement struct {
		// ExpireHeight is the height an endorsement is expired in legacy mode and it is the earliest height that can revoke the endorsement in new mode
		ExpireHeight uint64
	}
)

var _ protocol.ContractStorage = (*Endorsement)(nil)

// String returns a human-readable string of the endorsement status
func (s EndorsementStatus) String() string {
	switch s {
	case EndorseExpired:
		return "Expired"
	case UnEndorsing:
		return "UnEndorsing"
	case Endorsed:
		return "Endorsed"
	default:
		return "Unknown"
	}
}

func (e *Endorsement) LegacyStatus(height uint64) EndorsementStatus {
	if e.ExpireHeight == endorsementNotExpireHeight {
		return Endorsed
	}
	if height >= e.ExpireHeight {
		return EndorseExpired
	}
	return UnEndorsing
}

// Status returns the status of the endorsement
func (e *Endorsement) Status(height uint64) EndorsementStatus {
	if height < e.ExpireHeight {
		return Endorsed
	}
	return UnEndorsing
}

// Serialize serializes endorsement to bytes
func (e *Endorsement) Serialize() ([]byte, error) {
	pb, err := e.toProto()
	if err != nil {
		return nil, err
	}
	return proto.Marshal(pb)
}

// Deserialize deserializes bytes to endorsement
func (e *Endorsement) Deserialize(buf []byte) error {
	pb := &stakingpb.Endorsement{}
	if err := proto.Unmarshal(buf, pb); err != nil {
		return errors.Wrap(err, "failed to unmarshal endorsement")
	}
	return e.fromProto(pb)
}

func (e *Endorsement) storageContractAddress(ns string) (address.Address, error) {
	if ns != _stakingNameSpace {
		return nil, errors.Errorf("invalid namespace %s, expected %s", ns, _stakingNameSpace)
	}
	// Use the system contract address for endorsements
	return systemcontracts.SystemContracts[systemcontracts.EndorsementContractIndex].Address, nil
}

func (e *Endorsement) storageContract(ns string, key []byte, backend systemcontracts.ContractBackend) (*systemcontracts.GenericStorageContract, error) {
	addr, err := e.storageContractAddress(ns)
	if err != nil {
		return nil, err
	}
	contract, err := systemcontracts.NewGenericStorageContract(common.BytesToAddress(addr.Bytes()), backend)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create endorsement storage contract")
	}
	return contract, nil
}

func (e *Endorsement) StoreToContract(ns string, key []byte, backend systemcontracts.ContractBackend) error {
	contract, err := e.storageContract(ns, key, backend)
	if err != nil {
		return err
	}
	log.S().Debugf("Storing endorsement to contract %s with key %x value %+v", contract.Address().Hex(), key, e)
	data, err := e.Serialize()
	if err != nil {
		return errors.Wrap(err, "failed to serialize endorsement")
	}
	if err := contract.Put(key, systemcontracts.GenericValue{PrimaryData: data}); err != nil {
		return errors.Wrapf(err, "failed to put endorsement to contract")
	}
	return nil
}

func (e *Endorsement) LoadFromContract(ns string, key []byte, backend systemcontracts.ContractBackend) error {
	contract, err := e.storageContract(ns, key, backend)
	if err != nil {
		return err
	}
	storeResult, err := contract.Get(key)
	if err != nil {
		return errors.Wrapf(err, "failed to get endorsement from contract")
	}
	if !storeResult.KeyExists {
		return errors.Wrapf(state.ErrStateNotExist, "endorsement does not exist in contract")
	}
	defer func() {
		log.S().Debugf("Loaded endorsement from contract %s with key %x value %+v", contract.Address().Hex(), key, e)
	}()
	return e.Deserialize(storeResult.Value.PrimaryData)
}

func (e *Endorsement) DeleteFromContract(ns string, key []byte, backend systemcontracts.ContractBackend) error {
	contract, err := e.storageContract(ns, key, backend)
	if err != nil {
		return err
	}
	log.S().Debugf("Deleting endorsement from contract %s with key %x", contract.Address().Hex(), key)
	return contract.Remove(key)
}

func (e *Endorsement) ListFromContract(ns string, backend systemcontracts.ContractBackend) ([][]byte, []any, error) {
	contract, err := e.storageContract(ns, nil, backend)
	if err != nil {
		return nil, nil, err
	}
	count, err := contract.Count()
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to count endorsements in contract")
	}
	if count.Sign() == 0 {
		log.S().Debugf("No endorsements found in contract %s", contract.Address().Hex())
		return nil, nil, nil
	}
	listResult, err := contract.List(0, count.Uint64())
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to list endorsements from contract")
	}
	values := make([]any, 0, len(listResult.Values))
	for _, v := range listResult.Values {
		e := &Endorsement{}
		if err := e.Deserialize(v.PrimaryData); err != nil {
			return nil, nil, errors.Wrapf(err, "failed to deserialize endorsement from contract")
		}
		values = append(values, e)
	}
	log.S().Debugf("Listed %d endorsements from contract %s", len(values), contract.Address().Hex())
	return listResult.KeyList, values, nil
}

func (e *Endorsement) BatchFromContract(ns string, keys [][]byte, backend systemcontracts.ContractBackend) ([]any, error) {
	return nil, errors.New("not implemented")
}

func (e *Endorsement) toProto() (*stakingpb.Endorsement, error) {
	return &stakingpb.Endorsement{
		ExpireHeight: e.ExpireHeight,
	}, nil
}

func (e *Endorsement) fromProto(pb *stakingpb.Endorsement) error {
	e.ExpireHeight = pb.ExpireHeight
	return nil
}
