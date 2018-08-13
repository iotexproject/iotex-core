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
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/state"
)

// EVMStateDBAdapter represents the state db adapter for evm to access iotx blockchain
type EVMStateDBAdapter struct {
	bc Blockchain
	sf state.Factory
}

// NewEVMStateDBAdapter creates a new state db with iotx blockchain
func NewEVMStateDBAdapter(bc Blockchain) *EVMStateDBAdapter {
	return &EVMStateDBAdapter{
		bc,
		bc.GetFactory(),
	}
}

// CreateAccount creates an account in iotx blockchain
func (stateDB *EVMStateDBAdapter) CreateAccount(evmAddr common.Address) {
	addr, err := iotxaddress.GetAddressByHash(iotxaddress.IsTestnet, iotxaddress.ChainID, evmAddr.Bytes())
	if err != nil {
		logger.Error().Msg("Failed to get address by hash")
	}
	_, err = stateDB.bc.CreateState(addr.RawAddress, 0)
	if err != nil {
		logger.Error().Msg("Failed to create a new account")
	}
}

// SubBalance subtracts balance from account
func (stateDB *EVMStateDBAdapter) SubBalance(evmAddr common.Address, amount *big.Int) {
	addr, err := iotxaddress.GetAddressByHash(iotxaddress.IsTestnet, iotxaddress.ChainID, evmAddr.Bytes())
	if err != nil {
		logger.Error().Msg("Failed to get address by hash")
	}
	state, err := stateDB.bc.StateByAddr(addr.RawAddress)
	if err != nil {
		logger.Error().Msg("Failed to get the state")
	}
	state.SubBalance(amount)
}

// AddBalance adds balance to account
func (stateDB *EVMStateDBAdapter) AddBalance(evmAddr common.Address, amount *big.Int) {
	addr, err := iotxaddress.GetAddressByHash(iotxaddress.IsTestnet, iotxaddress.ChainID, evmAddr.Bytes())
	if err != nil {
		logger.Error().Msg("Failed to get address by hash")
	}
	state, err := stateDB.bc.StateByAddr(addr.RawAddress)
	if err != nil {
		logger.Error().Msg("Failed to get the state")
	}
	state.AddBalance(amount)
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
