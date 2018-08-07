// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"math/big"

	// "github.com/ethereum/go-ethereum/core/types"
	"github.com/iotexproject/iotex-core-internal/iotxaddress"
	"github.com/iotexproject/iotex-core-internal/logger"
	"github.com/iotexproject/iotex-core-internal/pkg/hash"
)

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
func (stateDB *EVMStateDBAdapter) CreateAccount([]byte) {
	logger.Error().Msg("CreateAccount is not implemented")
}

// SubBalance subtracts balance from account
func (stateDB *EVMStateDBAdapter) SubBalance([]byte, *big.Int) {
	logger.Error().Msg("SubBalance is not implemented")
}

// AddBalance adds balance to account
func (stateDB *EVMStateDBAdapter) AddBalance([]byte, *big.Int) {
	logger.Error().Msg("AddBalance is not implemented")
}

// GetBalance gets the balance of account
func (stateDB *EVMStateDBAdapter) GetBalance(hash []byte) *big.Int {
	addr := iotxaddress.GetAddressByHash(iotxaddress.IsTestnet, iotxaddress.ChainID, hash)
	balance, err := stateDB.bc.Balance(addr)
	if err != nil {
		logger.Error().Err(err).Msgf("Failed to get balance for %s", addr)
		return 0
	}
	return balance
}

// GetNonce gets the nonce of account
func (stateDB *EVMStateDBAdapter) GetNonce(hash []byte) uint64 {
	addr := iotxaddress.GetAddressByHash(iotxaddress.IsTestnet, iotxaddress.ChainID, hash)
	nonce, err := stateDB.bc.Nonce(addr)
	if err != nil {
		logger.Error().Err(err).Msgf("Failed to get nonce for %s", addr)
		return 0
	}
	return nonce
}

// SetNonce sets the nonce of account
func (stateDB *EVMStateDBAdapter) SetNonce([]byte, uint64) {
	logger.Error().Msg("SetNonce is not implemented")
}

// GetCodeHash gets the code hash of account
func (stateDB *EVMStateDBAdapter) GetCodeHash([]byte) hash.Hash32B {
	logger.Error().Msg("GetCodeHash is not implemented")
}

// GetCode gets the code saved in hash
func (stateDB *EVMStateDBAdapter) GetCode(hash []byte) []byte {
	addr := iotxaddress.GetAddressByHash(iotxaddress.IsTestnet, iotxaddress.ChainID, hash)
	state, error := stateDB.bc.StateByAddr(addr)
	if error != nil {
		logger.Error().Err(err).Msgf("Failed to get code for %s", addr)
		return nil
	}
	return state.CodeHash
}

// SetCode sets the code saved in hash
func (stateDB *EVMStateDBAdapter) SetCode(hash []byte, code []byte) {
	addr := iotxaddress.GetAddressByHash(iotxaddress.IsTestnet, iotxaddress.ChainID, hash)
	state, error := stateDB.bc.StateByAddr(addr)
	if err != nil {
		logger.Error().Err(err).Msgf("Failed to set code for %s", addr)
		return
	}
	state.CodeHash = code
}

// GetCodeSize gets the code size saved in hash
func (stateDB *EVMStateDBAdapter) GetCodeSize(hash []byte) int {
	code := stateDB.getCode(hash)
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
	return nil
}

// GetState gets state
func (stateDB *EVMStateDBAdapter) GetState([]byte, hash.Hash32B) hash.Hash32B {
	logger.Error().Msg("GetState is not implemented")
	return nil
}

// SetState sets state
func (stateDB *EVMStateDBAdapter) SetState([]byte, hash.Hash32B, hash.Hash32B) {
	logger.Error().Msg("SetState is not implemented")
}

// Suicide kills the contract
func (stateDB *EVMStateDBAdapter) Suicide([]byte) bool {
	logger.Error().Msg("Suicide is not implemented")
	return false
}

// HasSuicided returns whether the contract has been killed
func (stateDB *EVMStateDBAdapter) HasSuicided([]byte) bool {
	logger.Error().Msg("HasSuicide is not implemented")
	return false
}

// Exist exits the contract
func (stateDB *EVMStateDBAdapter) Exist([]byte) bool {
	logger.Error().Msg("Exist is not implemented")
	return false
}

// Empty empties the contract
func (stateDB *EVMStateDBAdapter) Empty([]byte) bool {
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

// TODO (zhi) add this function after extending the EVM
// AddLog adds log
// func (stateDB *EVMStateDBAdapter) AddLog(*types.Log) {
// 	logger.Error().Msg("AddLog is not implemented")
// }

// AddPreimage adds the preimage
func (stateDB *EVMStateDBAdapter) AddPreimage(hash.Hash32B, []byte) {
	logger.Error().Msg("AddPreimage is not implemented")
}

// ForEachStorage loops each storage
func (stateDB *EVMStateDBAdapter) ForEachStorage([]byte, func(hash.Hash32B, hash.Hash32B) bool) {
	logger.Error().Msg("ForEachStorage is not implemented")
}

// IoTXEVMCanTransfer checks whether the from account has enough balance
func IoTXEVMCanTransfer(db StateDB, fromHash []byte, balance *big.Int) bool {
	return db.GetBalance(fromHash) > balance
}

// IoTXEVMTransfer transfers account
func IoTXEVMTransfer(db StateDB, fromHash, toHash []byte, amount *big.Int) {
	db.SubBalance(fromHash, amount)
	db.AddBalance(toHash, amount)
}
