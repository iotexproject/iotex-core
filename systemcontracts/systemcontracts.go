// Package systemcontracts provides system contract management functionality
package systemcontracts

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

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
	// CandidateListV2Storage is the system contract for candidate list v2 storage
	CandidateListV2Storage int = iota
	// VoteBucketStorage is the system contract for vote bucket storage
	VoteBucketStorage
	// StakingBucketsContractIndex is the system contract for staking buckets
	StakingBucketsContractIndex
	// SystemContractCount is the total number of system contracts
	SystemContractCount
)

// SystemContracts holds all system contracts
var SystemContracts []SystemContract
var systemContractsInitialized bool

var systemContractCreatorAddr = hash.Hash160b([]byte("system_contract_creator"))

func init() {
	initSystemContracts()
}

// initSystemContracts initializes the system contracts array
func initSystemContracts() {
	// Initialize bytecodes first
	var err error
	CandidateListV2StorageByteCode, err = hex.DecodeString(CandidateListV2StorageByteCodeStr)
	if err != nil {
		log.S().Panic("failed to decode CandidateListV2StorageByteCode: " + err.Error())
	}

	VoteBucketStorageByteCode, err = hex.DecodeString(VoteBucketStorageByteCodeStr)
	if err != nil {
		log.S().Panic("failed to decode VoteBucketStorageByteCode: " + err.Error())
	}

	genericStorageByteCode, err := hex.DecodeString(GenericStorageByteCodeStr)
	if err != nil {
		log.S().Panic("failed to decode GenericStorageByteCode: " + err.Error())
	}

	candidateListV2Storage, err := address.FromBytes(crypto.CreateAddress(common.BytesToAddress(systemContractCreatorAddr[:]), 0).Bytes())
	if err != nil {
		log.S().Panic("Invalid candidate list v2 storage contract address: " + err.Error())
	}
	voteBucketStorage, err := address.FromBytes(crypto.CreateAddress(common.BytesToAddress(systemContractCreatorAddr[:]), 1).Bytes())
	if err != nil {
		log.S().Panic("Invalid vote bucket storage contract address: " + err.Error())
	}
	stakingBucketAddr, err := address.FromBytes(crypto.CreateAddress(common.BytesToAddress(systemContractCreatorAddr[:]), 2).Bytes())
	if err != nil {
		log.S().Panic("Invalid staking bucket contract address: " + err.Error())
	}

	SystemContracts = make([]SystemContract, SystemContractCount)
	SystemContracts[CandidateListV2Storage] = SystemContract{
		Address: candidateListV2Storage,
		Code:    CandidateListV2StorageByteCode,
	}
	SystemContracts[VoteBucketStorage] = SystemContract{
		Address: voteBucketStorage,
		Code:    VoteBucketStorageByteCode,
	}
	SystemContracts[StakingBucketsContractIndex] = SystemContract{
		Address: stakingBucketAddr,
		Code:    genericStorageByteCode,
	}
}

// ErrStateNotExist is the error that the state does not exist
var ErrStateNotExist = fmt.Errorf("state not found")

// isContractError checks if the error contains a specific contract error selector
func isContractError(err error, errorName string) bool {
	if err == nil {
		return false
	}
	errorStr := err.Error()
	// Check if error string contains revert with the specific error name
	// Solidity custom errors are typically reverted with the error name
	return strings.Contains(errorStr, "revert") && strings.Contains(errorStr, errorName)
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
