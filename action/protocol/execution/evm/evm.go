// Copyright (c) 2019 IoTeX Foundation
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
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

var (
	// TODO: whenever ActionGasLimit is removed from genesis, we need to hard code it to 5M to make it compatible with
	// the mainnet.
	preAleutianActionGasLimit = genesis.Default.ActionGasLimit

	// ErrInconsistentNonce is the error that the nonce is different from executor's nonce
	ErrInconsistentNonce = errors.New("Nonce is not identical to executor nonce")
)

// CanTransfer checks whether the from account has enough balance
func CanTransfer(db vm.StateDB, fromHash common.Address, balance *big.Int) bool {
	return db.GetBalance(fromHash).Cmp(balance) >= 0
}

// MakeTransfer transfers account
func MakeTransfer(db vm.StateDB, fromHash, toHash common.Address, amount *big.Int) {
	db.SubBalance(fromHash, amount)
	db.AddBalance(toHash, amount)
}

type (
	// Params is the context and parameters
	Params struct {
		context            vm.Context
		nonce              uint64
		executorRawAddress string
		amount             *big.Int
		contract           *common.Address
		gas                uint64
		data               []byte
	}
)

// NewParams creates a new context for use in the EVM.
func NewParams(
	ctx context.Context,
	execution *action.Execution,
	stateDB *StateDBAdapter,
	getBlockHash GetBlockHash,
) (*Params, error) {
	actionCtx := protocol.MustGetActionCtx(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	executorAddr := common.BytesToAddress(actionCtx.Caller.Bytes())
	var contractAddrPointer *common.Address
	if execution.Contract() != action.EmptyAddress {
		contract, err := address.FromString(execution.Contract())
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert encoded contract address to address")
		}
		contractAddr := common.BytesToAddress(contract.Bytes())
		contractAddrPointer = &contractAddr
	}

	gasLimit := execution.GasLimit()
	hu := config.NewHeightUpgrade(&bcCtx.Genesis)
	// Reset gas limit to the system wide action gas limit cap if it's greater than it
	if blkCtx.BlockHeight > 0 && hu.IsPre(config.Aleutian, blkCtx.BlockHeight) && gasLimit > preAleutianActionGasLimit {
		gasLimit = preAleutianActionGasLimit
	}

	context := vm.Context{
		CanTransfer: CanTransfer,
		Transfer:    MakeTransfer,
		GetHash: func(n uint64) common.Hash {
			hash, err := getBlockHash(stateDB.blockHeight - n)
			if err != nil {
				return common.BytesToHash(hash[:])
			}

			return common.Hash{}
		},
		Origin:      executorAddr,
		Coinbase:    common.BytesToAddress(blkCtx.Producer.Bytes()),
		BlockNumber: new(big.Int).SetUint64(blkCtx.BlockHeight),
		Time:        new(big.Int).SetInt64(blkCtx.BlockTimeStamp.Unix()),
		Difficulty:  new(big.Int).SetUint64(uint64(50)),
		GasLimit:    gasLimit,
		GasPrice:    execution.GasPrice(),
	}

	return &Params{
		context,
		execution.Nonce(),
		actionCtx.Caller.String(),
		execution.Amount(),
		contractAddrPointer,
		gasLimit,
		execution.Data(),
	}, nil
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
	getBlockHash GetBlockHash,
) ([]byte, *action.Receipt, error) {
	blkCtx := protocol.MustGetBlockCtx(ctx)
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	hu := config.NewHeightUpgrade(&bcCtx.Genesis)
	stateDB := NewStateDBAdapter(
		sm,
		blkCtx.BlockHeight,
		hu.IsPre(config.Aleutian, blkCtx.BlockHeight),
		execution.Hash(),
	)
	ps, err := NewParams(ctx, execution, stateDB, getBlockHash)
	if err != nil {
		return nil, nil, err
	}
	retval, depositGas, remainingGas, contractAddress, statusCode, err := executeInEVM(ps, stateDB, hu, blkCtx.GasLimit, blkCtx.BlockHeight)
	if err != nil {
		return nil, nil, err
	}
	receipt := &action.Receipt{
		GasConsumed:     ps.gas - remainingGas,
		BlockHeight:     blkCtx.BlockHeight,
		ActionHash:      execution.Hash(),
		ContractAddress: contractAddress,
	}

	receipt.Status = statusCode

	if hu.IsPost(config.Pacific, blkCtx.BlockHeight) {
		// Refund all deposit and, actual gas fee will be subtracted when depositing gas fee to the rewarding protocol
		stateDB.AddBalance(ps.context.Origin, big.NewInt(0).Mul(big.NewInt(0).SetUint64(depositGas), ps.context.GasPrice))
	} else {
		if remainingGas > 0 {
			remainingValue := new(big.Int).Mul(new(big.Int).SetUint64(remainingGas), ps.context.GasPrice)
			stateDB.AddBalance(ps.context.Origin, remainingValue)
		}
	}
	if depositGas-remainingGas > 0 {
		gasValue := new(big.Int).Mul(new(big.Int).SetUint64(depositGas-remainingGas), ps.context.GasPrice)
		if err := rewarding.DepositGas(ctx, sm, gasValue); err != nil {
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

func getChainConfig(beringHeight uint64) *params.ChainConfig {
	var chainConfig params.ChainConfig
	// chainConfig.ChainID
	chainConfig.ConstantinopleBlock = new(big.Int).SetUint64(0) // Constantinople switch block (nil = no fork, 0 = already activated)
	chainConfig.BeringBlock = new(big.Int).SetUint64(beringHeight)
	return &chainConfig
}

//Error in executeInEVM is a consensus issue
func executeInEVM(evmParams *Params, stateDB *StateDBAdapter, hu config.HeightUpgrade, gasLimit uint64, blockHeight uint64) ([]byte, uint64, uint64, string, uint64, error) {
	isBering := hu.IsPost(config.Bering, blockHeight)
	remainingGas := evmParams.gas
	if err := securityDeposit(evmParams, stateDB, gasLimit); err != nil {
		log.L().Warn("unexpected error: not enough security deposit", zap.Error(err))
		return nil, 0, 0, action.EmptyAddress, uint64(iotextypes.ReceiptStatus_Failure), err
	}
	var config vm.Config
	chainConfig := getChainConfig(hu.BeringBlockHeight())
	evm := vm.NewEVM(evmParams.context, stateDB, chainConfig, config)
	intriGas, err := intrinsicGas(evmParams.data)
	if err != nil {
		return nil, evmParams.gas, remainingGas, action.EmptyAddress, uint64(iotextypes.ReceiptStatus_Failure), err
	}
	if remainingGas < intriGas {
		return nil, evmParams.gas, remainingGas, action.EmptyAddress, uint64(iotextypes.ReceiptStatus_Failure), action.ErrOutOfGas
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
		log.L().Debug("evm error", zap.Error(evmErr))
		// The only possible consensus-error would be if there wasn't
		// sufficient balance to make the transfer happen.
		// Should be a hard fork (Bering)
		if evmErr == vm.ErrInsufficientBalance && isBering {
			return nil, evmParams.gas, remainingGas, action.EmptyAddress, uint64(iotextypes.ReceiptStatus_Failure), evmErr
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

	if evmErr != nil {
		return ret, evmParams.gas, remainingGas, contractRawAddress, evmErrToErrStatusCode(evmErr, isBering), nil
	}
	return ret, evmParams.gas, remainingGas, contractRawAddress, uint64(iotextypes.ReceiptStatus_Success), nil
}

// evmErrToErrStatusCode returns ReceiptStatuscode which describes error type
func evmErrToErrStatusCode(evmErr error, isBering bool) (errStatusCode uint64) {
	if isBering {
		switch evmErr {
		case vm.ErrOutOfGas:
			errStatusCode = uint64(iotextypes.ReceiptStatus_ErrOutOfGas)
		case vm.ErrCodeStoreOutOfGas:
			errStatusCode = uint64(iotextypes.ReceiptStatus_ErrCodeStoreOutOfGas)
		case vm.ErrDepth:
			errStatusCode = uint64(iotextypes.ReceiptStatus_ErrDepth)
		case vm.ErrContractAddressCollision:
			errStatusCode = uint64(iotextypes.ReceiptStatus_ErrContractAddressCollision)
		case vm.ErrNoCompatibleInterpreter:
			errStatusCode = uint64(iotextypes.ReceiptStatus_ErrNoCompatibleInterpreter)
		default:
			//This errors from go-ethereum, are not-accessible variable.
			switch evmErr.Error() {
			case "evm: execution reverted":
				errStatusCode = uint64(iotextypes.ReceiptStatus_ErrExecutionReverted)
			case "evm: max code size exceeded":
				errStatusCode = uint64(iotextypes.ReceiptStatus_ErrMaxCodeSizeExceeded)
			case "evm: write protection":
				errStatusCode = uint64(iotextypes.ReceiptStatus_ErrWriteProtection)
			default:
				errStatusCode = uint64(iotextypes.ReceiptStatus_ErrUnknown)
			}
		}
	} else {
		// it prevents from breaking chain
		errStatusCode = uint64(iotextypes.ReceiptStatus_Failure)
	}
	return
}

// intrinsicGas returns the intrinsic gas of an execution
func intrinsicGas(data []byte) (uint64, error) {
	dataSize := uint64(len(data))
	if (math.MaxInt64-action.ExecutionBaseIntrinsicGas)/action.ExecutionDataGas < dataSize {
		return 0, action.ErrOutOfGas
	}

	return dataSize*action.ExecutionDataGas + action.ExecutionBaseIntrinsicGas, nil
}
