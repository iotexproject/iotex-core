// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"math/big"

	"github.com/CoderZhi/go-ethereum/common"
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
	// ErrInsufficientBalanceForGas is the error that the balance in sender account is lower than gas
	ErrInsufficientBalanceForGas = errors.New("Insufficient balance for gas")
	// ErrInconsistentNonce is the error that the nonce is different from sender's nonce
	ErrInconsistentNonce = errors.New("Nonce is not identical to sender nonce")
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
	context          vm.Context
	nonce            uint64
	senderRawAddress string
	amount           *big.Int
	recipient        *common.Address
	gas              uint64
	data             []byte
}

// NewEVMParams creates a new context for use in the EVM.
func NewEVMParams(blk *Block, tsf *action.Transfer, stateDB *EVMStateDBAdapter) (*EVMParams, error) {
	// If we don't have an explicit author (i.e. not mining), extract from the header
	/*
		var beneficiary common.Address
		if author == nil {
			beneficiary, _ = chain.Engine().Author(header) // Ignore error, we're past header validation
		} else {
			beneficiary = *author
		}
	*/
	senderHash, err := iotxaddress.GetPubkeyHash(tsf.Sender)
	if err != nil {
		return nil, err
	}
	senderAddr := common.BytesToAddress(senderHash)
	senderIoTXAddress, err := iotxaddress.GetAddressByHash(iotxaddress.IsTestnet, iotxaddress.ChainID, senderHash)
	if err != nil {
		return nil, err
	}
	var recipientAddr common.Address
	if tsf.Recipient != action.EmptyAddress {
		recipientHash, err := iotxaddress.GetPubkeyHash(tsf.Recipient)
		if err != nil {
			return nil, err
		}
		recipientAddr = common.BytesToAddress(recipientHash)
	}
	producerHash := keypair.HashPubKey(blk.Header.Pubkey)
	producer := common.BytesToAddress(producerHash)
	context := vm.Context{
		CanTransfer: CanTransfer,
		Transfer:    MakeTransfer,
		GetHash:     GetHashFn(stateDB),
		Origin:      senderAddr,
		Coinbase:    producer,
		BlockNumber: new(big.Int).SetUint64(blk.Height()),
		Time:        new(big.Int).SetInt64(blk.Header.Timestamp().Unix()),
		Difficulty:  new(big.Int).SetUint64(uint64(50)),
		GasLimit:    GasLimit,
		GasPrice:    new(big.Int).SetUint64(uint64(10)),
	}

	return &EVMParams{
		context,
		tsf.Nonce,
		senderIoTXAddress.RawAddress,
		tsf.Amount,
		&recipientAddr,
		uint64(100), // gas
		tsf.Payload, // data
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
	senderNonce := stateDB.GetNonce(context.context.Origin)
	if senderNonce != context.nonce {
		return ErrInconsistentNonce
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

// ProcessBlockContracts process the contracts in a block
func ProcessBlockContracts(blk *Block, bc Blockchain) {
	gasLimit := GasLimit
	for _, tsf := range blk.Transfers {
		ProcessContract(blk, tsf, bc, &gasLimit)
	}
}

// ProcessContract processes a transfer which contains a contract
func ProcessContract(blk *Block, tsf *action.Transfer, bc Blockchain, gasLimit *uint64) (*Receipt, error) {
	if !tsf.IsContract() {
		return nil, nil
	}
	stateDB := NewEVMStateDBAdapter(bc)
	ps, err := NewEVMParams(blk, tsf, stateDB)
	if err != nil {
		return nil, err
	}
	remainingGas, contractAddress, err := executeInEVM(ps, stateDB, gasLimit)
	receipt := &Receipt{
		GasConsumed:     ps.gas - remainingGas,
		Hash:            tsf.Hash(),
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
	return receipt, err
}

func executeInEVM(evmParams *EVMParams, stateDB *EVMStateDBAdapter, gasLimit *uint64) (uint64, string, error) {
	remainingGas := evmParams.gas
	if err := securityDeposit(evmParams, stateDB, gasLimit); err != nil {
		return 0, action.EmptyAddress, err
	}
	var chainConfig params.ChainConfig
	var config vm.Config
	evm := vm.NewEVM(evmParams.context, stateDB, &chainConfig, config)
	intrinsicGas := uint64(10) // Intrinsic Gas
	if remainingGas < intrinsicGas {
		return remainingGas, action.EmptyAddress, ErrOutOfGas
	}
	var err error
	contractAddress := action.EmptyAddress
	remainingGas -= intrinsicGas
	sender := vm.AccountRef(evmParams.context.Origin)
	if evmParams.recipient == nil {
		// create contract
		_, _, remainingGas, err := evm.Create(sender, evmParams.data, remainingGas, evmParams.amount)
		if err != nil {
			return remainingGas, action.EmptyAddress, err
		}

		contractAddress, err = iotxaddress.CreateContractAddress(evmParams.senderRawAddress, evmParams.nonce)
		if err != nil {
			return remainingGas, action.EmptyAddress, err
		}
	} else {
		// process contract
		_, remainingGas, err = evm.Call(sender, *evmParams.recipient, evmParams.data, remainingGas, evmParams.amount)
	}
	if err != nil {
		if err == vm.ErrInsufficientBalance {
			return remainingGas, action.EmptyAddress, err
		}
	} else {
		// TODO (zhi) figure out what the following function does
		// stateDB.Finalise(true)
	}
	/*
		receipt.Logs = stateDB.GetLogs(tsf.Hash())
	*/
	return remainingGas, contractAddress, nil
}
