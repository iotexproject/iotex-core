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
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/blockchain/action"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/pkg/hash"
)

var (
	// ErrInsufficientBalanceForGas is the error that the balance in sender account is lower than gas
	ErrInsufficientBalanceForGas = errors.New("insufficient balance for gas")
	// ErrInconsistentNonce is the error that the nonce is different from sender's nonce
	ErrInconsistentNonce = errors.New("Nonce is not identical to sender nonce")
	// ErrOutOfGas is the error when running out of gas
	ErrOutOfGas = errors.New("Out of gas")
)

// Header represents the header of blockchain
type Header struct {
	ParentHash  common.Hash    `json:"parentHash"       gencodec:"required"`
	UncleHash   common.Hash    `json:"sha3Uncles"       gencodec:"required"`
	Coinbase    common.Address `json:"miner"            gencodec:"required"`
	Root        common.Hash    `json:"stateRoot"        gencodec:"required"`
	TxHash      common.Hash    `json:"transactionsRoot" gencodec:"required"`
	ReceiptHash common.Hash    `json:"receiptsRoot"     gencodec:"required"`
	// Bloom       Bloom          `json:"logsBloom"        gencodec:"required"`
	Difficulty *big.Int    `json:"difficulty"       gencodec:"required"`
	Number     *big.Int    `json:"number"           gencodec:"required"`
	GasLimit   uint64      `json:"gasLimit"         gencodec:"required"`
	GasUsed    uint64      `json:"gasUsed"          gencodec:"required"`
	Time       *big.Int    `json:"timestamp"        gencodec:"required"`
	Extra      []byte      `json:"extraData"        gencodec:"required"`
	MixDigest  common.Hash `json:"mixHash"          gencodec:"required"`
	// Nonce       BlockNonce     `json:"nonce"            gencodec:"required"`
}

var (
	// FailureStatus is the status that contract execution failed
	FailureStatus = uint64(0)
	// SuccessStatus is the status that contract execution success
	SuccessStatus = uint64(1)
)

// Receipt represents the result of a contract
type Receipt struct {
	Status          uint64
	Hash            hash.Hash32B
	GasConsumed     uint64
	ContractAddress string
}

// EVMStateDBAdapter represents the state db adapter for evm to access iotx blockchain
type EVMStateDBAdapter struct {
	bc Blockchain
}

// NewEVMStateDBAdapter creates a new state db with iotx blockchain
func NewEVMStateDBAdapter(bc Blockchain) *EVMStateDBAdapter {
	return &EVMStateDBAdapter{
		bc,
	}
}

// CreateAccount creates an account in iotx blockchain
func (stateDB *EVMStateDBAdapter) CreateAccount(common.Address) {
	logger.Error().Msg("CreateAccount is not implemented")
}

// SubBalance subtracts balance from account
func (stateDB *EVMStateDBAdapter) SubBalance(common.Address, *big.Int) {
	logger.Error().Msg("SubBalance is not implemented")
}

// AddBalance adds balance to account
func (stateDB *EVMStateDBAdapter) AddBalance(common.Address, *big.Int) {
	logger.Error().Msg("AddBalance is not implemented")
}

// GetBalance gets the balance of account
func (stateDB *EVMStateDBAdapter) GetBalance(evmAddr common.Address) *big.Int {
	addr, err := iotxaddress.GetAddressByHash(iotxaddress.IsTestnet, iotxaddress.ChainID, evmAddr.Bytes())
	if err != nil {
		logger.Error().Err(err).Msgf("Failed to generate address for %s", evmAddr)
		return nil
	}
	balance, err := stateDB.bc.Balance(addr.RawAddress)
	if err != nil {
		logger.Error().Err(err).Msgf("Failed to get balance for %s", addr.RawAddress)
		return nil
	}
	return balance
}

// GetNonce gets the nonce of account
func (stateDB *EVMStateDBAdapter) GetNonce(evmAddr common.Address) uint64 {
	addr, err := iotxaddress.GetAddressByHash(iotxaddress.IsTestnet, iotxaddress.ChainID, evmAddr.Bytes())
	if err != nil {
		logger.Error().Err(err).Msgf("Failed to generate address for %s", evmAddr)
		return 0
	}
	nonce, err := stateDB.bc.Nonce(addr.RawAddress)
	if err != nil {
		logger.Error().Err(err).Msgf("Failed to get nonce for %s", addr.RawAddress)
		return 0
	}
	return nonce
}

// SetNonce sets the nonce of account
func (stateDB *EVMStateDBAdapter) SetNonce(common.Address, uint64) {
	logger.Error().Msg("SetNonce is not implemented")
}

// GetCodeHash gets the code hash of account
func (stateDB *EVMStateDBAdapter) GetCodeHash(evmAddr common.Address) common.Hash {
	addr, err := iotxaddress.GetAddressByHash(iotxaddress.IsTestnet, iotxaddress.ChainID, evmAddr.Bytes())
	if err != nil {
		logger.Error().Err(err).Msgf("Failed to generate address for %s", evmAddr)
		return common.Hash{}
	}
	state, err := stateDB.bc.StateByAddr(addr.RawAddress)
	if err != nil {
		logger.Error().Err(err).Msgf("Failed to get code for %s", addr.RawAddress)
		return common.Hash{}
	}
	return common.BytesToHash(state.CodeHash)
}

// GetCode gets the code saved in hash
func (stateDB *EVMStateDBAdapter) GetCode(evmAddr common.Address) []byte {
	logger.Error().Msg("GetCode is not implemented")
	return nil
}

// SetCode sets the code saved in hash
func (stateDB *EVMStateDBAdapter) SetCode(evmAddr common.Address, code []byte) {
	logger.Error().Msg("SetCode is not implemented")
}

// GetCodeSize gets the code size saved in hash
func (stateDB *EVMStateDBAdapter) GetCodeSize(evmAddr common.Address) int {
	code := stateDB.GetCode(evmAddr)
	if code == nil {
		return 0
	}
	return len(code)
}

// AddRefund adds refund
func (stateDB *EVMStateDBAdapter) AddRefund(uint64) {
	logger.Error().Msg("AddRefund is not implemented")
}

// GetRefund gets refund
func (stateDB *EVMStateDBAdapter) GetRefund() uint64 {
	logger.Error().Msg("GetRefund is not implemented")
	return 0
}

// GetState gets state
func (stateDB *EVMStateDBAdapter) GetState(common.Address, common.Hash) common.Hash {
	logger.Error().Msg("GetState is not implemented")
	return common.Hash{}
}

// SetState sets state
func (stateDB *EVMStateDBAdapter) SetState(common.Address, common.Hash, common.Hash) {
	logger.Error().Msg("SetState is not implemented")
}

// Suicide kills the contract
func (stateDB *EVMStateDBAdapter) Suicide(common.Address) bool {
	logger.Error().Msg("Suicide is not implemented")
	return false
}

// HasSuicided returns whether the contract has been killed
func (stateDB *EVMStateDBAdapter) HasSuicided(common.Address) bool {
	logger.Error().Msg("HasSuicide is not implemented")
	return false
}

// Exist exits the contract
func (stateDB *EVMStateDBAdapter) Exist(common.Address) bool {
	logger.Error().Msg("Exist is not implemented")
	return false
}

// Empty empties the contract
func (stateDB *EVMStateDBAdapter) Empty(common.Address) bool {
	logger.Error().Msg("Empty is not implemented")
	return false
}

// RevertToSnapshot reverts the state factory to snapshot
func (stateDB *EVMStateDBAdapter) RevertToSnapshot(int) {
	logger.Error().Msg("RevertToSnapshot is not implemented")
}

// Snapshot returns the snapshot id
func (stateDB *EVMStateDBAdapter) Snapshot() int {
	logger.Error().Msg("Snapshot is not implemented")
	return 0
}

// AddLog adds log
// TODO (zhi) add this function after extending the EVM
func (stateDB *EVMStateDBAdapter) AddLog(*types.Log) {
	logger.Error().Msg("AddLog is not implemented")
}

// AddPreimage adds the preimage
func (stateDB *EVMStateDBAdapter) AddPreimage(common.Hash, []byte) {
	logger.Error().Msg("AddPreimage is not implemented")
}

// ForEachStorage loops each storage
func (stateDB *EVMStateDBAdapter) ForEachStorage(common.Address, func(common.Hash, common.Hash) bool) {
	logger.Error().Msg("ForEachStorage is not implemented")
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

// NewEVMContext creates a new context for use in the EVM.
func NewEVMContext(senderAddr common.Address, gasPrice *big.Int, header *Header, bc Blockchain, author *common.Address) vm.Context {
	// If we don't have an explicit author (i.e. not mining), extract from the header
	/*
		var beneficiary common.Address
		if author == nil {
			beneficiary, _ = chain.Engine().Author(header) // Ignore error, we're past header validation
		} else {
			beneficiary = *author
		}
	*/
	return vm.Context{
		CanTransfer: CanTransfer,
		Transfer:    MakeTransfer,
		GetHash:     GetHashFn(header, bc),
		Origin:      senderAddr,
		Coinbase:    *author,
		BlockNumber: new(big.Int).Set(header.Number),
		Time:        new(big.Int).Set(header.Time),
		Difficulty:  new(big.Int).Set(header.Difficulty),
		GasLimit:    header.GasLimit,
		GasPrice:    new(big.Int).Set(gasPrice),
	}
}

// GetHashFn returns a GetHashFunc which retrieves header hashes by number
func GetHashFn(ref *Header, bc Blockchain) func(n uint64) common.Hash {
	return func(n uint64) common.Hash {
		tipHeight, err := bc.TipHeight()
		if err != nil {
			hash, err := bc.GetHashByHeight(tipHeight - n)
			if err != nil {
				return common.BytesToHash(hash[:])
			}
		}

		return common.Hash{}
	}
}

func securityDeposit(stateDB vm.StateDB, checkNonce bool, senderAddr common.Address, gas uint64, gasPrice *big.Int, nonce uint64) (uint64, error) {
	if checkNonce {
		senderNonce := stateDB.GetNonce(senderAddr)
		if senderNonce != nonce {
			return 0, ErrInconsistentNonce
		}
	}
	maxGasValue := new(big.Int).Mul(new(big.Int).SetUint64(gas), gasPrice)
	if stateDB.GetBalance(senderAddr).Cmp(maxGasValue) < 0 {
		return 0, ErrInsufficientBalanceForGas
	}
	stateDB.SubBalance(senderAddr, maxGasValue)
	return gas, nil
}

// ExtractContractParams extracts contractor parameters from transfer payload
func ExtractContractParams(tsf *action.Transfer) ([]byte, uint64, *big.Int, bool, error) {
	payload := tsf.Payload
	// TODO (zhi) decode payload
	return payload, 0, new(big.Int), false, nil
}

// ProcessTransfer processes a transfer which contains a contract
func ProcessTransfer(tsf *action.Transfer, bc Blockchain) (*Receipt, error) {
	if !tsf.IsContract() {
		return nil, nil
	}
	receipt := &Receipt{
		GasConsumed: 0,
		Hash:        tsf.Hash(),
		Status:      FailureStatus,
	}
	senderHash, err := iotxaddress.GetPubkeyHash(tsf.Sender)
	if err != nil {
		return receipt, err
	}
	senderAddr := common.BytesToAddress(senderHash)
	transferAmount := tsf.Amount
	data, gas, gasPrice, checkNonce, err := ExtractContractParams(tsf)
	if err != nil {
		return receipt, err
	}
	var chainConfig params.ChainConfig
	var config vm.Config
	// var header Header
	stateDB := NewEVMStateDBAdapter(bc)
	context := NewEVMContext(senderAddr, gasPrice, nil, bc, nil)
	evm := vm.NewEVM(context, stateDB, &chainConfig, config)

	// TODO (zhi) There should be a global parameter of the total gas limit in a block
	securityDepositGas, err := securityDeposit(stateDB, checkNonce, senderAddr, gas, gasPrice, tsf.Nonce)
	if err != nil {
		return nil, err
	}
	remainingGas := securityDepositGas
	intrinsicGas := uint64(10) // Intrinsic Gas
	if remainingGas < intrinsicGas {
		receipt.Status = uint64(0)
		receipt.GasConsumed = securityDepositGas - remainingGas
		return receipt, ErrOutOfGas
	}
	remainingGas -= intrinsicGas
	sender := vm.AccountRef(senderAddr)
	if tsf.Recipient == action.EmptyAddress {
		// create contract
		_, _, remainingGas, err = evm.Create(sender, data, remainingGas, transferAmount)
		if err == nil {
			fromAddr, err := iotxaddress.GetAddressByHash(iotxaddress.IsTestnet, iotxaddress.ChainID, senderHash)
			if err == nil {
				receipt.ContractAddress, err = iotxaddress.CreateContractAddress(fromAddr.RawAddress, tsf.Nonce)
			}
		}
	} else {
		recipientHash, err := iotxaddress.GetPubkeyHash(tsf.Recipient)
		if err != nil {
			receipt.Status = FailureStatus
			receipt.GasConsumed = securityDepositGas - remainingGas
			return receipt, err
		}
		recipientAddr := common.BytesToAddress(recipientHash)
		// process contract
		_, remainingGas, err = evm.Call(sender, recipientAddr, data, remainingGas, transferAmount)
	}
	receipt.GasConsumed = securityDepositGas - remainingGas
	remainingValue := new(big.Int).Mul(new(big.Int).SetUint64(remainingGas), gasPrice)
	stateDB.AddBalance(senderAddr, remainingValue)
	if err != nil {
		receipt.Status = FailureStatus
		if err == vm.ErrInsufficientBalance {
			return receipt, err
		}
	}
	// TODO (zhi) figure out what the following function does
	// stateDB.Finalise(true)
	receipt.Status = SuccessStatus
	/*
		receipt.Logs = stateDB.GetLogs(tsf.Hash())
	*/
	return receipt, nil
}
