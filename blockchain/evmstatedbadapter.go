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
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/state"
)

// EVMStateDBAdapter represents the state db adapter for evm to access iotx blockchain
type EVMStateDBAdapter struct {
	bc             Blockchain
	sf             state.Factory
	logs           []*Log
	err            error
	blockHeight    uint64
	blockHash      hash.Hash32B
	executionIndex uint
	executionHash  hash.Hash32B
}

// NewEVMStateDBAdapter creates a new state db with iotx blockchain
func NewEVMStateDBAdapter(bc Blockchain, blockHeight uint64, blockHash hash.Hash32B, executionIndex uint, executionHash hash.Hash32B) *EVMStateDBAdapter {
	return &EVMStateDBAdapter{
		bc,
		bc.GetFactory(),
		[]*Log{},
		nil,
		blockHeight,
		blockHash,
		executionIndex,
		executionHash,
	}
}

func (stateDB *EVMStateDBAdapter) logError(err error) {
	if stateDB.err == nil {
		stateDB.err = err
	}
}

// Error returns the first stored error during evm contract execution
func (stateDB *EVMStateDBAdapter) Error() error {
	return stateDB.err
}

// CreateAccount creates an account in iotx blockchain
func (stateDB *EVMStateDBAdapter) CreateAccount(evmAddr common.Address) {
	addr, err := iotxaddress.GetAddressByHash(iotxaddress.IsTestnet, iotxaddress.ChainID, evmAddr.Bytes())
	if err != nil {
		logger.Error().Err(err).Msgf("Failed to generate address for %s", evmAddr.Hex())
		stateDB.logError(err)
		return
	}
	_, err = stateDB.sf.LoadOrCreateState(addr.RawAddress, 0)
	if err != nil {
		logger.Error().Err(err).Msg("CreateAccount")
		// stateDB.logError(err)
		return
	}
	logger.Debug().Hex("addrHash", evmAddr[:]).Msg("CreateAccount")
}

// SubBalance subtracts balance from account
func (stateDB *EVMStateDBAdapter) SubBalance(evmAddr common.Address, amount *big.Int) {
	if amount.Cmp(big.NewInt(int64(0))) == 0 {
		return
	}
	// stateDB.GetBalance(evmAddr)
	logger.Debug().Msgf("SubBalance %v from %s", amount, evmAddr.Hex())
	addr, err := iotxaddress.GetAddressByHash(iotxaddress.IsTestnet, iotxaddress.ChainID, evmAddr.Bytes())
	if err != nil {
		logger.Error().Err(err).Msgf("Failed to generate address for %s", evmAddr.Hex())
		stateDB.logError(err)
		return
	}
	state, err := stateDB.sf.CachedState(addr.RawAddress)
	if err != nil {
		logger.Error().Err(err).Msg("SubBalance")
		stateDB.logError(err)
		return
	}
	state.SubBalance(amount)
	// stateDB.GetBalance(evmAddr)
}

// AddBalance adds balance to account
func (stateDB *EVMStateDBAdapter) AddBalance(evmAddr common.Address, amount *big.Int) {
	if amount.Cmp(big.NewInt(int64(0))) == 0 {
		return
	}
	// stateDB.GetBalance(evmAddr)
	logger.Debug().Msgf("AddBalance %v to %s", amount, evmAddr.Hex())

	addr, err := iotxaddress.GetAddressByHash(iotxaddress.IsTestnet, iotxaddress.ChainID, evmAddr.Bytes())
	if err != nil {
		logger.Error().Err(err).Msgf("Failed to generate address for %s", evmAddr.Hex())
		stateDB.logError(err)
		return
	}
	state, err := stateDB.sf.CachedState(addr.RawAddress)
	if err != nil {
		logger.Error().Err(err).Hex("addrHash", evmAddr[:]).Msg("AddBalance")
		stateDB.logError(err)
		return
	}
	state.AddBalance(amount)
	// stateDB.GetBalance(evmAddr)
}

// GetBalance gets the balance of account
func (stateDB *EVMStateDBAdapter) GetBalance(evmAddr common.Address) *big.Int {
	addr, err := iotxaddress.GetAddressByHash(iotxaddress.IsTestnet, iotxaddress.ChainID, evmAddr.Bytes())
	if err != nil {
		logger.Error().Err(err).Msgf("Failed to generate address for %s", evmAddr.Hex())
		stateDB.logError(err)
		return nil
	}
	state, err := stateDB.sf.CachedState(addr.RawAddress)
	if err != nil {
		logger.Error().Err(err).Msg("GetBalance")
		return nil
	}
	logger.Debug().Msgf("Balance of %s is %v", evmAddr.Hex(), state.Balance)

	return state.Balance
}

// GetNonce gets the nonce of account
func (stateDB *EVMStateDBAdapter) GetNonce(evmAddr common.Address) uint64 {
	addr, err := iotxaddress.GetAddressByHash(iotxaddress.IsTestnet, iotxaddress.ChainID, evmAddr.Bytes())
	if err != nil {
		logger.Error().Err(err).Msgf("Failed to generate address for %s", evmAddr.Hex())
		stateDB.logError(err)
		return 0
	}
	nonce, err := stateDB.bc.Nonce(addr.RawAddress)
	if err != nil {
		logger.Error().Err(err).Msg("GetNonce")
		// stateDB.logError(err)
		return 0
	}
	logger.Debug().Uint64("nonce", nonce).Msg("GetNonce")
	return nonce
}

// SetNonce sets the nonce of account
func (stateDB *EVMStateDBAdapter) SetNonce(common.Address, uint64) {
	logger.Error().Msg("SetNonce is not implemented")
}

// GetCodeHash gets the code hash of account
func (stateDB *EVMStateDBAdapter) GetCodeHash(evmAddr common.Address) common.Hash {
	codeHash := common.Hash{}
	hash, err := stateDB.sf.GetCodeHash(byteutil.BytesTo20B(evmAddr[:]))
	if err != nil {
		logger.Error().Err(err).Msgf("GetCodeHash")
		// TODO (zhi) not all err should be logged
		// stateDB.logError(err)
		return codeHash
	}
	logger.Debug().Hex("codeHash", hash[:]).Msg("GetCodeHash")
	copy(codeHash[:], hash[:])
	return codeHash
}

// GetCode gets the code saved in hash
func (stateDB *EVMStateDBAdapter) GetCode(evmAddr common.Address) []byte {
	code, err := stateDB.sf.GetCode(byteutil.BytesTo20B(evmAddr[:]))
	if err != nil {
		// TODO: we need to change the log level to error later
		logger.Debug().Err(err).Msg("GetCode")
		return nil
	}
	logger.Debug().Hex("addrHash", evmAddr[:]).Msg("GetCode")
	return code
}

// SetCode sets the code saved in hash
func (stateDB *EVMStateDBAdapter) SetCode(evmAddr common.Address, code []byte) {
	if err := stateDB.sf.SetCode(byteutil.BytesTo20B(evmAddr[:]), code); err != nil {
		logger.Error().Err(err).Msg("SetCode")
	}
	logger.Debug().Hex("code", code).Hex("hash", hash.Hash256b(code)[:]).Msg("SetCode")
}

// GetCodeSize gets the code size saved in hash
func (stateDB *EVMStateDBAdapter) GetCodeSize(evmAddr common.Address) int {
	code := stateDB.GetCode(evmAddr)
	if code == nil {
		return 0
	}
	logger.Debug().Hex("addrHash", evmAddr[:]).Msg("GetCodeSize")
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
func (stateDB *EVMStateDBAdapter) GetState(evmAddr common.Address, k common.Hash) common.Hash {
	storage := common.Hash{}
	v, err := stateDB.sf.GetContractState(byteutil.BytesTo20B(evmAddr[:]), byteutil.BytesTo32B(k[:]))
	if err != nil {
		logger.Error().Err(err).Msg("GetState")
		return storage
	}
	logger.Debug().Hex("addrHash", evmAddr[:]).Hex("k", k[:]).Msg("GetState")
	copy(storage[:], v[:])
	return storage
}

// SetState sets state
func (stateDB *EVMStateDBAdapter) SetState(evmAddr common.Address, k, v common.Hash) {
	if err := stateDB.sf.SetContractState(byteutil.BytesTo20B(evmAddr[:]), byteutil.BytesTo32B(k[:]), byteutil.BytesTo32B(v[:])); err != nil {
		logger.Error().Err(err).Msg("SetState")
		return
	}
	logger.Debug().Hex("addrHash", evmAddr[:]).Hex("k", k[:]).Hex("v", v[:]).Msg("SetState")
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

// Exist checks the existance of an address
func (stateDB *EVMStateDBAdapter) Exist(evmAddr common.Address) bool {
	addr, err := iotxaddress.GetAddressByHash(iotxaddress.IsTestnet, iotxaddress.ChainID, evmAddr.Bytes())
	if err != nil {
		logger.Error().Err(err).Msgf("Failed to generate address for %s", evmAddr.Hex())
		stateDB.logError(err)
		return false
	}
	logger.Debug().Msgf("Check existence of %s", addr.RawAddress)
	if state, err := stateDB.sf.CachedState(addr.RawAddress); err != nil || state == nil {
		logger.Debug().Msgf("Account %s does not exist", addr.RawAddress)
		return false
	}
	return true
}

// Empty empties the contract
func (stateDB *EVMStateDBAdapter) Empty(common.Address) bool {
	logger.Debug().Msg("Empty is not implemented")
	return false
}

// RevertToSnapshot reverts the state factory to snapshot
func (stateDB *EVMStateDBAdapter) RevertToSnapshot(int) {
	logger.Debug().Msg("RevertToSnapshot is not implemented")
}

// Snapshot returns the snapshot id
func (stateDB *EVMStateDBAdapter) Snapshot() int {
	logger.Debug().Msg("Snapshot is not implemented")
	return 0
}

// AddLog adds log
func (stateDB *EVMStateDBAdapter) AddLog(evmLog *types.Log) {
	logger.Debug().Msgf("AddLog %+v\n", evmLog)
	addr, err := iotxaddress.GetAddressByHash(iotxaddress.IsTestnet, iotxaddress.ChainID, evmLog.Address.Bytes())
	if err != nil {
		logger.Error().Err(err).Msg("Invalid address in Log")
	}
	var topics []hash.Hash32B
	for _, evmTopic := range evmLog.Topics {
		var topic hash.Hash32B
		copy(topic[:], evmTopic.Bytes())
		topics = append(topics, topic)
	}
	log := &Log{
		addr.RawAddress,
		topics,
		evmLog.Data,
		stateDB.blockHeight,
		stateDB.executionHash,
		stateDB.blockHash,
		stateDB.executionIndex,
	}
	stateDB.logs = append(stateDB.logs, log)
}

// Logs returns the logs
func (stateDB *EVMStateDBAdapter) Logs() []*Log {
	return stateDB.logs
}

// AddPreimage adds the preimage
func (stateDB *EVMStateDBAdapter) AddPreimage(common.Hash, []byte) {
	logger.Error().Msg("AddPreimage is not implemented")
}

// ForEachStorage loops each storage
func (stateDB *EVMStateDBAdapter) ForEachStorage(common.Address, func(common.Hash, common.Hash) bool) {
	logger.Error().Msg("ForEachStorage is not implemented")
}
