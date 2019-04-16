// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package evm

import (
	"context"
	"math"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/pkg/log"
)

// ErrInconsistentNonce is the error that the nonce is different from executor's nonce
var ErrInconsistentNonce = errors.New("Nonce is not identical to executor nonce")

// CanTransfer checks whether the from account has enough balance
func CanTransfer(db vm.StateDB, fromHash common.Address, balance *big.Int) bool {
	return db.GetBalance(fromHash).Cmp(balance) >= 0
}

// MakeTransfer transfers account
func MakeTransfer(db vm.StateDB, fromHash, toHash common.Address, amount *big.Int) {
	db.SubBalance(fromHash, amount)
	db.AddBalance(toHash, amount)
}

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
func NewParams(raCtx protocol.RunActionsCtx, execution *action.Execution, stateDB *StateDBAdapter) (*Params, error) {
	// If we don't have an explicit author (i.e. not mining), extract from the header
	/*
		var beneficiary common.Address
		if author == nil {
			beneficiary, _ = chain.Engine().Author(header) // Ignore error, we're past header validation
		} else {
			beneficiary = *author
		}
	*/
	executorAddr := common.BytesToAddress(raCtx.Caller.Bytes())
	var contractAddrPointer *common.Address
	if execution.Contract() != action.EmptyAddress {
		contract, err := address.FromString(execution.Contract())
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert encoded contract address to address")
		}
		contractAddr := common.BytesToAddress(contract.Bytes())
		contractAddrPointer = &contractAddr
	}
	producer := common.BytesToAddress(raCtx.Producer.Bytes())

	gasLimit := execution.GasLimit()
	// Reset gas limit to the system wide action gas limit cap if it's greater than it
	if gasLimit > raCtx.ActionGasLimit {
		gasLimit = raCtx.ActionGasLimit
	}

	context := vm.Context{
		CanTransfer: CanTransfer,
		Transfer:    MakeTransfer,
		GetHash:     GetHashFn(stateDB),
		Origin:      executorAddr,
		Coinbase:    producer,
		BlockNumber: new(big.Int).SetUint64(raCtx.BlockHeight),
		Time:        new(big.Int).SetInt64(raCtx.BlockTimeStamp.Unix()),
		Difficulty:  new(big.Int).SetUint64(uint64(50)),
		GasLimit:    gasLimit,
		GasPrice:    execution.GasPrice(),
	}

	return &Params{
		context,
		execution.Nonce(),
		raCtx.Caller.String(),
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

func securityDeposit(ps *Params, stateDB vm.StateDB, gasLimit uint64) error {
	executorNonce := stateDB.GetNonce(ps.context.Origin)
	if executorNonce > ps.nonce {
		log.S().Errorf("Nonce on %v: %d vs %d", ps.context.Origin, executorNonce, ps.nonce)
		// TODO ignore inconsistent nonce problem until the actions are executed sequentially
		// return ErrInconsistentNonce
	}
	if gasLimit < ps.gas {
		return action.ErrHitGasLimit
	}
	maxGasValue := new(big.Int).Mul(new(big.Int).SetUint64(ps.gas), ps.context.GasPrice)
	if stateDB.GetBalance(ps.context.Origin).Cmp(maxGasValue) < 0 {
		return action.ErrInsufficientBalanceForGas
	}
	stateDB.SubBalance(ps.context.Origin, maxGasValue)
	return nil
}

// ExecuteContract processes a transfer which contains a contract
func ExecuteContract(
	ctx context.Context,
	sm protocol.StateManager,
	execution *action.Execution,
	cm protocol.ChainManager,
) ([]byte, *action.Receipt, error) {
	raCtx := protocol.MustGetRunActionsCtx(ctx)
	stateDB := NewStateDBAdapter(cm, sm, raCtx.BlockHeight, execution.Hash())
	ps, err := NewParams(raCtx, execution, stateDB)
	if err != nil {
		return nil, nil, err
	}
	retval, depositGas, remainingGas, contractAddress, failed, err := executeInEVM(ps, stateDB, raCtx.GasLimit)
	if err != nil {
		return nil, nil, err
	}
	receipt := &action.Receipt{
		GasConsumed:     ps.gas - remainingGas,
		BlockHeight:     raCtx.BlockHeight,
		ActionHash:      execution.Hash(),
		ContractAddress: contractAddress,
	}
	if failed {
		receipt.Status = action.FailureReceiptStatus
	} else {
		receipt.Status = action.SuccessReceiptStatus
	}
	if remainingGas > 0 {
		remainingValue := new(big.Int).Mul(new(big.Int).SetUint64(remainingGas), ps.context.GasPrice)
		stateDB.AddBalance(ps.context.Origin, remainingValue)
	}
	if depositGas-remainingGas > 0 {
		gasValue := new(big.Int).Mul(new(big.Int).SetUint64(depositGas-remainingGas), ps.context.GasPrice)
		if err := rewarding.DepositGas(ctx, sm, gasValue, raCtx.Registry); err != nil {
			return nil, nil, err
		}
	}

	if err := stateDB.CommitContracts(); err != nil {
		return nil, nil, errors.Wrap(err, "failed to commit contracts to underlying db")
	}
	stateDB.clear()
	receipt.Logs = stateDB.Logs()
	log.S().Debugf("Receipt: %+v, %v", receipt, err)
	return retval, receipt, nil
}

func getChainConfig() *params.ChainConfig {
	var chainConfig params.ChainConfig
	// chainConfig.ChainID
	chainConfig.ConstantinopleBlock = new(big.Int).SetUint64(0) // Constantinople switch block (nil = no fork, 0 = already activated)

	return &chainConfig
}

func executeInEVM(evmParams *Params, stateDB *StateDBAdapter, gasLimit uint64) ([]byte, uint64, uint64, string, bool, error) {
	remainingGas := evmParams.gas
	if err := securityDeposit(evmParams, stateDB, gasLimit); err != nil {
		log.L().Warn("unexpected error: not enough security deposit", zap.Error(err))
		return nil, 0, 0, action.EmptyAddress, true, err
	}
	var config vm.Config
	chainConfig := getChainConfig()
	evm := vm.NewEVM(evmParams.context, stateDB, chainConfig, config)
	intriGas, err := intrinsicGas(evmParams.data)
	if err != nil {
		return nil, evmParams.gas, remainingGas, action.EmptyAddress, true, err
	}
	if remainingGas < intriGas {
		return nil, evmParams.gas, remainingGas, action.EmptyAddress, true, action.ErrOutOfGas
	}
	remainingGas -= intriGas
	contractRawAddress := action.EmptyAddress
	executor := vm.AccountRef(evmParams.context.Origin)
	var ret []byte
	var evmErr error
	if evmParams.contract == nil {
		// create contract
		var evmContractAddress common.Address
		_, evmContractAddress, remainingGas, evmErr = evm.Create(executor, evmParams.data, remainingGas, evmParams.amount)
		log.L().Debug("evm Create.", log.Hex("addrHash", evmContractAddress[:]))
		if evmErr == nil {
			if contractAddress, err := address.FromBytes(evmContractAddress.Bytes()); err == nil {
				contractRawAddress = contractAddress.String()
			}
		}
	} else {
		stateDB.SetNonce(evmParams.context.Origin, stateDB.GetNonce(evmParams.context.Origin)+1)
		// process contract
		ret, remainingGas, evmErr = evm.Call(executor, *evmParams.contract, evmParams.data, remainingGas, evmParams.amount)
	}
	if evmErr != nil {
		log.L().Debug("evm error", zap.Error(err))
		if err == vm.ErrInsufficientBalance {
			return nil, evmParams.gas, remainingGas, action.EmptyAddress, true, evmErr
		}
	}
	if stateDB.Error() != nil {
		log.L().Debug("statedb error", zap.Error(stateDB.Error()))
	}
	refund := (evmParams.gas - remainingGas) / 2
	if refund > stateDB.GetRefund() {
		refund = stateDB.GetRefund()
	}
	remainingGas += refund

	return ret, evmParams.gas, remainingGas, contractRawAddress, evmErr != nil, nil
}

// intrinsicGas returns the intrinsic gas of an execution
func intrinsicGas(data []byte) (uint64, error) {
	dataSize := uint64(len(data))
	if (math.MaxInt64-action.ExecutionBaseIntrinsicGas)/action.ExecutionDataGas < dataSize {
		return 0, action.ErrOutOfGas
	}

	return dataSize*action.ExecutionDataGas + action.ExecutionBaseIntrinsicGas, nil
}
