// Package systemcontracts provides system contract management functionality
package systemcontracts

import (
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/pkg/log"
)

// SystemContract represents a system contract with its address and bytecode
type SystemContract struct {
	Address address.Address
	Code    []byte
}

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
)

var systemContractTypes = map[int]int{
	StakingViewContractIndex: namespaceStorageContractType,
}

// SystemContracts holds all system contracts
var SystemContracts []SystemContract

// systemContractCreatorAddr is the address used to create system contracts
var systemContractCreatorAddr = hash.Hash160b([]byte("system_contract_creator"))

func init() {
	initSystemContracts()
}

// initSystemContracts initializes the system contracts array
func initSystemContracts() {
	genericStorageByteCode, err := hex.DecodeString(GenericStorageByteCodeStr)
	if err != nil {
		log.S().Panic("failed to decode GenericStorageByteCode: " + err.Error())
	}
	namespaceStorageByteCode, err := hex.DecodeString(NamespaceStorageContractByteCodeStr)
	if err != nil {
		log.S().Panic("failed to decode NamespaceStorageContractByteCode: " + err.Error())
	}

	SystemContracts = make([]SystemContract, SystemContractCount)
	for i := 0; i < SystemContractCount; i++ {
		addr, err := address.FromBytes(crypto.CreateAddress(common.BytesToAddress(systemContractCreatorAddr[:]), uint64(i)).Bytes())
		if err != nil {
			log.S().Panic("Invalid system contract address: " + err.Error())
		}
		var byteCode []byte
		switch systemContractTypes[i] {
		case namespaceStorageContractType:
			byteCode = namespaceStorageByteCode
		default:
			byteCode = genericStorageByteCode
		}
		SystemContracts[i] = SystemContract{
			Address: addr,
			Code:    byteCode,
		}
	}
}

// SystemContractCreatorAddr returns the address used to create system contracts
func SystemContractCreatorAddr() address.Address {
	addr, err := address.FromBytes(systemContractCreatorAddr[:])
	if err != nil {
		log.S().Panic("Invalid system contract creator address: " + err.Error())
	}
	return addr
}

// ContractBackend defines the interface for contract backend operations
type ContractBackend interface {
	Call(callMsg *ethereum.CallMsg) ([]byte, error)
	Handle(callMsg *ethereum.CallMsg) error
}

// ContractDeployer defines the interface for contract deployment operations
type ContractDeployer interface {
	Deploy(callMsg *ethereum.CallMsg) (address.Address, error)
	Exists(addr address.Address) bool
}

// DeploySystemContractsIfNotExist deploys system contracts if they don't exist
func DeploySystemContractsIfNotExist(deployer ContractDeployer) error {
	for idx, contract := range SystemContracts {
		exists := deployer.Exists(contract.Address)
		if !exists {
			log.S().Infof("Deploying system contract [%d] %s", idx, contract.Address.String())
			msg := &ethereum.CallMsg{
				From:  common.BytesToAddress(systemContractCreatorAddr[:]),
				Data:  contract.Code,
				Value: big.NewInt(0),
				Gas:   10000000,
			}
			if addr, err := deployer.Deploy(msg); err != nil {
				return fmt.Errorf("failed to deploy system contract %s: %w", contract.Address.String(), err)
			} else if addr.String() != contract.Address.String() {
				return fmt.Errorf("deployed contract address %s does not match expected address %s", addr.String(), contract.Address.String())
			}
			log.S().Infof("System contract [%d] %s deployed successfully", idx, contract.Address.String())
		} else {
			log.S().Infof("System contract [%d] %s already exists", idx, contract.Address.String())
		}
	}
	return nil
}
