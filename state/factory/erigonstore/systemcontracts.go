package erigonstore

import (
	"encoding/hex"
	"log"

	"github.com/erigontech/erigon-lib/common"
	"github.com/erigontech/erigon-lib/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/systemcontracts"
)

// SystemContract represents a system contract with its address and bytecode
type SystemContract struct {
	Address address.Address
	Code    []byte
}

const (
	// AccountIndex is the system contract for account storage
	AccountIndex = -1
)

const (
	// StakingBucketsContractIndex is the system contract for staking buckets storage
	StakingBucketsContractIndex int = iota
	// BucketPoolContractIndex is the system contract for bucket pool storage
	BucketPoolContractIndex
	// BucketIndicesContractIndex is the system contract for bucket indices storage
	BucketIndicesContractIndex
	// EndorsementContractIndex is the system contract for endorsement storage
	EndorsementContractIndex
	// CandidateMapContractIndex is the system contract for candidate map storage
	CandidateMapContractIndex
	// CandidatesContractIndex is the system contract for candidates storage
	CandidatesContractIndex
	// PollCandidateListContractIndex is the system contract for poll candidate storage
	PollCandidateListContractIndex
	// PollLegacyCandidateListContractIndex is the system contract for poll legacy candidate storage
	PollLegacyCandidateListContractIndex
	// PollProbationListContractIndex is the system contract for poll probation list storage
	PollProbationListContractIndex
	// PollUnproductiveDelegateContractIndex is the system contract for poll unproductive delegate storage
	PollUnproductiveDelegateContractIndex
	// PollBlockMetaContractIndex is the system contract for poll block meta storage
	PollBlockMetaContractIndex
	// RewardingContractV1Index is the system contract for rewarding admin storage
	RewardingContractV1Index
	// RewardingContractV2Index is the system contract for rewarding admin storage v2
	RewardingContractV2Index
	// StakingViewContractIndex is the system contract for staking view storage
	StakingViewContractIndex
	// AccountInfoContractIndex is the system contract for account info storage
	AccountInfoContractIndex
	// SystemContractCount is the total number of system contracts
	SystemContractCount
)

const (
	defaultSystemContractType = iota
	namespaceStorageContractType
	accountStorageType
)

var systemContractTypes = map[int]int{
	StakingViewContractIndex: namespaceStorageContractType,
	AccountIndex:             accountStorageType,
}

// systemContracts holds all system contracts
var systemContracts []SystemContract

// systemContractCreatorAddr is the address used to create system contracts
// TODO: review and delete it
var systemContractCreatorAddr = hash.Hash160b([]byte("system_contract_creator"))

func init() {
	genericStorageByteCode, err := hex.DecodeString(systemcontracts.GenericStorageByteCodeStr)
	if err != nil {
		log.Panic(errors.Wrap(err, "failed to decode GenericStorageByteCode"))
	}
	namespaceStorageByteCode, err := hex.DecodeString(systemcontracts.NamespaceStorageContractByteCodeStr)
	if err != nil {
		log.Panic(errors.Wrap(err, "failed to decode NamespaceStorageContractByteCode"))
	}

	systemContracts = make([]SystemContract, SystemContractCount)
	for i := 0; i < SystemContractCount; i++ {
		addr, err := address.FromBytes(crypto.CreateAddress(common.BytesToAddress(systemContractCreatorAddr[:]), uint64(i)).Bytes())
		if err != nil {
			log.Panic(errors.Wrap(err, "invalid system contract address"))
		}
		var byteCode []byte
		switch systemContractTypes[i] {
		case namespaceStorageContractType:
			byteCode = namespaceStorageByteCode
		default:
			byteCode = genericStorageByteCode
		}
		systemContracts[i] = SystemContract{
			Address: addr,
			Code:    byteCode,
		}
	}
}
