// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"math/big"

	"github.com/CoderZhi/go-ethereum/common"
	"github.com/CoderZhi/go-ethereum/core/types"
	"github.com/CoderZhi/go-ethereum/core/vm"
	"github.com/CoderZhi/go-ethereum/params"
	"github.com/iotexproject/iotex-core/blockchain/action"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/pkg/errors"
)

var (
	// ErrHitGasLimit is the error when hit gas limit
	ErrHitGasLimit = errors.New("Hit Gas Limit")
	// ErrInsufficientBalanceForGas is the error that the balance in executor account is lower than gas
	ErrInsufficientBalanceForGas = errors.New("Insufficient balance for gas")
	// ErrInconsistentNonce is the error that the nonce is different from executor's nonce
	ErrInconsistentNonce = errors.New("Nonce is not identical to executor nonce")
	// ErrOutOfGas is the error when running out of gas
	ErrOutOfGas = errors.New("Out of gas")
)

var (
	// FailureStatus is the status that contract execution failed
	FailureStatus = uint64(0)
	// SuccessStatus is the status that contract execution success
	SuccessStatus = uint64(1)
	// GasLimit is the total gas limit to be consumed in a block
	GasLimit = uint64(100000)
)

// Receipt represents the result of a contract
type Receipt struct {
	Status          uint64
	Hash            hash.Hash32B
	GasConsumed     uint64
	ContractAddress string
	Logs            []*types.Log
}

// CanTransfer checks whether the from account has enough balance
func CanTransfer(db vm.StateDB, fromHash common.Address, balance *big.Int) bool {
	return db.GetBalance(fromHash).Cmp(balance) > 0
}

// MakeTransfer transfers account
func MakeTransfer(db vm.StateDB, fromHash, toHash common.Address, amount *big.Int) {
	db.SubBalance(fromHash, amount)
	db.AddBalance(toHash, amount)
}

// EVMParams is the context and parameters
type EVMParams struct {
	context            vm.Context
	nonce              uint64
	executorRawAddress string
	amount             *big.Int
	contract           *common.Address
	gas                uint64
	data               []byte
}

// NewEVMParams creates a new context for use in the EVM.
func NewEVMParams(blk *Block, execution *action.Execution, stateDB *EVMStateDBAdapter) (*EVMParams, error) {
	// If we don't have an explicit author (i.e. not mining), extract from the header
	/*
		var beneficiary common.Address
		if author == nil {
			beneficiary, _ = chain.Engine().Author(header) // Ignore error, we're past header validation
		} else {
			beneficiary = *author
		}
	*/
	executorHash, err := iotxaddress.GetPubkeyHash(execution.Executor)
	if err != nil {
		return nil, err
	}
	executorAddr := common.BytesToAddress(executorHash)
	executorIoTXAddress, err := iotxaddress.GetAddressByHash(iotxaddress.IsTestnet, iotxaddress.ChainID, executorHash)
	if err != nil {
		return nil, err
	}
	var contractAddrPointer *common.Address
	if execution.Contract != action.EmptyAddress {
		contractHash, err := iotxaddress.GetPubkeyHash(execution.Contract)
		if err != nil {
			return nil, err
		}
		contractAddr := common.BytesToAddress(contractHash)
		contractAddrPointer = &contractAddr
	}
	producerHash := keypair.HashPubKey(blk.Header.Pubkey)
	producer := common.BytesToAddress(producerHash)
	context := vm.Context{
		CanTransfer: CanTransfer,
		Transfer:    MakeTransfer,
		GetHash:     GetHashFn(stateDB),
		Origin:      executorAddr,
		Coinbase:    producer,
		BlockNumber: new(big.Int).SetUint64(blk.Height()),
		Time:        new(big.Int).SetInt64(blk.Header.Timestamp().Unix()),
		Difficulty:  new(big.Int).SetUint64(uint64(50)),
		GasLimit:    GasLimit,
		GasPrice:    new(big.Int).SetUint64(uint64(execution.GasPrice)),
	}

	return &EVMParams{
		context,
		execution.Nonce,
		executorIoTXAddress.RawAddress,
		execution.Amount,
		contractAddrPointer,
		uint64(execution.Gas),
		execution.Data,
	}, nil
}

// GetHashFn returns a GetHashFunc which retrieves hashes by number
func GetHashFn(stateDB *EVMStateDBAdapter) func(n uint64) common.Hash {
	return func(n uint64) common.Hash {
		tipHeight, err := stateDB.bc.TipHeight()
		if err != nil {
			hash, err := stateDB.bc.GetHashByHeight(tipHeight - n)
			if err != nil {
				return common.BytesToHash(hash[:])
			}
		}

		return common.Hash{}
	}
}

func securityDeposit(context *EVMParams, stateDB vm.StateDB, gasLimit *uint64) error {
	executorNonce := stateDB.GetNonce(context.context.Origin)
	if executorNonce != context.nonce {
		//return ErrInconsistentNonce
	}
	if *gasLimit < context.gas {
		return ErrHitGasLimit
	}
	maxGasValue := new(big.Int).Mul(new(big.Int).SetUint64(context.gas), context.context.GasPrice)
	if stateDB.GetBalance(context.context.Origin).Cmp(maxGasValue) < 0 {
		return ErrInsufficientBalanceForGas
	}
	*gasLimit -= context.gas
	stateDB.SubBalance(context.context.Origin, maxGasValue)
	return nil
}

// ExecuteContracts process the contracts in a block
func ExecuteContracts(blk *Block, bc Blockchain) {
	gasLimit := GasLimit
	for _, execution := range blk.Executions {
		// TODO (zhi) log receipt to stateDB
		ExecuteContract(blk, execution, bc, &gasLimit)
	}
}

// ExecuteContract processes a transfer which contains a contract
func ExecuteContract(blk *Block, execution *action.Execution, bc Blockchain, gasLimit *uint64) (*Receipt, error) {
	stateDB := NewEVMStateDBAdapter(bc)
	ps, err := NewEVMParams(blk, execution, stateDB)
	if err != nil {
		return nil, err
	}
	depositGas, remainingGas, contractAddress, err := executeInEVM(ps, stateDB, gasLimit)
	receipt := &Receipt{
		GasConsumed:     ps.gas - remainingGas,
		Hash:            execution.Hash(),
		ContractAddress: contractAddress,
	}
	if err != nil {
		receipt.Status = FailureStatus
	} else {
		receipt.Status = SuccessStatus
	}
	if remainingGas > 0 {
		*gasLimit += remainingGas
		remainingValue := new(big.Int).Mul(new(big.Int).SetUint64(remainingGas), ps.context.GasPrice)
		stateDB.AddBalance(ps.context.Origin, remainingValue)
	}
	if depositGas-remainingGas > 0 {
		gasValue := new(big.Int).Mul(new(big.Int).SetUint64(depositGas-remainingGas), ps.context.GasPrice)
		stateDB.AddBalance(ps.context.Coinbase, gasValue)
	}
	receipt.Logs = stateDB.Logs()
	return receipt, err
}

func intrinsicGas(evmParams *EVMParams) uint64 {
	return uint64(10)
}

func executeInEVM(evmParams *EVMParams, stateDB *EVMStateDBAdapter, gasLimit *uint64) (uint64, uint64, string, error) {
	remainingGas := evmParams.gas
	if err := securityDeposit(evmParams, stateDB, gasLimit); err != nil {
		return 0, 0, action.EmptyAddress, err
	}
	var chainConfig params.ChainConfig
	var config vm.Config
	evm := vm.NewEVM(evmParams.context, stateDB, &chainConfig, config)
	intriGas := intrinsicGas(evmParams)
	if remainingGas < intriGas {
		return evmParams.gas, remainingGas, action.EmptyAddress, ErrOutOfGas
	}
	var err error
	contractRawAddress := action.EmptyAddress
	remainingGas -= intriGas
	executor := vm.AccountRef(evmParams.context.Origin)
	if evmParams.contract == nil {
		// create contract
		_, evmContractAddress, remainingGas, err := evm.Create(executor, evmParams.data, remainingGas, evmParams.amount)
		if err != nil {
			return evmParams.gas, remainingGas, action.EmptyAddress, err
		}
		contractAddress, err := iotxaddress.GetAddressByHash(iotxaddress.IsTestnet, iotxaddress.ChainID, evmContractAddress.Bytes())
		if err != nil {
			return evmParams.gas, remainingGas, action.EmptyAddress, err
		}
		contractRawAddress = contractAddress.RawAddress
	} else {
		// process contract
		_, remainingGas, err = evm.Call(executor, *evmParams.contract, evmParams.data, remainingGas, evmParams.amount)
	}
	if err == nil {
		err = stateDB.Error()
	}
	if err == vm.ErrInsufficientBalance {
		return evmParams.gas, remainingGas, action.EmptyAddress, err
	}
	if err != nil {
		// TODO (zhi) should we refund if any error
		return evmParams.gas, 0, action.EmptyAddress, err
	}
	// TODO (zhi) figure out what the following function does
	// stateDB.Finalise(true)
	return evmParams.gas, remainingGas, contractRawAddress, nil
}
