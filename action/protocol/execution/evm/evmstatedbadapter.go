// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package evm

import (
	"bytes"
	"math/big"

	"github.com/CoderZhi/go-ethereum/common"
	"github.com/CoderZhi/go-ethereum/core/types"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/state"
)

type (
	// deleteAccount records the account/contract to be deleted
	deleteAccount map[hash.PKHash]struct{}

	// contractMap records the contracts being changed
	contractMap map[hash.PKHash]Contract

	// StateDBAdapter represents the state db adapter for evm to access iotx blockchain
	StateDBAdapter struct {
		cm               protocol.ChainManager
		sm               protocol.StateManager
		logs             []*action.Log
		err              error
		blockHeight      uint64
		blockHash        hash.Hash32B
		executionHash    hash.Hash32B
		refund           uint64
		cachedContract   contractMap
		contractSnapshot map[int]contractMap   // snapshots of contracts
		suicided         deleteAccount         // account/contract calling Suicide
		suicideSnapshot  map[int]deleteAccount // snapshots of suicide accounts
		dao              db.KVStore
		cb               db.CachedBatch
	}
)

// NewStateDBAdapter creates a new state db with iotex blockchain
func NewStateDBAdapter(cm protocol.ChainManager, sm protocol.StateManager, blockHeight uint64, blockHash hash.Hash32B, executionHash hash.Hash32B) *StateDBAdapter {
	return &StateDBAdapter{
		cm:               cm,
		sm:               sm,
		logs:             []*action.Log{},
		err:              nil,
		blockHeight:      blockHeight,
		blockHash:        blockHash,
		executionHash:    executionHash,
		cachedContract:   make(contractMap),
		contractSnapshot: make(map[int]contractMap),
		suicided:         make(deleteAccount),
		suicideSnapshot:  make(map[int]deleteAccount),
		dao:              sm.GetDB(),
		cb:               sm.GetCachedBatch(),
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
	_, err := account.LoadOrCreateAccount(stateDB.sm, addr.IotxAddress(), big.NewInt(0))
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
	if err := account.StoreAccount(stateDB.sm, addr.IotxAddress(), state); err != nil {
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
	state, err := account.LoadOrCreateAccount(stateDB.sm, addr.IotxAddress(), big.NewInt(0))
	if err != nil {
		logger.Error().Err(err).Hex("addrHash", evmAddr[:]).Msg("AddBalance")
		stateDB.logError(err)
		return
	}
	state.AddBalance(amount)
	if err := account.StoreAccount(stateDB.sm, addr.IotxAddress(), state); err != nil {
		logger.Error().Err(err).Msg("Failed to update pending account changes to trie")
	}
}

// GetBalance gets the balance of account
func (stateDB *StateDBAdapter) GetBalance(evmAddr common.Address) *big.Int {
	addr := address.New(stateDB.cm.ChainID(), evmAddr.Bytes())
	state, err := stateDB.AccountState(addr.IotxAddress())
	if err != nil {
		logger.Error().Err(err).Msg("GetBalance")
		return big.NewInt(0)
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
	logger.Debug().Uint64("nonce", state.Nonce).Str("address", addr.IotxAddress()).Msg("GetNonce")
	return state.Nonce
}

// SetNonce sets the nonce of account
func (stateDB *StateDBAdapter) SetNonce(evmAddr common.Address, nonce uint64) {
	addr := address.New(stateDB.cm.ChainID(), evmAddr.Bytes())
	s, err := stateDB.AccountState(addr.IotxAddress())
	if err != nil {
		logger.Error().Err(err).Msg("GetNonce")
		// stateDB.logError(err)
		return
	}
	logger.Debug().Uint64("nonce", nonce).Str("address", addr.IotxAddress()).Msg("SetNonce")
	s.Nonce = nonce
	if err := account.StoreAccount(stateDB.sm, addr.IotxAddress(), s); err != nil {
		logger.Error().Err(err).Msg("failed to set nonce")
		// stateDB.logError(err)
	}
}

// AddRefund adds refund
func (stateDB *StateDBAdapter) AddRefund(gas uint64) {
	logger.Debug().Uint64("gas", gas).Msg("AddRefund")
	// stateDB.journal.append(refundChange{prev: self.refund})
	stateDB.refund += gas
}

// GetRefund gets refund
func (stateDB *StateDBAdapter) GetRefund() uint64 {
	logger.Debug().Msg("GetRefund")
	return stateDB.refund
}

// Suicide kills the contract
func (stateDB *StateDBAdapter) Suicide(evmAddr common.Address) bool {
	addr := address.New(stateDB.cm.ChainID(), evmAddr.Bytes())
	s, err := stateDB.AccountState(addr.IotxAddress())
	if err != nil || s == state.EmptyAccount {
		logger.Debug().Msgf("account %s does not exist", addr.IotxAddress())
		return false
	}
	// clears the account balance
	s.Balance = nil
	s.Balance = big.NewInt(0)
	addrHash := byteutil.BytesTo20B(evmAddr.Bytes())
	stateDB.sm.PutState(addrHash, s)
	// mark it as deleted
	stateDB.suicided[addrHash] = struct{}{}
	return true
}

// HasSuicided returns whether the contract has been killed
func (stateDB *StateDBAdapter) HasSuicided(evmAddr common.Address) bool {
	addrHash := byteutil.BytesTo20B(evmAddr.Bytes())
	_, ok := stateDB.suicided[addrHash]
	return ok
}

// Exist checks the existence of an address
func (stateDB *StateDBAdapter) Exist(evmAddr common.Address) bool {
	addr := address.New(stateDB.cm.ChainID(), evmAddr.Bytes())
	logger.Debug().Msgf("Check existence of %s, %x", addr.IotxAddress(), evmAddr[:])
	if s, err := stateDB.AccountState(addr.IotxAddress()); err != nil || s == state.EmptyAccount {
		logger.Debug().Msgf("account %s does not exist", addr.IotxAddress())
		return false
	}
	return true
}

// Empty returns true if the the contract is empty
func (stateDB *StateDBAdapter) Empty(evmAddr common.Address) bool {
	addr := address.New(stateDB.cm.ChainID(), evmAddr.Bytes())
	logger.Debug().Msgf("Check whether the contract is empty")
	s, err := stateDB.AccountState(addr.IotxAddress())
	if err != nil || s == state.EmptyAccount {
		return true
	}
	// TODO: delete hash.ZeroHash32B
	return s.Nonce == 0 &&
		s.Balance.Sign() == 0 &&
		(len(s.CodeHash) == 0 || bytes.Equal(s.CodeHash, hash.ZeroHash32B[:]))
}

// RevertToSnapshot reverts the state factory to the state at a given snapshot
func (stateDB *StateDBAdapter) RevertToSnapshot(snapshot int) {
	// TODO: temp disable until we solve the memory thrash/leak issue
	logger.Debug().Msg("RevertToSnapshot is not implemented")
	return

	if err := stateDB.sm.Revert(snapshot); err != nil {
		err := errors.New("unexpected error: state manager's Revert() failed")
		logger.Error().Err(err).Msg("failed to RevertToSnapshot")
		// stateDB.err = err
		return
	}
	ds, ok := stateDB.suicideSnapshot[snapshot]
	if !ok {
		// this should not happen, b/c we save the suicide accounts on a successful return of Snapshot(), but check anyway
		logger.Error().Msgf("failed to get snapshot = %d", snapshot)
		return
	}
	// restore the suicide accounts
	stateDB.suicided = nil
	stateDB.suicided = ds
	// restore modified contracts
	stateDB.cachedContract = nil
	stateDB.cachedContract = stateDB.contractSnapshot[snapshot]
	for addr, c := range stateDB.cachedContract {
		if err := c.LoadRoot(); err != nil {
			logger.Error().Msgf("failed to load root for contract %x", addr)
			return
		}
	}
}

// Snapshot returns the snapshot id
func (stateDB *StateDBAdapter) Snapshot() int {
	// TODO: temp disable until we solve the memory thrash/leak issue
	logger.Debug().Msg("Snapshot is not implemented")
	return 0

	sn := stateDB.sm.Snapshot()
	if _, ok := stateDB.suicideSnapshot[sn]; ok {
		err := errors.New("unexpected error: duplicate snapshot version")
		logger.Error().Err(err).Msg("failed to Snapshot")
		// stateDB.err = err
		return sn
	}
	// save a copy of current suicide accounts
	sa := make(deleteAccount)
	for k, v := range stateDB.suicided {
		sa[k] = v
	}
	stateDB.suicideSnapshot[sn] = sa
	// save a copy of modified contracts
	c := make(contractMap)
	for k, v := range stateDB.cachedContract {
		c[k] = v.Snapshot()
	}
	stateDB.contractSnapshot[sn] = c
	return sn
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
	return account.LoadAccount(stateDB.sm, addrHash)
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
	account, err := account.LoadAccount(stateDB.sm, addr)
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
	account, err := account.LoadAccount(stateDB.sm, addr)
	if err != nil {
		logger.Error().Err(err).Msgf("Failed to load account state for address %x", addr)
		return nil
	}
	code, err := stateDB.dao.Get(CodeKVNameSpace, account.CodeHash[:])
	if err != nil {
		// TODO: Suppress the as it's too much now
		// logger.Error().Err(err).Msg("Failed to get code from trie")
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
		if _, ok := stateDB.suicided[addr]; ok {
			// no need to update a suicide account/contract
			continue
		}
		if err := contract.Commit(); err != nil {
			return errors.Wrap(err, "failed to commit contract")
		}
		state := contract.SelfState()
		// store the account (with new storage trie root) into account trie
		if err := stateDB.sm.PutState(addr, state); err != nil {
			return errors.Wrap(err, "failed to update pending account changes to trie")
		}
	}
	// delete suicided accounts/contract
	for addr := range stateDB.suicided {
		if err := stateDB.sm.DelState(addr); err != nil {
			return errors.Wrapf(err, "failed to delete suicide account/contract %x", addr[:])
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
	account, err := account.LoadAccount(stateDB.sm, addr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load account state for address %x", addr)
	}
	if account == state.EmptyAccount {
		// emptyAccount is read-only, instantiate a new empty account for contract creation
		account = &state.Account{
			Balance:      big.NewInt(0),
			VotingWeight: big.NewInt(0),
		}
	}
	contract, err := newContract(account, stateDB.dao, stateDB.cb)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create storage trie for new contract %x", addr)
	}
	// add to contract cache
	stateDB.cachedContract[addr] = contract
	return contract, nil
}

// clear clears local changes
func (stateDB *StateDBAdapter) clear() {
	stateDB.cachedContract = nil
	stateDB.contractSnapshot = nil
	stateDB.suicided = nil
	stateDB.suicideSnapshot = nil
	stateDB.cachedContract = make(contractMap)
	stateDB.contractSnapshot = make(map[int]contractMap)
	stateDB.suicided = make(deleteAccount)
	stateDB.suicideSnapshot = make(map[int]deleteAccount)
}
