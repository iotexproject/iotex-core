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
	"github.com/iotexproject/iotex-core-internal/iotxaddress"
	"github.com/iotexproject/iotex-core-internal/logger"
	"github.com/iotexproject/iotex-core-internal/pkg/hash"
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

// Message represents a message sent to a contract.
type Message interface {
	From() common.Address
	//FromFrontier() (common.Address, error)
	To() *common.Address

	GasPrice() *big.Int
	Gas() uint64
	Value() *big.Int

	Nonce() uint64
	CheckNonce() bool
	Data() []byte
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
func (stateDB *EVMStateDBAdapter) GetCodeHash(common.Address) hash.Hash32B {
	logger.Error().Msg("GetCodeHash is not implemented")
	return hash.ZeroHash32B
}

// GetCode gets the code saved in hash
func (stateDB *EVMStateDBAdapter) GetCode(evmAddr common.Address) []byte {
	addr, err := iotxaddress.GetAddressByHash(iotxaddress.IsTestnet, iotxaddress.ChainID, evmAddr.Bytes())
	if err != nil {
		logger.Error().Err(err).Msgf("Failed to generate address for %s", evmAddr)
		return nil
	}
	state, err := stateDB.bc.StateByAddr(addr.RawAddress)
	if err != nil {
		logger.Error().Err(err).Msgf("Failed to get code for %s", addr.RawAddress)
		return nil
	}
	return state.CodeHash
}

// SetCode sets the code saved in hash
func (stateDB *EVMStateDBAdapter) SetCode(evmAddr common.Address, code []byte) {
	addr, err := iotxaddress.GetAddressByHash(iotxaddress.IsTestnet, iotxaddress.ChainID, evmAddr.Bytes())
	if err != nil {
		logger.Error().Err(err).Msgf("Failed to generate address for %s", evmAddr)
		return
	}
	state, err := stateDB.bc.StateByAddr(addr.RawAddress)
	if err != nil {
		logger.Error().Err(err).Msgf("Failed to set code for %s", addr.RawAddress)
		return
	}
	state.CodeHash = code
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
func (stateDB *EVMStateDBAdapter) GetState(common.Address, hash.Hash32B) hash.Hash32B {
	logger.Error().Msg("GetState is not implemented")
	return hash.ZeroHash32B
}

// SetState sets state
func (stateDB *EVMStateDBAdapter) SetState(common.Address, hash.Hash32B, hash.Hash32B) {
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
func (stateDB *EVMStateDBAdapter) AddPreimage(hash.Hash32B, []byte) {
	logger.Error().Msg("AddPreimage is not implemented")
}

// ForEachStorage loops each storage
func (stateDB *EVMStateDBAdapter) ForEachStorage(common.Address, func(hash.Hash32B, hash.Hash32B) bool) {
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
func NewEVMContext(msg Message, header *Header, bc Blockchain, author *common.Address) vm.Context {
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
		Origin:      msg.From(),
		Coinbase:    *author,
		BlockNumber: new(big.Int).Set(header.Number),
		Time:        new(big.Int).Set(header.Time),
		Difficulty:  new(big.Int).Set(header.Difficulty),
		GasLimit:    header.GasLimit,
		GasPrice:    new(big.Int).Set(msg.GasPrice()),
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
