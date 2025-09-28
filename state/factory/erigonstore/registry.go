package erigonstore

import (
	"reflect"

	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action/protocol/poll"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/action/protocol/vote"
	"github.com/iotexproject/iotex-core/v2/pkg/util/assertions"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/systemcontracts"
)

var (
	// ErrObjectStorageNotRegistered is returned when an object storage is not registered
	ErrObjectStorageNotRegistered = errors.New("object storage not registered")
	// ErrObjectStorageAlreadyRegistered is returned when an object storage is already registered
	ErrObjectStorageAlreadyRegistered = errors.New("object storage already registered")
)

var (
	storageRegistry = newObjectStorageRegistry()
)

// ObjectStorageRegistry is a registry for object storage
type ObjectStorageRegistry struct {
	contracts map[string]map[reflect.Type]int
	fallback  map[string]int
}

func init() {
	assertions.MustNoError(storageRegistry.RegisterNamespace(state.AccountKVNamespace, RewardingContractV1Index))
	assertions.MustNoError(storageRegistry.RegisterNamespace(state.RewardingNamespace, RewardingContractV2Index))
	assertions.MustNoError(storageRegistry.RegisterNamespace(state.CandidateNamespace, CandidatesContractIndex))
	assertions.MustNoError(storageRegistry.RegisterNamespace(state.CandsMapNamespace, CandidateMapContractIndex))
	assertions.MustNoError(storageRegistry.RegisterNamespace(state.StakingNamespace, BucketPoolContractIndex))
	assertions.MustNoError(storageRegistry.RegisterNamespace(state.StakingViewNamespace, StakingViewContractIndex))
	assertions.MustNoError(storageRegistry.RegisterNamespace(state.StakingNamespace, BucketPoolContractIndex))

	assertions.MustNoError(storageRegistry.RegisterObjectStorage(state.AccountKVNamespace, &state.Account{}, AccountIndex))
	assertions.MustNoError(storageRegistry.RegisterObjectStorage(state.AccountKVNamespace, &state.CandidateList{}, PollLegacyCandidateListContractIndex))
	assertions.MustNoError(storageRegistry.RegisterObjectStorage(state.SystemNamespace, &state.CandidateList{}, PollCandidateListContractIndex))
	assertions.MustNoError(storageRegistry.RegisterObjectStorage(state.SystemNamespace, &vote.UnproductiveDelegate{}, PollUnproductiveDelegateContractIndex))
	assertions.MustNoError(storageRegistry.RegisterObjectStorage(state.SystemNamespace, &vote.ProbationList{}, PollProbationListContractIndex))
	assertions.MustNoError(storageRegistry.RegisterObjectStorage(state.SystemNamespace, &poll.BlockMeta{}, PollBlockMetaContractIndex))
	assertions.MustNoError(storageRegistry.RegisterObjectStorage(state.StakingNamespace, &staking.VoteBucket{}, StakingBucketsContractIndex))
	assertions.MustNoError(storageRegistry.RegisterObjectStorage(state.StakingNamespace, &staking.Endorsement{}, EndorsementContractIndex))
	assertions.MustNoError(storageRegistry.RegisterObjectStorage(state.StakingNamespace, &staking.BucketIndices{}, BucketIndicesContractIndex))
}

// GetObjectStorageRegistry returns the global object storage registry
func GetObjectStorageRegistry() *ObjectStorageRegistry {
	return storageRegistry
}

func newObjectStorageRegistry() *ObjectStorageRegistry {
	return &ObjectStorageRegistry{
		contracts: make(map[string]map[reflect.Type]int),
		fallback:  make(map[string]int),
	}
}

// ObjectStorage returns the object storage for the given namespace and object type
func (osr *ObjectStorageRegistry) ObjectStorage(ns string, obj any, backend *contractBackend) (ObjectStorage, error) {
	contractIndex, exist := osr.matchContractIndex(ns, obj)
	if !exist {
		return nil, errors.Wrapf(ErrObjectStorageNotRegistered, "namespace: %s, type: %T", ns, obj)
	}
	// TODO: cache storage
	switch systemContractTypes[contractIndex] {
	case accountStorageType:
		return newAccountStorage(
			common.BytesToAddress(systemContracts[AccountInfoContractIndex].Address.Bytes()),
			backend,
		)
	case namespaceStorageContractType:
		contractAddr := systemContracts[contractIndex].Address
		contract, err := systemcontracts.NewNamespaceStorageContractWrapper(common.BytesToAddress(contractAddr.Bytes()[:]), backend, common.Address(systemContractCreatorAddr), ns)
		if err != nil {
			return nil, err
		}
		return newContractObjectStorage(contract), nil
	default:
		contractAddr := systemContracts[contractIndex].Address
		contract, err := systemcontracts.NewGenericStorageContract(common.BytesToAddress(contractAddr.Bytes()[:]), backend, common.Address(systemContractCreatorAddr))
		if err != nil {
			return nil, err
		}
		return newContractObjectStorage(contract), nil
	}
}

// RegisterObjectStorage registers a generic object storage
func (osr *ObjectStorageRegistry) RegisterObjectStorage(ns string, obj any, index int) error {
	if index < AccountIndex || index >= SystemContractCount {
		return errors.Errorf("invalid system contract index %d", index)
	}
	return osr.register(ns, obj, index)
}

// RegisterNamespace registers a namespace object storage
func (osr *ObjectStorageRegistry) RegisterNamespace(ns string, index int) error {
	if index < AccountIndex || index >= SystemContractCount {
		return errors.Errorf("invalid system contract index %d", index)
	}
	return osr.register(ns, nil, index)
}

func (osr *ObjectStorageRegistry) register(ns string, obj any, index int) error {
	if obj == nil {
		if _, exists := osr.fallback[ns]; exists {
			return errors.Wrapf(ErrObjectStorageAlreadyRegistered, "registered: %v", osr.fallback[ns])
		}
		osr.fallback[ns] = index
		return nil
	}
	types, ok := osr.contracts[ns]
	if !ok {
		osr.contracts[ns] = make(map[reflect.Type]int)
		types = osr.contracts[ns]
	}
	if registered, exists := types[reflect.TypeOf(obj)]; exists {
		return errors.Wrapf(ErrObjectStorageAlreadyRegistered, "registered: %v", registered)
	}
	types[reflect.TypeOf(obj)] = index
	return nil
}

func (osr *ObjectStorageRegistry) matchContractIndex(ns string, obj any) (int, bool) {
	if obj != nil {
		types, ok := osr.contracts[ns]
		if ok {
			index, exist := types[reflect.TypeOf(obj)]
			if exist {
				return index, true
			}
		}
	}
	index, exist := osr.fallback[ns]
	return index, exist
}
