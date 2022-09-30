// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package evm

import (
	"bytes"
	"context"
	"math"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/tracer"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

var (
	// TODO: whenever ActionGasLimit is removed from genesis, we need to hard code it to 5M to make it compatible with
	// the mainnet.
	_preAleutianActionGasLimit = genesis.Default.ActionGasLimit

	_inContractTransfer = hash.BytesToHash256([]byte{byte(iotextypes.TransactionLogType_IN_CONTRACT_TRANSFER)})

	// _revertSelector is a special function selector for revert reason unpacking.
	_revertSelector = crypto.Keccak256([]byte("Error(string)"))[:4]

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

	db.AddLog(&types.Log{
		Topics: []common.Hash{
			common.BytesToHash(_inContractTransfer[:]),
			common.BytesToHash(fromHash[:]),
			common.BytesToHash(toHash[:]),
		},
		Data: amount.Bytes(),
	})
}

type (
	// Params is the context and parameters
	Params struct {
		context            vm.BlockContext
		txCtx              vm.TxContext
		nonce              uint64
		evmNetworkID       uint32
		executorRawAddress string
		amount             *big.Int
		contract           *common.Address
		gas                uint64
		data               []byte
		accessList         types.AccessList
	}
)

// newParams creates a new context for use in the EVM.
func newParams(
	ctx context.Context,
	execution *action.Execution,
	stateDB *StateDBAdapter,
	getBlockHash GetBlockHash,
) (*Params, error) {
	actionCtx := protocol.MustGetActionCtx(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)
	featureCtx := protocol.MustGetFeatureCtx(ctx)
	executorAddr := common.BytesToAddress(actionCtx.Caller.Bytes())
	var contractAddrPointer *common.Address
	if dest := execution.Contract(); dest != action.EmptyAddress {
		contract, err := address.FromString(execution.Contract())
		if err != nil {
			return nil, errors.Wrapf(err, "failed to decode contract address %s", dest)
		}
		contractAddr := common.BytesToAddress(contract.Bytes())
		contractAddrPointer = &contractAddr
	}

	gasLimit := execution.GasLimit()
	// Reset gas limit to the system wide action gas limit cap if it's greater than it
	if blkCtx.BlockHeight > 0 && featureCtx.SystemWideActionGasLimit && gasLimit > _preAleutianActionGasLimit {
		gasLimit = _preAleutianActionGasLimit
	}

	var getHashFn vm.GetHashFunc
	switch {
	case featureCtx.CorrectGetHashFn:
		getHashFn = func(n uint64) common.Hash {
			hash, err := getBlockHash(n)
			if err == nil {
				return common.BytesToHash(hash[:])
			}
			return common.Hash{}
		}
	case featureCtx.FixGetHashFnHeight:
		getHashFn = func(n uint64) common.Hash {
			hash, err := getBlockHash(stateDB.blockHeight - (n + 1))
			if err == nil {
				return common.BytesToHash(hash[:])
			}
			return common.Hash{}
		}
	default:
		getHashFn = func(n uint64) common.Hash {
			hash, err := getBlockHash(stateDB.blockHeight - n)
			if err != nil {
				// initial implementation did wrong, should return common.Hash{} in case of error
				return common.BytesToHash(hash[:])
			}
			return common.Hash{}
		}
	}

	context := vm.BlockContext{
		CanTransfer: CanTransfer,
		Transfer:    MakeTransfer,
		GetHash:     getHashFn,
		Coinbase:    common.BytesToAddress(blkCtx.Producer.Bytes()),
		GasLimit:    gasLimit,
		BlockNumber: new(big.Int).SetUint64(blkCtx.BlockHeight),
		Time:        new(big.Int).SetInt64(blkCtx.BlockTimeStamp.Unix()),
		Difficulty:  new(big.Int).SetUint64(uint64(50)),
		BaseFee:     new(big.Int),
	}

	return &Params{
		context,
		vm.TxContext{
			Origin:   executorAddr,
			GasPrice: execution.GasPrice(),
		},
		execution.Nonce(),
		protocol.MustGetBlockchainCtx(ctx).EvmNetworkID,
		actionCtx.Caller.String(),
		execution.Amount(),
		contractAddrPointer,
		gasLimit,
		execution.Data(),
		execution.AccessList(),
	}, nil
}

func securityDeposit(ps *Params, stateDB vm.StateDB, gasLimit uint64) error {
	executorNonce := stateDB.GetNonce(ps.txCtx.Origin)
	if executorNonce > ps.nonce {
		log.S().Errorf("Nonce on %v: %d vs %d", ps.txCtx.Origin, executorNonce, ps.nonce)
		// TODO ignore inconsistent nonce problem until the actions are executed sequentially
		// return ErrInconsistentNonce
	}
	if gasLimit < ps.gas {
		return action.ErrGasLimit
	}
	gasConsumed := new(big.Int).Mul(new(big.Int).SetUint64(ps.gas), ps.txCtx.GasPrice)
	if stateDB.GetBalance(ps.txCtx.Origin).Cmp(gasConsumed) < 0 {
		return action.ErrInsufficientFunds
	}
	stateDB.SubBalance(ps.txCtx.Origin, gasConsumed)
	return nil
}

// ExecuteContract processes a transfer which contains a contract
func ExecuteContract(
	ctx context.Context,
	sm protocol.StateManager,
	execution *action.Execution,
	getBlockHash GetBlockHash,
	depositGasFunc DepositGas,
) ([]byte, *action.Receipt, error) {
	ctx, span := tracer.NewSpan(ctx, "evm.ExecuteContract")
	defer span.End()
	actionCtx := protocol.MustGetActionCtx(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)
	g := genesis.MustExtractGenesisContext(ctx)
	featureCtx := protocol.MustGetFeatureCtx(ctx)
	stateDB, err := prepareStateDB(ctx, sm)
	if err != nil {
		return nil, nil, err
	}
	ps, err := newParams(ctx, execution, stateDB, getBlockHash)
	if err != nil {
		return nil, nil, err
	}
	retval, depositGas, remainingGas, contractAddress, statusCode, err := executeInEVM(ctx, ps, stateDB, g.Blockchain, blkCtx.GasLimit, blkCtx.BlockHeight)
	if err != nil {
		return nil, nil, err
	}
	receipt := &action.Receipt{
		GasConsumed:     ps.gas - remainingGas,
		BlockHeight:     blkCtx.BlockHeight,
		ActionHash:      actionCtx.ActionHash,
		ContractAddress: contractAddress,
	}

	receipt.Status = uint64(statusCode)
	var burnLog *action.TransactionLog
	if featureCtx.FixDoubleChargeGas {
		// Refund all deposit and, actual gas fee will be subtracted when depositing gas fee to the rewarding protocol
		stateDB.AddBalance(ps.txCtx.Origin, big.NewInt(0).Mul(big.NewInt(0).SetUint64(depositGas), ps.txCtx.GasPrice))
	} else {
		if remainingGas > 0 {
			remainingValue := new(big.Int).Mul(new(big.Int).SetUint64(remainingGas), ps.txCtx.GasPrice)
			stateDB.AddBalance(ps.txCtx.Origin, remainingValue)
		}
		if depositGas-remainingGas > 0 {
			burnLog = &action.TransactionLog{
				Type:      iotextypes.TransactionLogType_GAS_FEE,
				Sender:    actionCtx.Caller.String(),
				Recipient: "", // burned
				Amount:    new(big.Int).Mul(new(big.Int).SetUint64(depositGas-remainingGas), ps.txCtx.GasPrice),
			}
		}
	}
	var depositLog *action.TransactionLog
	if depositGas-remainingGas > 0 {
		gasValue := new(big.Int).Mul(new(big.Int).SetUint64(depositGas-remainingGas), ps.txCtx.GasPrice)
		depositLog, err = depositGasFunc(ctx, sm, gasValue)
		if err != nil {
			return nil, nil, err
		}
	}

	if err := stateDB.CommitContracts(); err != nil {
		return nil, nil, errors.Wrap(err, "failed to commit contracts to underlying db")
	}
	receipt.AddLogs(stateDB.Logs()...).AddTransactionLogs(depositLog, burnLog)
	if receipt.Status == uint64(iotextypes.ReceiptStatus_Success) ||
		featureCtx.AddOutOfGasToTransactionLog && receipt.Status == uint64(iotextypes.ReceiptStatus_ErrCodeStoreOutOfGas) {
		receipt.AddTransactionLogs(stateDB.TransactionLogs()...)
	}
	stateDB.clear()

	if featureCtx.SetRevertMessageToReceipt && receipt.Status == uint64(iotextypes.ReceiptStatus_ErrExecutionReverted) && retval != nil && bytes.Equal(retval[:4], _revertSelector) {
		// in case of the execution revert error, parse the retVal and add to receipt
		data := retval[4:]
		msgLength := byteutil.BytesToUint64BigEndian(data[56:64])
		revertMsg := string(data[64 : 64+msgLength])
		receipt.SetExecutionRevertMsg(revertMsg)
	}
	log.S().Debugf("Receipt: %+v, %v", receipt, err)
	return retval, receipt, nil
}

// ReadContractStorage reads contract's storage
func ReadContractStorage(
	ctx context.Context,
	sm protocol.StateManager,
	contract address.Address,
	key []byte,
) ([]byte, error) {
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	ctx = protocol.WithFeatureCtx(protocol.WithBlockCtx(protocol.WithActionCtx(ctx,
		protocol.ActionCtx{
			ActionHash: hash.ZeroHash256,
		}),
		protocol.BlockCtx{
			BlockHeight: bcCtx.Tip.Height + 1,
		},
	))
	stateDB, err := prepareStateDB(ctx, sm)
	if err != nil {
		return nil, err
	}
	res := stateDB.GetState(common.BytesToAddress(contract.Bytes()), common.BytesToHash(key))
	return res[:], nil
}

func prepareStateDB(ctx context.Context, sm protocol.StateManager) (*StateDBAdapter, error) {
	actionCtx := protocol.MustGetActionCtx(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)
	featureCtx := protocol.MustGetFeatureCtx(ctx)
	opts := []StateDBAdapterOption{}
	if featureCtx.CreateLegacyNonceAccount {
		opts = append(opts, LegacyNonceAccountOption())
	}
	if !featureCtx.FixSortCacheContractsAndUsePendingNonce {
		opts = append(opts, DisableSortCachedContractsOption(), UseConfirmedNonceOption())
	}
	if featureCtx.NotFixTopicCopyBug {
		opts = append(opts, NotFixTopicCopyBugOption())
	}
	if featureCtx.AsyncContractTrie {
		opts = append(opts, AsyncContractTrieOption())
	}
	if featureCtx.FixSnapshotOrder {
		opts = append(opts, FixSnapshotOrderOption())
	}
	if featureCtx.RevertLog {
		opts = append(opts, RevertLogOption())
	}
	if !featureCtx.FixUnproductiveDelegates {
		opts = append(opts, NotCheckPutStateErrorOption())
	}
	if !featureCtx.CorrectGasRefund {
		opts = append(opts, ManualCorrectGasRefundOption())
	}

	return NewStateDBAdapter(
		sm,
		blkCtx.BlockHeight,
		actionCtx.ActionHash,
		opts...,
	)
}

func getChainConfig(g genesis.Blockchain, height uint64, id uint32) *params.ChainConfig {
	var chainConfig params.ChainConfig
	chainConfig.ConstantinopleBlock = new(big.Int).SetUint64(0) // Constantinople switch block (nil = no fork, 0 = already activated)
	chainConfig.BeringBlock = new(big.Int).SetUint64(g.BeringBlockHeight)
	// enable earlier Ethereum forks at Greenland
	chainConfig.GreenlandBlock = new(big.Int).SetUint64(g.GreenlandBlockHeight)
	// support chainid and enable Istanbul + MuirGlacier at Iceland
	chainConfig.IstanbulBlock = new(big.Int).SetUint64(g.IcelandBlockHeight)
	chainConfig.MuirGlacierBlock = new(big.Int).SetUint64(g.IcelandBlockHeight)
	if g.IsIceland(height) {
		chainConfig.ChainID = new(big.Int).SetUint64(uint64(id))
	}
	// enable Berlin and London
	chainConfig.BerlinBlock = new(big.Int).SetUint64(g.OkhotskBlockHeight)
	chainConfig.LondonBlock = new(big.Int).SetUint64(g.OkhotskBlockHeight)
	return &chainConfig
}

// Error in executeInEVM is a consensus issue
func executeInEVM(ctx context.Context, evmParams *Params, stateDB *StateDBAdapter, g genesis.Blockchain, gasLimit uint64, blockHeight uint64) ([]byte, uint64, uint64, string, iotextypes.ReceiptStatus, error) {
	remainingGas := evmParams.gas
	if err := securityDeposit(evmParams, stateDB, gasLimit); err != nil {
		log.L().Warn("unexpected error: not enough security deposit", zap.Error(err))
		return nil, 0, 0, action.EmptyAddress, iotextypes.ReceiptStatus_Failure, err
	}
	var (
		config     vm.Config
		accessList types.AccessList
	)
	if vmCfg, ok := protocol.GetVMConfigCtx(ctx); ok {
		config = vmCfg
	}
	chainConfig := getChainConfig(g, blockHeight, evmParams.evmNetworkID)
	evm := vm.NewEVM(evmParams.context, evmParams.txCtx, stateDB, chainConfig, config)
	if g.IsOkhotsk(blockHeight) {
		accessList = evmParams.accessList
	}
	intriGas, err := intrinsicGas(uint64(len(evmParams.data)), accessList)
	if err != nil {
		return nil, evmParams.gas, remainingGas, action.EmptyAddress, iotextypes.ReceiptStatus_Failure, err
	}
	if remainingGas < intriGas {
		return nil, evmParams.gas, remainingGas, action.EmptyAddress, iotextypes.ReceiptStatus_Failure, action.ErrInsufficientFunds
	}
	remainingGas -= intriGas

	// Set up the initial access list
	if rules := chainConfig.Rules(evm.Context.BlockNumber, false); rules.IsBerlin {
		stateDB.PrepareAccessList(evmParams.txCtx.Origin, evmParams.contract, vm.ActivePrecompiles(rules), evmParams.accessList)
	}
	var (
		contractRawAddress = action.EmptyAddress
		executor           = vm.AccountRef(evmParams.txCtx.Origin)
		london             = evm.ChainConfig().IsLondon(evm.Context.BlockNumber)
		ret                []byte
		evmErr             error
		refund             uint64
	)
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
		stateDB.SetNonce(evmParams.txCtx.Origin, stateDB.GetNonce(evmParams.txCtx.Origin)+1)
		// process contract
		ret, remainingGas, evmErr = evm.Call(executor, *evmParams.contract, evmParams.data, remainingGas, evmParams.amount)
	}
	if evmErr != nil {
		log.L().Debug("evm error", zap.Error(evmErr))
		// The only possible consensus-error would be if there wasn't
		// sufficient balance to make the transfer happen.
		// Should be a hard fork (Bering)
		if evmErr == vm.ErrInsufficientBalance && g.IsBering(blockHeight) {
			return nil, evmParams.gas, remainingGas, action.EmptyAddress, iotextypes.ReceiptStatus_Failure, evmErr
		}
	}
	if stateDB.Error() != nil {
		log.L().Debug("statedb error", zap.Error(stateDB.Error()))
	}
	if !london {
		// Before EIP-3529: refunds were capped to gasUsed / 2
		refund = (evmParams.gas - remainingGas) / params.RefundQuotient
	} else {
		// After EIP-3529: refunds are capped to gasUsed / 5
		refund = (evmParams.gas - remainingGas) / params.RefundQuotientEIP3529
	}
	// before London EVM activation (at Okhotsk height), in certain cases dynamicGas
	// has caused gas refund to change, which needs to be manually adjusted after
	// the tx is reverted. After Okhotsk height, it is fixed inside RevertToSnapshot()
	var (
		refundBeforeDynamicGas = evm.TxContext.RefundBeforeDynamicGas
		currentRefund          = stateDB.GetRefund()
		featureCtx             = protocol.MustGetFeatureCtx(ctx)
	)
	if evmErr != nil && !featureCtx.CorrectGasRefund && evm.TxContext.HitErrWriteProtection && refundBeforeDynamicGas != currentRefund {
		if refundBeforeDynamicGas > currentRefund {
			stateDB.AddRefund(refundBeforeDynamicGas - currentRefund)
		} else {
			stateDB.SubRefund(currentRefund - refundBeforeDynamicGas)
		}
	}
	if refund > stateDB.GetRefund() {
		refund = stateDB.GetRefund()
	}
	remainingGas += refund

	errCode := iotextypes.ReceiptStatus_Success
	if evmErr != nil {
		errCode = evmErrToErrStatusCode(evmErr, g, blockHeight)
		if errCode == iotextypes.ReceiptStatus_ErrUnknown {
			var addr string
			if evmParams.contract != nil {
				ioAddr, _ := address.FromBytes((*evmParams.contract)[:])
				addr = ioAddr.String()
			} else {
				addr = "contract creation"
			}
			log.L().Warn("evm internal error", zap.Error(evmErr),
				zap.String("address", addr),
				log.Hex("calldata", evmParams.data))
		}
	}
	return ret, evmParams.gas, remainingGas, contractRawAddress, errCode, nil
}

// evmErrToErrStatusCode returns ReceiptStatuscode which describes error type
func evmErrToErrStatusCode(evmErr error, g genesis.Blockchain, height uint64) iotextypes.ReceiptStatus {
	// specific error starting London
	if g.IsOkhotsk(height) {
		if evmErr == vm.ErrInvalidCode {
			return iotextypes.ReceiptStatus_ErrInvalidCode
		}
	}

	// specific error starting Jutland
	if g.IsJutland(height) {
		switch evmErr {
		case vm.ErrInsufficientBalance:
			return iotextypes.ReceiptStatus_ErrInsufficientBalance
		case vm.ErrInvalidJump:
			return iotextypes.ReceiptStatus_ErrInvalidJump
		case vm.ErrReturnDataOutOfBounds:
			return iotextypes.ReceiptStatus_ErrReturnDataOutOfBounds
		case vm.ErrGasUintOverflow:
			return iotextypes.ReceiptStatus_ErrGasUintOverflow
		}
	}

	// specific error starting Bering
	if g.IsBering(height) {
		switch evmErr {
		case vm.ErrOutOfGas:
			return iotextypes.ReceiptStatus_ErrOutOfGas
		case vm.ErrCodeStoreOutOfGas:
			return iotextypes.ReceiptStatus_ErrCodeStoreOutOfGas
		case vm.ErrDepth:
			return iotextypes.ReceiptStatus_ErrDepth
		case vm.ErrContractAddressCollision:
			return iotextypes.ReceiptStatus_ErrContractAddressCollision
		case vm.ErrExecutionReverted:
			return iotextypes.ReceiptStatus_ErrExecutionReverted
		case vm.ErrMaxCodeSizeExceeded:
			return iotextypes.ReceiptStatus_ErrMaxCodeSizeExceeded
		case vm.ErrWriteProtection:
			return iotextypes.ReceiptStatus_ErrWriteProtection
		default:
			// internal errors from go-ethereum are not directly accessible
			switch evmErr.Error() {
			case "no compatible interpreter":
				return iotextypes.ReceiptStatus_ErrNoCompatibleInterpreter
			default:
				return iotextypes.ReceiptStatus_ErrUnknown
			}
		}
	}
	// before Bering height, return one common failure
	return iotextypes.ReceiptStatus_Failure
}

// intrinsicGas returns the intrinsic gas of an execution
func intrinsicGas(size uint64, list types.AccessList) (uint64, error) {
	if action.ExecutionDataGas == 0 {
		panic("payload gas price cannot be zero")
	}

	var accessListGas uint64
	if len(list) > 0 {
		accessListGas = uint64(len(list)) * action.TxAccessListAddressGas
		accessListGas += uint64(list.StorageKeys()) * action.TxAccessListStorageKeyGas
	}
	if (math.MaxInt64-action.ExecutionBaseIntrinsicGas-accessListGas)/action.ExecutionDataGas < size {
		return 0, action.ErrInsufficientFunds
	}
	return size*action.ExecutionDataGas + action.ExecutionBaseIntrinsicGas + accessListGas, nil
}

// SimulateExecution simulates the execution in evm
func SimulateExecution(
	ctx context.Context,
	sm protocol.StateManager,
	caller address.Address,
	ex *action.Execution,
	getBlockHash GetBlockHash,
) ([]byte, *action.Receipt, error) {
	ctx, span := tracer.NewSpan(ctx, "evm.SimulateExecution")
	defer span.End()
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	g := genesis.MustExtractGenesisContext(ctx)
	ctx = protocol.WithActionCtx(
		ctx,
		protocol.ActionCtx{
			Caller:     caller,
			ActionHash: hash.Hash256b(byteutil.Must(proto.Marshal(ex.Proto()))),
		},
	)
	zeroAddr, err := address.FromString(address.ZeroAddress)
	if err != nil {
		return nil, nil, err
	}
	ctx = protocol.WithBlockCtx(
		ctx,
		protocol.BlockCtx{
			BlockHeight:    bcCtx.Tip.Height + 1,
			BlockTimeStamp: bcCtx.Tip.Timestamp.Add(g.BlockInterval),
			GasLimit:       g.BlockGasLimit,
			Producer:       zeroAddr,
		},
	)

	ctx = protocol.WithFeatureCtx(ctx)
	return ExecuteContract(
		ctx,
		sm,
		ex,
		getBlockHash,
		func(context.Context, protocol.StateManager, *big.Int) (*action.TransactionLog, error) {
			return nil, nil
		},
	)
}
