package erigonstore

import (
	"reflect"

	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"

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
}

func init() {
	assertions.MustNoError(storageRegistry.RegisterAccount(state.AccountKVNamespace, &state.Account{}))
	assertions.MustNoError(storageRegistry.RegisterPollCandidateList(state.SystemNamespace, &state.CandidateList{}))
	assertions.MustNoError(storageRegistry.RegisterPollLegacyCandidateList(state.AccountKVNamespace, &state.CandidateList{}))
}

// GetObjectStorageRegistry returns the global object storage registry
func GetObjectStorageRegistry() *ObjectStorageRegistry {
	return storageRegistry
}

func newObjectStorageRegistry() *ObjectStorageRegistry {
	return &ObjectStorageRegistry{
		contracts: make(map[string]map[reflect.Type]int),
	}
}

// ObjectStorage returns the object storage for the given namespace and object type
func (osr *ObjectStorageRegistry) ObjectStorage(ns string, obj any, backend *contractBackend) (ObjectStorage, error) {
	types, ok := osr.contracts[ns]
	if !ok {
		return nil, errors.Wrapf(ErrObjectStorageNotRegistered, "namespace: %s", ns)
	}
	contractIndex, ok := types[reflect.TypeOf(obj)]
	if !ok {
		return nil, errors.Wrapf(ErrObjectStorageNotRegistered, "namespace: %s, object: %T", ns, obj)
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

// RegisterAccount registers an account object storage
func (osr *ObjectStorageRegistry) RegisterAccount(ns string, obj any) error {
	return osr.register(ns, obj, AccountIndex)
}

// RegisterStakingBuckets registers a staking buckets object storage
func (osr *ObjectStorageRegistry) RegisterStakingBuckets(ns string, obj systemcontracts.GenericValueContainer) error {
	return osr.register(ns, obj, StakingBucketsContractIndex)
}

// RegisterBucketPool registers a bucket pool object storage
func (osr *ObjectStorageRegistry) RegisterBucketPool(ns string, obj systemcontracts.GenericValueContainer) error {
	return osr.register(ns, obj, BucketPoolContractIndex)
}

// RegisterBucketIndices registers a bucket indices object storage
func (osr *ObjectStorageRegistry) RegisterBucketIndices(ns string, obj systemcontracts.GenericValueContainer) error {
	return osr.register(ns, obj, BucketIndicesContractIndex)
}

// RegisterEndorsement registers an endorsement object storage
func (osr *ObjectStorageRegistry) RegisterEndorsement(ns string, obj systemcontracts.GenericValueContainer) error {
	return osr.register(ns, obj, EndorsementContractIndex)
}

// RegisterCandidateMap registers a candidate map object storage
func (osr *ObjectStorageRegistry) RegisterCandidateMap(ns string, obj systemcontracts.GenericValueContainer) error {
	return osr.register(ns, obj, CandidateMapContractIndex)
}

// RegisterCandidates registers a candidates object storage
func (osr *ObjectStorageRegistry) RegisterCandidates(ns string, obj systemcontracts.GenericValueContainer) error {
	return osr.register(ns, obj, CandidatesContractIndex)
}

// RegisterPollCandidateList registers a poll candidate list object storage
func (osr *ObjectStorageRegistry) RegisterPollCandidateList(ns string, obj systemcontracts.GenericValueContainer) error {
	return osr.register(ns, obj, PollCandidateListContractIndex)
}

// RegisterPollLegacyCandidateList registers a poll legacy candidate list object storage
func (osr *ObjectStorageRegistry) RegisterPollLegacyCandidateList(ns string, obj systemcontracts.GenericValueContainer) error {
	return osr.register(ns, obj, PollLegacyCandidateListContractIndex)
}

// RegisterPollProbationList registers a poll probation list object storage
func (osr *ObjectStorageRegistry) RegisterPollProbationList(ns string, obj systemcontracts.GenericValueContainer) error {
	return osr.register(ns, obj, PollProbationListContractIndex)
}

// RegisterPollUnproductiveDelegate registers a poll unproductive delegate object storage
func (osr *ObjectStorageRegistry) RegisterPollUnproductiveDelegate(ns string, obj systemcontracts.GenericValueContainer) error {
	return osr.register(ns, obj, PollUnproductiveDelegateContractIndex)
}

// RegisterPollBlockMeta registers a poll block meta object storage
func (osr *ObjectStorageRegistry) RegisterPollBlockMeta(ns string, obj systemcontracts.GenericValueContainer) error {
	return osr.register(ns, obj, PollBlockMetaContractIndex)
}

// RegisterRewardingV1 registers a rewarding v1 object storage
func (osr *ObjectStorageRegistry) RegisterRewardingV1(ns string, obj systemcontracts.GenericValueContainer) error {
	return osr.register(ns, obj, RewardingContractV1Index)
}

// RegisterRewardingV2 registers a rewarding v2 object storage
func (osr *ObjectStorageRegistry) RegisterRewardingV2(ns string, obj systemcontracts.GenericValueContainer) error {
	return osr.register(ns, obj, RewardingContractV2Index)
}

// RegisterStakingView registers a staking view object storage
func (osr *ObjectStorageRegistry) RegisterStakingView(ns string, obj systemcontracts.GenericValueContainer) error {
	return osr.register(ns, obj, StakingViewContractIndex)
}

func (osr *ObjectStorageRegistry) register(ns string, obj any, index int) error {
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
