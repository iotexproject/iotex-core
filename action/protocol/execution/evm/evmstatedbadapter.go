// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package evm

import (
	"math/big"

	"github.com/CoderZhi/go-ethereum/common"
	"github.com/CoderZhi/go-ethereum/core/types"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/db/trie"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/state"
)

// StateDBAdapter represents the state db adapter for evm to access iotx blockchain
type StateDBAdapter struct {
	cm             protocol.ChainManager
	sm             protocol.StateManager
	logs           []*action.Log
	err            error
	blockHeight    uint64
	blockHash      hash.Hash32B
	executionHash  hash.Hash32B
	cachedContract map[hash.PKHash]Contract
	dao            db.KVStore
	cb             db.CachedBatch
}

// NewStateDBAdapter creates a new state db with iotex blockchain
func NewStateDBAdapter(cm protocol.ChainManager, sm protocol.StateManager, blockHeight uint64, blockHash hash.Hash32B, executionHash hash.Hash32B) *StateDBAdapter {
	return &StateDBAdapter{
		cm:             cm,
		sm:             sm,
		logs:           []*action.Log{},
		err:            nil,
		blockHeight:    blockHeight,
		blockHash:      blockHash,
		executionHash:  executionHash,
		cachedContract: make(map[hash.PKHash]Contract),
		dao:            sm.GetDB(),
		cb:             sm.GetCachedBatch(),
	}
}

func (stateDB *StateDBAdapter) logError(err error) {
	if stateDB.err == nil {
		stateDB.err = err
	}
}

// Error returns the first stored error during evm contract execution
func (stateDB *StateDBAdapter) Error() error {
	return stateDB.err
}

// CreateAccount creates an account in iotx blockchain
func (stateDB *StateDBAdapter) CreateAccount(evmAddr common.Address) {
	addr := address.New(stateDB.cm.ChainID(), evmAddr.Bytes())
	_, err := account.LoadOrCreateAccountState(stateDB.sm, addr.IotxAddress(), big.NewInt(0))
	if err != nil {
		logger.Error().Err(err).Msg("CreateAccount")
		// stateDB.logError(err)
		return
	}
	logger.Debug().Hex("addrHash", evmAddr[:]).Msg("CreateAccount")
}

// SubBalance subtracts balance from account
func (stateDB *StateDBAdapter) SubBalance(evmAddr common.Address, amount *big.Int) {
	if amount.Cmp(big.NewInt(int64(0))) == 0 {
		return
	}
	// stateDB.GetBalance(evmAddr)
	logger.Debug().Msgf("SubBalance %v from %s", amount, evmAddr.Hex())
	addr := address.New(stateDB.cm.ChainID(), evmAddr.Bytes())
	state, err := stateDB.AccountState(addr.IotxAddress())
	if err != nil {
		logger.Error().Err(err).Msg("SubBalance")
		stateDB.logError(err)
		return
	}
	state.SubBalance(amount)
	if err := account.StoreState(stateDB.sm, addr.IotxAddress(), state); err != nil {
		logger.Error().Err(err).Msg("Failed to update pending account changes to trie")
	}
	// stateDB.GetBalance(evmAddr)
}

// AddBalance adds balance to account
func (stateDB *StateDBAdapter) AddBalance(evmAddr common.Address, amount *big.Int) {
	if amount.Cmp(big.NewInt(int64(0))) == 0 {
		return
	}
	// stateDB.GetBalance(evmAddr)
	logger.Debug().Msgf("AddBalance %v to %s", amount, evmAddr.Hex())

	addr := address.New(stateDB.cm.ChainID(), evmAddr.Bytes())
	state, err := account.LoadOrCreateAccountState(stateDB.sm, addr.IotxAddress(), big.NewInt(0))
	if err != nil {
		logger.Error().Err(err).Hex("addrHash", evmAddr[:]).Msg("AddBalance")
		stateDB.logError(err)
		return
	}
	state.AddBalance(amount)
	if err := account.StoreState(stateDB.sm, addr.IotxAddress(), state); err != nil {
		logger.Error().Err(err).Msg("Failed to update pending account changes to trie")
	}
}

// GetBalance gets the balance of account
func (stateDB *StateDBAdapter) GetBalance(evmAddr common.Address) *big.Int {
	addr := address.New(stateDB.cm.ChainID(), evmAddr.Bytes())
	state, err := stateDB.AccountState(addr.IotxAddress())
	if err != nil {
		logger.Error().Err(err).Msg("GetBalance")
		return nil
	}
	logger.Debug().Msgf("Balance of %s is %v", evmAddr.Hex(), state.Balance)

	return state.Balance
}

// GetNonce gets the nonce of account
func (stateDB *StateDBAdapter) GetNonce(evmAddr common.Address) uint64 {
	addr := address.New(stateDB.cm.ChainID(), evmAddr.Bytes())
	state, err := stateDB.AccountState(addr.IotxAddress())
	if err != nil {
		logger.Error().Err(err).Msg("GetNonce")
		// stateDB.logError(err)
		return 0
	}
	logger.Debug().Uint64("nonce", state.Nonce).Msg("GetNonce")
	return state.Nonce
}

// SetNonce sets the nonce of account
func (stateDB *StateDBAdapter) SetNonce(common.Address, uint64) {
	logger.Error().Msg("SetNonce is not implemented")
}

// AddRefund adds refund
func (stateDB *StateDBAdapter) AddRefund(uint64) {
	logger.Error().Msg("AddRefund is not implemented")
}

// GetRefund gets refund
func (stateDB *StateDBAdapter) GetRefund() uint64 {
	logger.Error().Msg("GetRefund is not implemented")
	return 0
}

// Suicide kills the contract
func (stateDB *StateDBAdapter) Suicide(common.Address) bool {
	logger.Error().Msg("Suicide is not implemented")
	return false
}

// HasSuicided returns whether the contract has been killed
func (stateDB *StateDBAdapter) HasSuicided(common.Address) bool {
	logger.Error().Msg("HasSuicide is not implemented")
	return false
}

// Exist checks the existence of an address
func (stateDB *StateDBAdapter) Exist(evmAddr common.Address) bool {
	addr := address.New(stateDB.cm.ChainID(), evmAddr.Bytes())
	logger.Debug().Msgf("Check existence of %s, %x", addr.IotxAddress(), evmAddr[:])
	if state, err := stateDB.AccountState(addr.IotxAddress()); err != nil || state == nil {
		logger.Debug().Msgf("account %s does not exist", addr.IotxAddress())
		return false
	}
	return true
}

// Empty empties the contract
func (stateDB *StateDBAdapter) Empty(common.Address) bool {
	logger.Debug().Msg("Empty is not implemented")
	return false
}

// RevertToSnapshot reverts the state factory to snapshot
func (stateDB *StateDBAdapter) RevertToSnapshot(int) {
	logger.Debug().Msg("RevertToSnapshot is not implemented")
}

// Snapshot returns the snapshot id
func (stateDB *StateDBAdapter) Snapshot() int {
	logger.Debug().Msg("Snapshot is not implemented")
	return 0
}

// AddLog adds log
func (stateDB *StateDBAdapter) AddLog(evmLog *types.Log) {
	logger.Debug().Msgf("AddLog %+v\n", evmLog)
	addr := address.New(stateDB.cm.ChainID(), evmLog.Address.Bytes())
	var topics []hash.Hash32B
	for _, evmTopic := range evmLog.Topics {
		var topic hash.Hash32B
		copy(topic[:], evmTopic.Bytes())
		topics = append(topics, topic)
	}
	log := &action.Log{
		Address:     addr.IotxAddress(),
		Topics:      topics,
		Data:        evmLog.Data,
		BlockNumber: stateDB.blockHeight,
		TxnHash:     stateDB.executionHash,
		BlockHash:   stateDB.blockHash,
	}
	stateDB.logs = append(stateDB.logs, log)
}

// Logs returns the logs
func (stateDB *StateDBAdapter) Logs() []*action.Log {
	return stateDB.logs
}

// AddPreimage adds the preimage
func (stateDB *StateDBAdapter) AddPreimage(common.Hash, []byte) {
	logger.Error().Msg("AddPreimage is not implemented")
}

// ForEachStorage loops each storage
func (stateDB *StateDBAdapter) ForEachStorage(addr common.Address, cb func(common.Hash, common.Hash) bool) {
	ctt, err := stateDB.Contract(byteutil.BytesTo20B(addr[:]))
	if err != nil {
		// stateDB.err = err
		return
	}
	iter, err := ctt.Iterator()
	if err != nil {
		// stateDB.err = err
		return
	}

	for {
		key, value, err := iter.Next()
		if err != nil {
			break
		}
		ckey := common.Hash{}
		copy(ckey[:], key[:])
		cvalue := common.Hash{}
		copy(cvalue[:], value[:])
		cb(ckey, cvalue)
	}
}

// AccountState returns an account state
func (stateDB *StateDBAdapter) AccountState(addr string) (*state.Account, error) {
	addrHash, err := iotxaddress.AddressToPKHash(addr)
	if err != nil {
		return nil, err
	}
	if contract, ok := stateDB.cachedContract[addrHash]; ok {
		return contract.SelfState(), nil
	}
	return account.LoadAccountState(stateDB.sm, addrHash)
}

//======================================
// Contract functions
//======================================

// GetCodeHash returns contract's code hash
func (stateDB *StateDBAdapter) GetCodeHash(evmAddr common.Address) common.Hash {
	addr := byteutil.BytesTo20B(evmAddr[:])
	codeHash := common.Hash{}
	if contract, ok := stateDB.cachedContract[addr]; ok {
		copy(codeHash[:], contract.SelfState().CodeHash)
		return codeHash
	}
	account, err := account.LoadAccountState(stateDB.sm, addr)
	if err != nil {
		logger.Error().Err(err).Msg("GetCodeHash")
		// TODO (zhi) not all err should be logged
		// stateDB.logError(err)
		return codeHash
	}
	copy(codeHash[:], account.CodeHash)
	return codeHash
}

// GetCode returns contract's code
func (stateDB *StateDBAdapter) GetCode(evmAddr common.Address) []byte {
	addr := byteutil.BytesTo20B(evmAddr[:])
	if contract, ok := stateDB.cachedContract[addr]; ok {
		code, err := contract.GetCode()
		if err != nil {
			logger.Error().Err(err).Msg("GetCode")
			return nil
		}
		return code
	}
	account, err := account.LoadAccountState(stateDB.sm, addr)
	if err != nil {
		logger.Error().Err(err).Msgf("Failed to load account state for address %x", addr)
		return nil
	}
	code, err := stateDB.dao.Get(CodeKVNameSpace, account.CodeHash[:])
	if err != nil {
		logger.Error().Err(err).Msg("Failed to get code from trie")
		return nil
	}
	return code
}

// GetCodeSize gets the code size saved in hash
func (stateDB *StateDBAdapter) GetCodeSize(evmAddr common.Address) int {
	code := stateDB.GetCode(evmAddr)
	if code == nil {
		return 0
	}
	logger.Debug().Hex("addrHash", evmAddr[:]).Msg("GetCodeSize")
	return len(code)
}

// SetCode sets contract's code
func (stateDB *StateDBAdapter) SetCode(evmAddr common.Address, code []byte) {
	addr := byteutil.BytesTo20B(evmAddr[:])
	if contract, ok := stateDB.cachedContract[addr]; ok {
		contract.SetCode(byteutil.BytesTo32B(hash.Hash256b(code)), code)
		return
	}
	contract, err := stateDB.getContract(addr)
	if err != nil {
		logger.Error().Err(err).Msgf("Failed to get contract for address %x", addr)
	}
	contract.SetCode(byteutil.BytesTo32B(hash.Hash256b(code)), code)
}

// GetState gets state
func (stateDB *StateDBAdapter) GetState(evmAddr common.Address, k common.Hash) common.Hash {
	storage := common.Hash{}
	v, err := stateDB.getContractState(byteutil.BytesTo20B(evmAddr[:]), byteutil.BytesTo32B(k[:]))
	if err != nil {
		logger.Error().Err(err).Msg("GetState")
		return storage
	}
	logger.Debug().Hex("addrHash", evmAddr[:]).Hex("k", k[:]).Msg("GetState")
	copy(storage[:], v[:])
	return storage
}

// SetState sets state
func (stateDB *StateDBAdapter) SetState(evmAddr common.Address, k, v common.Hash) {
	if err := stateDB.setContractState(byteutil.BytesTo20B(evmAddr[:]), byteutil.BytesTo32B(k[:]), byteutil.BytesTo32B(v[:])); err != nil {
		logger.Error().Err(err).Msg("SetState")
		return
	}
	logger.Debug().Hex("addrHash", evmAddr[:]).Hex("k", k[:]).Hex("v", v[:]).Msg("SetState")
}

// getContractState returns contract's storage value
func (stateDB *StateDBAdapter) getContractState(addr hash.PKHash, key hash.Hash32B) (hash.Hash32B, error) {
	if contract, ok := stateDB.cachedContract[addr]; ok {
		v, err := contract.GetState(key)
		return byteutil.BytesTo32B(v), err
	}
	contract, err := stateDB.getContract(addr)
	if err != nil {
		return hash.ZeroHash32B, errors.Wrapf(err, "failed to GetContractState for contract %x", addr)
	}
	v, err := contract.GetState(key)
	return byteutil.BytesTo32B(v), err
}

// setContractState writes contract's storage value
func (stateDB *StateDBAdapter) setContractState(addr hash.PKHash, key, value hash.Hash32B) error {
	if contract, ok := stateDB.cachedContract[addr]; ok {
		return contract.SetState(key, value[:])
	}
	contract, err := stateDB.getContract(addr)
	if err != nil {
		return errors.Wrapf(err, "failed to SetContractState for contract %x", addr)
	}
	return contract.SetState(key, value[:])
}

// commitContracts commits contract code to db and update pending contract account changes to trie
func (stateDB *StateDBAdapter) commitContracts() error {
	for addr, contract := range stateDB.cachedContract {
		if err := contract.Commit(); err != nil {
			return errors.Wrap(err, "failed to commit contract")
		}
		state := contract.SelfState()
		// store the account (with new storage trie root) into account trie
		if err := stateDB.sm.PutState(addr, state); err != nil {
			return errors.Wrap(err, "failed to update pending account changes to trie")
		}
	}
	return nil
}

// Contract returns the contract of addr
func (stateDB *StateDBAdapter) Contract(addr hash.PKHash) (Contract, error) {
	if contract, ok := stateDB.cachedContract[addr]; ok {
		return contract, nil
	}
	return stateDB.getContract(addr)
}

func (stateDB *StateDBAdapter) getContract(addr hash.PKHash) (Contract, error) {
	account, err := account.LoadAccountState(stateDB.sm, addr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load account state for address %x", addr)
	}
	if account.Root == hash.ZeroHash32B {
		account.Root = trie.EmptyBranchNodeHash
	}
	// add to contract cache
	contract, err := newContract(account, stateDB.dao, stateDB.cb)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create storage trie for new contract %x", addr)
	}
	stateDB.cachedContract[addr] = contract
	return contract, nil
}

// clearCachedContracts clears cached contracts
func (stateDB *StateDBAdapter) clearCachedContracts() {
	stateDB.cachedContract = nil
	stateDB.cachedContract = make(map[hash.PKHash]Contract)
}
