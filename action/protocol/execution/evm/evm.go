// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package evm

import (
	"bytes"
	"context"
	"math"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/pkg/tracer"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
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

type (
	// GetBlockHash gets block hash by height
	GetBlockHash func(uint64) (hash.Hash256, error)

	// GetBlockTime gets block time by height
	GetBlockTime func(uint64) (time.Time, error)
)

// CanTransfer checks whether the from account has enough balance
func CanTransfer(db vm.StateDB, fromHash common.Address, balance *uint256.Int) bool {
	return db.GetBalance(fromHash).Cmp(balance) >= 0
}

// MakeTransfer transfers account
func MakeTransfer(db vm.StateDB, fromHash, toHash common.Address, amount *uint256.Int) {
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
		context     vm.BlockContext
		txCtx       vm.TxContext
		nonce       uint64
		amount      *big.Int
		contract    *common.Address
		gas         uint64
		data        []byte
		accessList  types.AccessList
		evmConfig   vm.Config
		chainConfig *params.ChainConfig
		genesis     genesis.Blockchain
		blkCtx      protocol.BlockCtx
		featureCtx  protocol.FeatureCtx
		actionCtx   protocol.ActionCtx
		helperCtx   HelperContext
	}

	stateDB interface {
		vm.StateDB

		CommitContracts() error
		Logs() []*action.Log
		TransactionLogs() []*action.TransactionLog
		clear()
		Error() error
	}
)

// newParams creates a new context for use in the EVM.
func newParams(
	ctx context.Context,
	execution action.TxData,
) (*Params, error) {
	var (
		actionCtx    = protocol.MustGetActionCtx(ctx)
		blkCtx       = protocol.MustGetBlockCtx(ctx)
		featureCtx   = protocol.MustGetFeatureCtx(ctx)
		g            = genesis.MustExtractGenesisContext(ctx)
		helperCtx    = mustGetHelperCtx(ctx)
		evmNetworkID = protocol.MustGetBlockchainCtx(ctx).EvmNetworkID
		executorAddr = common.BytesToAddress(actionCtx.Caller.Bytes())
		getBlockHash = helperCtx.GetBlockHash

		vmConfig  vm.Config
		getHashFn vm.GetHashFunc
	)

	gasLimit := execution.Gas()
	// Reset gas limit to the system wide action gas limit cap if it's greater than it
	if blkCtx.BlockHeight > 0 && featureCtx.SystemWideActionGasLimit && gasLimit > _preAleutianActionGasLimit {
		gasLimit = _preAleutianActionGasLimit
	}

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
			hash, err := getBlockHash(blkCtx.BlockHeight - (n + 1))
			if err == nil {
				return common.BytesToHash(hash[:])
			}
			return common.Hash{}
		}
	default:
		getHashFn = func(n uint64) common.Hash {
			hash, err := getBlockHash(blkCtx.BlockHeight - n)
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
		Time:        new(big.Int).SetInt64(blkCtx.BlockTimeStamp.Unix()).Uint64(),
		Difficulty:  new(big.Int).SetUint64(uint64(50)),
		BaseFee:     new(big.Int),
	}
	if g.IsSumatra(blkCtx.BlockHeight) {
		// Random opcode (EIP-4399) is not supported
		context.Random = &common.Hash{}
	}
	if g.IsVanuatu(blkCtx.BlockHeight) {
		// enable BLOBBASEFEE opcode
		context.BlobBaseFee = protocol.CalcBlobFee(blkCtx.ExcessBlobGas)
		// enable BASEFEE opcode
		if blkCtx.BaseFee != nil {
			context.BaseFee = new(big.Int).Set(blkCtx.BaseFee)
		}
	}

	if vmCfg, ok := protocol.GetVMConfigCtx(ctx); ok {
		vmConfig = vmCfg
	}
	chainConfig, err := getChainConfig(g.Blockchain, blkCtx.BlockHeight, evmNetworkID, func(height uint64) (*time.Time, error) {
		return blockHeightToTime(ctx, height)
	})
	if err != nil {
		return nil, err
	}
	vmTxCtx := vm.TxContext{
		Origin:   executorAddr,
		GasPrice: execution.GasPrice(),
	}
	if g.IsVanuatu(blkCtx.BlockHeight) {
		// enable BLOBHASH opcode
		vmTxCtx.BlobHashes = execution.BlobHashes()
		vmTxCtx.BlobFeeCap = execution.BlobGasFeeCap()
	}
	return &Params{
		context,
		vmTxCtx,
		execution.Nonce(),
		execution.Value(),
		execution.To(),
		gasLimit,
		execution.Data(),
		execution.AccessList(),
		vmConfig,
		chainConfig,
		g.Blockchain,
		blkCtx,
		featureCtx,
		actionCtx,
		helperCtx,
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
	gasConsumed := uint256.MustFromBig(new(big.Int).Mul(new(big.Int).SetUint64(ps.gas), ps.txCtx.GasPrice))
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
	execution action.TxData,
) ([]byte, *action.Receipt, error) {
	ctx, span := tracer.NewSpan(ctx, "evm.ExecuteContract")
	defer span.End()

	stateDB, err := prepareStateDB(ctx, sm)
	if err != nil {
		return nil, nil, err
	}
	ps, err := newParams(ctx, execution)
	if err != nil {
		return nil, nil, err
	}
	retval, depositGas, remainingGas, contractAddress, statusCode, err := executeInEVM(ctx, ps, stateDB)
	if err != nil {
		return nil, nil, err
	}
	receipt := &action.Receipt{
		GasConsumed:       ps.gas - remainingGas,
		BlockHeight:       ps.blkCtx.BlockHeight,
		ActionHash:        ps.actionCtx.ActionHash,
		ContractAddress:   contractAddress,
		Status:            uint64(statusCode),
		EffectiveGasPrice: protocol.EffectiveGasPrice(ctx, execution),
	}
	var (
		depositLog  []*action.TransactionLog
		burnLog     *action.TransactionLog
		consumedGas = depositGas - remainingGas
	)
	if ps.featureCtx.FixDoubleChargeGas {
		// Refund all deposit and, actual gas fee will be subtracted when depositing gas fee to the rewarding protocol
		stateDB.AddBalance(ps.txCtx.Origin, uint256.MustFromBig(big.NewInt(0).Mul(big.NewInt(0).SetUint64(depositGas), ps.txCtx.GasPrice)))
	} else {
		if remainingGas > 0 {
			remainingValue := new(big.Int).Mul(new(big.Int).SetUint64(remainingGas), ps.txCtx.GasPrice)
			stateDB.AddBalance(ps.txCtx.Origin, uint256.MustFromBig(remainingValue))
		}
		if consumedGas > 0 {
			burnLog = &action.TransactionLog{
				Type:      iotextypes.TransactionLogType_GAS_FEE,
				Sender:    ps.actionCtx.Caller.String(),
				Recipient: "", // burned
				Amount:    new(big.Int).Mul(new(big.Int).SetUint64(consumedGas), ps.txCtx.GasPrice),
			}
		}
	}
	if consumedGas > 0 {
		priorityFee, baseFee, err := protocol.SplitGas(ctx, execution, consumedGas)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "failed to split gas")
		}
		depositLog, err = ps.helperCtx.DepositGasFunc(ctx, sm, baseFee, protocol.PriorityFeeOption(priorityFee))
		if err != nil {
			return nil, nil, err
		}
	}

	if err := stateDB.CommitContracts(); err != nil {
		return nil, nil, errors.Wrap(err, "failed to commit contracts to underlying db")
	}
	receipt.AddLogs(stateDB.Logs()...).AddTransactionLogs(depositLog...)
	receipt.AddTransactionLogs(burnLog)
	if receipt.Status == uint64(iotextypes.ReceiptStatus_Success) ||
		ps.featureCtx.AddOutOfGasToTransactionLog && receipt.Status == uint64(iotextypes.ReceiptStatus_ErrCodeStoreOutOfGas) {
		receipt.AddTransactionLogs(stateDB.TransactionLogs()...)
	}
	stateDB.clear()

	if ps.featureCtx.SetRevertMessageToReceipt && receipt.Status == uint64(iotextypes.ReceiptStatus_ErrExecutionReverted) && retval != nil && bytes.Equal(retval[:4], _revertSelector) {
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
	var (
		actionCtx  = protocol.MustGetActionCtx(ctx)
		blkCtx     = protocol.MustGetBlockCtx(ctx)
		featureCtx = protocol.MustGetFeatureCtx(ctx)
		opts       = []StateDBAdapterOption{}
	)
	if featureCtx.CreateLegacyNonceAccount {
		opts = append(opts, LegacyNonceAccountOption())
	}
	if !featureCtx.FixSortCacheContractsAndUsePendingNonce {
		opts = append(opts, DisableSortCachedContractsOption(), UseConfirmedNonceOption())
	}
	// Before featureCtx.RefactorFreshAccountConversion is activated,
	// the type of a legacy fresh account is always 1
	if featureCtx.RefactorFreshAccountConversion {
		opts = append(opts, ZeroNonceForFreshAccountOption())
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
	if !featureCtx.CorrectGasRefund {
		opts = append(opts, ManualCorrectGasRefundOption())
	}
	if featureCtx.SuicideTxLogMismatchPanic {
		opts = append(opts, SuicideTxLogMismatchPanicOption())
	}
	if featureCtx.PanicUnrecoverableError {
		opts = append(opts, PanicUnrecoverableErrorOption())
	}
	if featureCtx.EnableCancunEVM {
		opts = append(opts, EnableCancunEVMOption())
	}
	if featureCtx.FixRevertSnapshot || actionCtx.ReadOnly {
		opts = append(opts, FixRevertSnapshotOption())
		opts = append(opts, WithContext(ctx))
	}
	return NewStateDBAdapter(
		sm,
		blkCtx.BlockHeight,
		actionCtx.ActionHash,
		opts...,
	)
}

func getChainConfig(g genesis.Blockchain, height uint64, id uint32, getBlockTime func(uint64) (*time.Time, error)) (*params.ChainConfig, error) {
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
	// enable Berlin and London at Okhotsk
	chainConfig.BerlinBlock = new(big.Int).SetUint64(g.OkhotskBlockHeight)
	chainConfig.LondonBlock = new(big.Int).SetUint64(g.OkhotskBlockHeight)
	// enable ArrowGlacier, GrayGlacier at Redsea
	chainConfig.ArrowGlacierBlock = new(big.Int).SetUint64(g.RedseaBlockHeight)
	chainConfig.GrayGlacierBlock = new(big.Int).SetUint64(g.RedseaBlockHeight)
	// enable Merge, Shanghai at Sumatra
	chainConfig.MergeNetsplitBlock = new(big.Int).SetUint64(g.SumatraBlockHeight)
	// Starting Shanghai, fork scheduling on Ethereum was switched from blocks to timestamps
	sumatraTime, err := getBlockTime(g.SumatraBlockHeight)
	if err != nil {
		return nil, err
	} else if sumatraTime != nil {
		sumatraTimestamp := (uint64)(sumatraTime.Unix())
		chainConfig.ShanghaiTime = &sumatraTimestamp
	}
	// enable Cancun at Vanuatu
	cancunTime, err := getBlockTime(g.VanuatuBlockHeight)
	if err != nil {
		return nil, err
	} else if cancunTime != nil {
		cancunTimestamp := (uint64)(cancunTime.Unix())
		chainConfig.CancunTime = &cancunTimestamp
	}
	return &chainConfig, nil
}

// blockHeightToTime returns the block time by height
// if height is greater than current block height, return nil
// if height is equal to current block height, return current block time
// otherwise, return the block time by height from the blockchain
func blockHeightToTime(ctx context.Context, height uint64) (*time.Time, error) {
	blkCtx := protocol.MustGetBlockCtx(ctx)
	if height > blkCtx.BlockHeight {
		return nil, nil
	}
	if height == blkCtx.BlockHeight {
		return &blkCtx.BlockTimeStamp, nil
	}
	t, err := mustGetHelperCtx(ctx).GetBlockTime(height)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// Error in executeInEVM is a consensus issue
func executeInEVM(ctx context.Context, evmParams *Params, stateDB stateDB) ([]byte, uint64, uint64, string, iotextypes.ReceiptStatus, error) {
	var (
		gasLimit     = evmParams.blkCtx.GasLimit
		blockHeight  = evmParams.blkCtx.BlockHeight
		g            = evmParams.genesis
		remainingGas = evmParams.gas
		chainConfig  = evmParams.chainConfig
	)
	if err := securityDeposit(evmParams, stateDB, gasLimit); err != nil {
		log.T(ctx).Warn("unexpected error: not enough security deposit", zap.Error(err))
		return nil, 0, 0, action.EmptyAddress, iotextypes.ReceiptStatus_Failure, err
	}
	var (
		accessList types.AccessList
	)
	evm := vm.NewEVM(evmParams.context, evmParams.txCtx, stateDB, chainConfig, evmParams.evmConfig)
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
	rules := chainConfig.Rules(evm.Context.BlockNumber, g.IsSumatra(evmParams.blkCtx.BlockHeight), evmParams.context.Time)
	if rules.IsBerlin {
		stateDB.Prepare(rules, evmParams.txCtx.Origin, evmParams.context.Coinbase, evmParams.contract, vm.ActivePrecompiles(rules), evmParams.accessList)
	}
	var (
		contractRawAddress = action.EmptyAddress
		executor           = vm.AccountRef(evmParams.txCtx.Origin)
		ret                []byte
		evmErr             error
		refund             uint64
		amount             = uint256.MustFromBig(evmParams.amount)
	)
	if evmParams.contract == nil {
		// create contract
		var evmContractAddress common.Address
		_, evmContractAddress, remainingGas, evmErr = evm.Create(executor, evmParams.data, remainingGas, amount)
		log.T(ctx).Debug("evm Create.", log.Hex("addrHash", evmContractAddress[:]))
		if evmErr == nil {
			if contractAddress, err := address.FromBytes(evmContractAddress.Bytes()); err == nil {
				contractRawAddress = contractAddress.String()
			}
		}
	} else {
		stateDB.SetNonce(evmParams.txCtx.Origin, stateDB.GetNonce(evmParams.txCtx.Origin)+1)
		// process contract
		ret, remainingGas, evmErr = evm.Call(executor, *evmParams.contract, evmParams.data, remainingGas, amount)
	}
	if evmErr != nil {
		log.T(ctx).Debug("evm error", zap.Error(evmErr))
		// The only possible consensus-error would be if there wasn't
		// sufficient balance to make the transfer happen.
		// Should be a hard fork (Bering)
		if evmErr == vm.ErrInsufficientBalance && g.IsBering(blockHeight) {
			return nil, evmParams.gas, remainingGas, action.EmptyAddress, iotextypes.ReceiptStatus_Failure, evmErr
		}
	}
	if stateDB.Error() != nil {
		log.T(ctx).Debug("statedb error", zap.Error(stateDB.Error()))
	}
	if !rules.IsLondon {
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
		deltaRefundByDynamicGas = evm.DeltaRefundByDynamicGas
		featureCtx              = evmParams.featureCtx
	)
	if !featureCtx.CorrectGasRefund && deltaRefundByDynamicGas != 0 {
		if deltaRefundByDynamicGas > 0 {
			stateDB.SubRefund(uint64(deltaRefundByDynamicGas))
		} else {
			stateDB.AddRefund(uint64(-deltaRefundByDynamicGas))
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
			log.T(ctx).Warn("evm internal error", zap.Error(evmErr),
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
	ex action.TxDataForSimulation,
	opts ...protocol.SimulateOption,
) ([]byte, *action.Receipt, error) {
	ctx, span := tracer.NewSpan(ctx, "evm.SimulateExecution")
	defer span.End()
	if err := ex.SanityCheck(); err != nil {
		return nil, nil, err
	}
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	g := genesis.MustExtractGenesisContext(ctx)
	ctx = protocol.WithActionCtx(
		ctx,
		protocol.ActionCtx{
			Caller:     caller,
			ActionHash: hash.Hash256b(byteutil.Must(proto.Marshal(ex.Proto()))),
			ReadOnly:   true,
		},
	)
	zeroAddr, err := address.FromString(address.ZeroAddress)
	if err != nil {
		return nil, nil, err
	}
	cfg := &protocol.SimulateOptionConfig{}
	for _, opt := range opts {
		opt(cfg)
	}
	if cfg.PreOpt != nil {
		if err := cfg.PreOpt(sm); err != nil {
			return nil, nil, err
		}
	}
	ctx = protocol.WithFeatureCtx(protocol.WithBlockCtx(
		ctx,
		protocol.BlockCtx{
			BlockHeight:    bcCtx.Tip.Height + 1,
			BlockTimeStamp: bcCtx.Tip.Timestamp.Add(g.BlockInterval),
			GasLimit:       g.BlockGasLimitByHeight(bcCtx.Tip.Height + 1),
			Producer:       zeroAddr,
			BaseFee:        protocol.CalcBaseFee(g.Blockchain, &bcCtx.Tip),
			ExcessBlobGas:  protocol.CalcExcessBlobGas(bcCtx.Tip.ExcessBlobGas, bcCtx.Tip.BlobGasUsed),
		},
	))
	return ExecuteContract(ctx, sm, ex)
}
