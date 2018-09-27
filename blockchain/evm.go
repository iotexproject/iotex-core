// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"math"
	"math/big"

	"github.com/CoderZhi/go-ethereum/common"
	"github.com/CoderZhi/go-ethereum/core/vm"
	"github.com/CoderZhi/go-ethereum/params"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/blockchain/action"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
)

// ErrInconsistentNonce is the error that the nonce is different from executor's nonce
var ErrInconsistentNonce = errors.New("Nonce is not identical to executor nonce")

// CanTransfer checks whether the from account has enough balance
func CanTransfer(db vm.StateDB, fromHash common.Address, balance *big.Int) bool {
	return db.GetBalance(fromHash).Cmp(balance) > 0
}

// MakeTransfer transfers account
func MakeTransfer(db vm.StateDB, fromHash, toHash common.Address, amount *big.Int) {
	db.SubBalance(fromHash, amount)
	db.AddBalance(toHash, amount)
}

const (
	// FailureStatus is the status that contract execution failed
	FailureStatus = uint64(0)
	// SuccessStatus is the status that contract execution success
	SuccessStatus = uint64(1)
)

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
	executorHash, err := iotxaddress.GetPubkeyHash(execution.Executor())
	if err != nil {
		return nil, err
	}
	executorAddr := common.BytesToAddress(executorHash)
	executorIoTXAddress, err := iotxaddress.GetAddressByHash(iotxaddress.IsTestnet, iotxaddress.ChainID, executorHash)
	if err != nil {
		return nil, err
	}
	var contractAddrPointer *common.Address
	if execution.Contract() != action.EmptyAddress {
		contractHash, err := iotxaddress.GetPubkeyHash(execution.Contract())
		if err != nil {
			return nil, err
		}
		contractAddr := common.BytesToAddress(contractHash)
		contractAddrPointer = &contractAddr
	}
	producerHash := keypair.HashPubKey(blk.Header.Pubkey)
	producer := common.BytesToAddress(producerHash[:])
	context := vm.Context{
		CanTransfer: CanTransfer,
		Transfer:    MakeTransfer,
		GetHash:     GetHashFn(stateDB),
		Origin:      executorAddr,
		Coinbase:    producer,
		BlockNumber: new(big.Int).SetUint64(blk.Height()),
		Time:        new(big.Int).SetInt64(blk.Header.Timestamp().Unix()),
		Difficulty:  new(big.Int).SetUint64(uint64(50)),
		GasLimit:    action.GasLimit,
		GasPrice:    execution.GasPrice(),
	}

	return &EVMParams{
		context,
		execution.Nonce(),
		executorIoTXAddress.RawAddress,
		execution.Amount(),
		contractAddrPointer,
		execution.GasLimit(),
		execution.Data(),
	}, nil
}

// GetHashFn returns a GetHashFunc which retrieves hashes by number
func GetHashFn(stateDB *EVMStateDBAdapter) func(n uint64) common.Hash {
	return func(n uint64) common.Hash {
		hash, err := stateDB.bc.GetHashByHeight(stateDB.blockHeight - n)
		if err != nil {
			return common.BytesToHash(hash[:])
		}

		return common.Hash{}
	}
}

func securityDeposit(ps *EVMParams, stateDB vm.StateDB, gasLimit *uint64) error {
	executorNonce := stateDB.GetNonce(ps.context.Origin)
	if executorNonce > ps.nonce {
		logger.Error().Msgf("Nonce on %v: %d vs %d", ps.context.Origin, executorNonce, ps.nonce)
		return ErrInconsistentNonce
	}
	if *gasLimit < ps.gas {
		return action.ErrHitGasLimit
	}
	maxGasValue := new(big.Int).Mul(new(big.Int).SetUint64(ps.gas), ps.context.GasPrice)
	if stateDB.GetBalance(ps.context.Origin).Cmp(maxGasValue) < 0 {
		return action.ErrInsufficientBalanceForGas
	}
	*gasLimit -= ps.gas
	stateDB.SubBalance(ps.context.Origin, maxGasValue)
	return nil
}

// ExecuteContracts process the contracts in a block
func ExecuteContracts(blk *Block, bc Blockchain) {
	gasLimit := action.GasLimit
	blk.receipts = make(map[hash.Hash32B]*Receipt)
	for idx, execution := range blk.Executions {
		// TODO (zhi) log receipt to stateDB
		if receipt, _ := executeContract(blk, idx, execution, bc, &gasLimit); receipt != nil {
			blk.receipts[execution.Hash()] = receipt
		}
	}
}

// executeContract processes a transfer which contains a contract
func executeContract(blk *Block, idx int, execution *action.Execution, bc Blockchain, gasLimit *uint64) (*Receipt, error) {
	stateDB := NewEVMStateDBAdapter(bc, blk.Height(), blk.HashBlock(), uint(idx), execution.Hash())
	ps, err := NewEVMParams(blk, execution, stateDB)
	if err != nil {
		return nil, err
	}
	retval, depositGas, remainingGas, contractAddress, err := executeInEVM(ps, stateDB, gasLimit)
	receipt := &Receipt{
		ReturnValue:     retval,
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
	logger.Debug().Msgf("Receipt: %+v, %v", receipt, err)
	return receipt, err
}

func getChainConfig() *params.ChainConfig {
	var chainConfig params.ChainConfig
	// chainConfig.ChainID
	chainConfig.ConstantinopleBlock = new(big.Int).SetUint64(0) // Constantinople switch block (nil = no fork, 0 = already activated)

	return &chainConfig
}

func executeInEVM(evmParams *EVMParams, stateDB *EVMStateDBAdapter, gasLimit *uint64) ([]byte, uint64, uint64, string, error) {
	remainingGas := evmParams.gas
	if err := securityDeposit(evmParams, stateDB, gasLimit); err != nil {
		return nil, 0, 0, action.EmptyAddress, err
	}
	var config vm.Config
	chainConfig := getChainConfig()
	evm := vm.NewEVM(evmParams.context, stateDB, chainConfig, config)
	intriGas, err := intrinsicGas(evmParams.data)
	if err != nil {
		return nil, evmParams.gas, remainingGas, action.EmptyAddress, err
	}
	if remainingGas < intriGas {
		return nil, evmParams.gas, remainingGas, action.EmptyAddress, action.ErrOutOfGas
	}
	contractRawAddress := action.EmptyAddress
	remainingGas -= intriGas
	executor := vm.AccountRef(evmParams.context.Origin)
	var ret []byte
	if evmParams.contract == nil {
		// create contract
		var evmContractAddress common.Address
		ret, evmContractAddress, remainingGas, err = evm.Create(executor, evmParams.data, remainingGas, evmParams.amount)
		logger.Warn().Hex("contract addrHash", evmContractAddress[:]).Msg("evm.Create")
		if err != nil {
			return nil, evmParams.gas, remainingGas, action.EmptyAddress, err
		}
		contractAddress, err := iotxaddress.GetAddressByHash(iotxaddress.IsTestnet, iotxaddress.ChainID, evmContractAddress.Bytes())
		if err != nil {
			return nil, evmParams.gas, remainingGas, action.EmptyAddress, err
		}
		contractRawAddress = contractAddress.RawAddress
	} else {
		// process contract
		ret, remainingGas, err = evm.Call(executor, *evmParams.contract, evmParams.data, remainingGas, evmParams.amount)
	}
	if err == nil {
		err = stateDB.Error()
	}
	if err == vm.ErrInsufficientBalance {
		return nil, evmParams.gas, remainingGas, action.EmptyAddress, err
	}
	if err != nil {
		// TODO (zhi) should we refund if any error
		return nil, evmParams.gas, 0, contractRawAddress, err
	}
	// TODO (zhi) figure out what the following function does
	// stateDB.Finalise(true)
	return ret, evmParams.gas, remainingGas, contractRawAddress, nil
}

// intrinsicGas returns the intrinsic gas of an execution
func intrinsicGas(data []byte) (uint64, error) {
	dataSize := uint64(len(data))
	if (math.MaxInt64-action.ExecutionBaseIntrinsicGas)/action.ExecutionDataGas < dataSize {
		return 0, action.ErrOutOfGas
	}

	return dataSize*action.ExecutionDataGas + action.ExecutionBaseIntrinsicGas, nil
}
