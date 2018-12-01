// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package evm

import (
	"math"
	"math/big"

	"github.com/CoderZhi/go-ethereum/common"
	"github.com/CoderZhi/go-ethereum/core/vm"
	"github.com/CoderZhi/go-ethereum/params"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
)

// GasLimit is the total gas limit could be consumed in a block
const GasLimit = uint64(1000000000)

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

// Params is the context and parameters
type Params struct {
	context            vm.Context
	nonce              uint64
	executorRawAddress string
	amount             *big.Int
	contract           *common.Address
	gas                uint64
	data               []byte
}

// NewParams creates a new context for use in the EVM.
func NewParams(blkHeight uint64, producerPubKey keypair.PublicKey, blkTimeStamp int64, execution *action.Execution, stateDB *StateDBAdapter) (*Params, error) {
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
	var contractAddrPointer *common.Address
	if execution.Contract() != action.EmptyAddress {
		contractHash, err := iotxaddress.GetPubkeyHash(execution.Contract())
		if err != nil {
			return nil, err
		}
		contractAddr := common.BytesToAddress(contractHash)
		contractAddrPointer = &contractAddr
	}
	producerHash := keypair.HashPubKey(producerPubKey)
	producer := common.BytesToAddress(producerHash[:])
	context := vm.Context{
		CanTransfer: CanTransfer,
		Transfer:    MakeTransfer,
		GetHash:     GetHashFn(stateDB),
		Origin:      executorAddr,
		Coinbase:    producer,
		BlockNumber: new(big.Int).SetUint64(blkHeight),
		Time:        new(big.Int).SetInt64(blkTimeStamp),
		Difficulty:  new(big.Int).SetUint64(uint64(50)),
		GasLimit:    GasLimit,
		GasPrice:    execution.GasPrice(),
	}

	return &Params{
		context,
		execution.Nonce(),
		execution.Executor(),
		execution.Amount(),
		contractAddrPointer,
		execution.GasLimit(),
		execution.Data(),
	}, nil
}

// GetHashFn returns a GetHashFunc which retrieves hashes by number
func GetHashFn(stateDB *StateDBAdapter) func(n uint64) common.Hash {
	return func(n uint64) common.Hash {
		hash, err := stateDB.cm.GetHashByHeight(stateDB.blockHeight - n)
		if err != nil {
			return common.BytesToHash(hash[:])
		}

		return common.Hash{}
	}
}

func securityDeposit(ps *Params, stateDB vm.StateDB, gasLimit *uint64) error {
	executorNonce := stateDB.GetNonce(ps.context.Origin)
	if executorNonce > ps.nonce {
		logger.Error().Msgf("Nonce on %v: %d vs %d", ps.context.Origin, executorNonce, ps.nonce)
		// TODO ignore inconsistent nonce problem until the actions are executed sequentially
		// return ErrInconsistentNonce
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

// ExecuteContract processes a transfer which contains a contract
func ExecuteContract(
	blkHeight uint64,
	blkHash hash.Hash32B,
	producerPubKey keypair.PublicKey,
	blkTimeStamp int64,
	sm protocol.StateManager,
	execution *action.Execution,
	cm protocol.ChainManager,
	gasLimit *uint64,
	enableGasCharge bool,
) (*action.Receipt, error) {
	stateDB := NewStateDBAdapter(cm, sm, blkHeight, blkHash, execution.Hash())
	ps, err := NewParams(blkHeight, producerPubKey, blkTimeStamp, execution, stateDB)
	if err != nil {
		return nil, err
	}
	retval, depositGas, remainingGas, contractAddress, err := executeInEVM(ps, stateDB, gasLimit)
	if !enableGasCharge {
		remainingGas = depositGas
	}

	receipt := &action.Receipt{
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

	if err := stateDB.commitContracts(); err != nil {
		return nil, errors.Wrap(err, "failed to commit contracts to underlying db")
	}
	stateDB.clearCachedContracts()
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

func executeInEVM(evmParams *Params, stateDB *StateDBAdapter, gasLimit *uint64) ([]byte, uint64, uint64, string, error) {
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
	remainingGas -= intriGas
	contractRawAddress := action.EmptyAddress
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
		contractAddress := address.New(stateDB.cm.ChainID(), evmContractAddress.Bytes())
		contractRawAddress = contractAddress.IotxAddress()
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
