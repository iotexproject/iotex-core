package erigonstore

import (
	"bytes"
	"reflect"
	"strings"

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
	contracts              map[string]map[reflect.Type]int
	ns                     map[string]int
	nsPrefix               map[string]int
	spliter                map[int]KeySplitter
	kvList                 map[int]struct{}
	rewardingHistoryPrefix map[int]rewardingHistoryConfig
}

type rewardingHistoryConfig struct {
	Prefixs  [][]byte
	KeySplit KeySplitter
}

type RegisterOption func(int, *ObjectStorageRegistry)

func WithKeySplitOption(split KeySplitter) RegisterOption {
	return func(index int, osr *ObjectStorageRegistry) {
		osr.spliter[index] = split
	}
}

func WithKVListOption() RegisterOption {
	return func(index int, osr *ObjectStorageRegistry) {
		osr.kvList[index] = struct{}{}
	}
}

func WithRewardingHistoryPrefixOption(split KeySplitter, prefix ...[]byte) RegisterOption {
	return func(index int, osr *ObjectStorageRegistry) {
		osr.rewardingHistoryPrefix[index] = rewardingHistoryConfig{
			Prefixs:  prefix,
			KeySplit: split,
		}
	}
}

func init() {
	rewardHistoryPrefixs := [][]byte{
		append(state.RewardingKeyPrefix[:], state.BlockRewardHistoryKeyPrefix...),
		append(state.RewardingKeyPrefix[:], state.EpochRewardHistoryKeyPrefix...),
	}
	// pollPrefix := [][]byte{
	// 	[]byte(state.PollCandidatesPrefix),
	// }
	genKeySplit := func(prefixs [][]byte) KeySplitter {
		return func(key []byte) (part1 []byte, part2 []byte) {
			for _, p := range prefixs {
				if len(key) >= len(p)+8 && bytes.Equal(key[:len(p)], p) {
					// split into prefix + last 8 bytes
					return key[:len(key)-8], key[len(key)-8:]
				}
			}
			return key, nil
		}
	}
	epochRewardKeySplit := genKeySplit(rewardHistoryPrefixs[1:])
	blockRewardKeySplit := genKeySplit(rewardHistoryPrefixs[:1])
	// pollKeySplit := genKeySplit(pollPrefix)

	assertions.MustNoError(storageRegistry.RegisterNamespace(state.AccountKVNamespace, RewardingContractV1Index, WithKeySplitOption(epochRewardKeySplit), WithRewardingHistoryPrefixOption(blockRewardKeySplit, rewardHistoryPrefixs[0])))
	assertions.MustNoError(storageRegistry.RegisterNamespace(state.RewardingNamespace, RewardingContractV2Index, WithKeySplitOption(epochRewardKeySplit), WithRewardingHistoryPrefixOption(blockRewardKeySplit, rewardHistoryPrefixs[0])))
	assertions.MustNoError(storageRegistry.RegisterNamespace(state.CandidateNamespace, CandidatesContractIndex))
	assertions.MustNoError(storageRegistry.RegisterNamespace(state.CandsMapNamespace, CandidateMapContractIndex, WithKVListOption()))
	assertions.MustNoError(storageRegistry.RegisterNamespace(state.StakingNamespace, BucketPoolContractIndex))
	assertions.MustNoError(storageRegistry.RegisterNamespace(state.StakingViewNamespace, StakingViewContractIndex, WithKVListOption()))
	assertions.MustNoError(storageRegistry.RegisterNamespace(state.StakingContractMetaNamespace, ContractStakingBucketContractIndex))
	assertions.MustNoError(storageRegistry.RegisterNamespacePrefix(state.ContractStakingBucketNamespacePrefix, ContractStakingBucketContractIndex))
	assertions.MustNoError(storageRegistry.RegisterNamespacePrefix(state.ContractStakingBucketTypeNamespacePrefix, ContractStakingBucketContractIndex))

	assertions.MustNoError(storageRegistry.RegisterObjectStorage(state.AccountKVNamespace, &state.Account{}, AccountIndex))
	assertions.MustNoError(storageRegistry.RegisterObjectStorage(state.AccountKVNamespace, &state.CandidateList{}, PollLegacyCandidateListContractIndex))
	assertions.MustNoError(storageRegistry.RegisterObjectStorage(state.SystemNamespace, &state.CandidateList{}, PollCandidateListContractIndex, WithKVListOption()))
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
		contracts:              make(map[string]map[reflect.Type]int),
		ns:                     make(map[string]int),
		nsPrefix:               make(map[string]int),
		spliter:                make(map[int]KeySplitter),
		kvList:                 make(map[int]struct{}),
		rewardingHistoryPrefix: make(map[int]rewardingHistoryConfig),
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
		var os ObjectStorage
		os = newContractObjectStorage(contract)
		split := osr.spliter[contractIndex]
		_, kvList := osr.kvList[contractIndex]
		if kvList {
			os = newKVListStorage(contract)
		}
		if split != nil {
			prefixFallbacks := map[string]ObjectStorage{}
			if config, ok := osr.rewardingHistoryPrefix[contractIndex]; ok {
				for _, prefix := range config.Prefixs {
					prefixFallbacks[string(prefix)] = newRewardHistoryStorage(contract, backend.height, config.KeySplit)
				}
			}
			os = newKeySplitContractStorageWithfallback(contract, split, os, prefixFallbacks)
		}
		return os, nil
	default:
		contractAddr := systemContracts[contractIndex].Address
		contract, err := systemcontracts.NewGenericStorageContract(common.BytesToAddress(contractAddr.Bytes()[:]), backend, common.Address(systemContractCreatorAddr))
		if err != nil {
			return nil, err
		}
		var os ObjectStorage
		split := osr.spliter[contractIndex]
		_, kvList := osr.kvList[contractIndex]
		os = newContractObjectStorage(contract)
		if kvList {
			os = newKVListStorage(contract)
		}
		if split != nil {
			prefixFallbacks := map[string]ObjectStorage{}
			if config, ok := osr.rewardingHistoryPrefix[contractIndex]; ok {
				for _, prefix := range config.Prefixs {
					prefixFallbacks[string(prefix)] = newRewardHistoryStorage(contract, backend.height, config.KeySplit)
				}
			}
			os = newKeySplitContractStorageWithfallback(contract, split, os, prefixFallbacks)
		}
		return os, nil
	}
}

// RegisterObjectStorage registers a generic object storage
func (osr *ObjectStorageRegistry) RegisterObjectStorage(ns string, obj any, index int, opts ...RegisterOption) error {
	if index < AccountIndex || index >= SystemContractCount {
		return errors.Errorf("invalid system contract index %d", index)
	}
	if err := osr.register(ns, obj, index); err != nil {
		return err
	}
	for _, opt := range opts {
		opt(index, osr)
	}
	return nil
}

// RegisterNamespace registers a namespace object storage
func (osr *ObjectStorageRegistry) RegisterNamespace(ns string, index int, opts ...RegisterOption) error {
	if index < AccountIndex || index >= SystemContractCount {
		return errors.Errorf("invalid system contract index %d", index)
	}
	if err := osr.register(ns, nil, index); err != nil {
		return err
	}
	for _, opt := range opts {
		opt(index, osr)
	}
	return nil
}

// RegisterNamespacePrefix registers a namespace prefix object storage
func (osr *ObjectStorageRegistry) RegisterNamespacePrefix(prefix string, index int) error {
	if index < AccountIndex || index >= SystemContractCount {
		return errors.Errorf("invalid system contract index %d", index)
	}
	return osr.registerPrefix(prefix, index)
}

func (osr *ObjectStorageRegistry) registerPrefix(ns string, index int) error {
	if _, exists := osr.nsPrefix[ns]; exists {
		return errors.Wrapf(ErrObjectStorageAlreadyRegistered, "registered: %v", osr.nsPrefix[ns])
	}
	osr.nsPrefix[ns] = index
	return nil
}

func (osr *ObjectStorageRegistry) register(ns string, obj any, index int) error {
	if obj == nil {
		if _, exists := osr.ns[ns]; exists {
			return errors.Wrapf(ErrObjectStorageAlreadyRegistered, "registered: %v", osr.ns[ns])
		}
		osr.ns[ns] = index
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
	// object specific storage
	if obj != nil {
		types, ok := osr.contracts[ns]
		if ok {
			index, exist := types[reflect.TypeOf(obj)]
			if exist {
				return index, true
			}
		}
	}
	// namespace specific storage
	index, exist := osr.ns[ns]
	if exist {
		return index, true
	}
	// namespace prefix specific storage
	for prefix, index := range osr.nsPrefix {
		if strings.HasPrefix(ns, prefix) {
			return index, true
		}
	}
	return 0, false
}
