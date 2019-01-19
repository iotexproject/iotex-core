// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package evm

import (
	"bytes"
	"fmt"
	"math/big"

	"github.com/CoderZhi/go-ethereum/common"
	"github.com/CoderZhi/go-ethereum/core/types"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/state"
)

type (
	// deleteAccount records the account/contract to be deleted
	deleteAccount map[hash.PKHash]struct{}

	// contractMap records the contracts being changed
	contractMap map[hash.PKHash]Contract

	// preimageMap records the preimage of hash reported by VM
	preimageMap map[common.Hash][]byte

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
		preimages        preimageMap
		preimageSnapshot map[int]preimageMap
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
		preimages:        make(preimageMap),
		preimageSnapshot: make(map[int]preimageMap),
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
	addr := address.New(evmAddr.Bytes())
	_, err := account.LoadOrCreateAccount(stateDB.sm, addr.Bech32(), big.NewInt(0))
	if err != nil {
		log.L().Error("Failed to create account.", zap.Error(err))
		// stateDB.logError(err)
		return
	}
	log.L().Debug("Called CreateAccount.", log.Hex("addrHash", evmAddr[:]))
}

// SubBalance subtracts balance from account
func (stateDB *StateDBAdapter) SubBalance(evmAddr common.Address, amount *big.Int) {
	if amount.Cmp(big.NewInt(int64(0))) == 0 {
		return
	}
	// stateDB.GetBalance(evmAddr)
	log.L().Debug(fmt.Sprintf("SubBalance %v from %s", amount, evmAddr.Hex()))
	addr := address.New(evmAddr.Bytes())
	state, err := stateDB.AccountState(addr.Bech32())
	if err != nil {
		log.L().Error("Failed to sub balance.", zap.Error(err))
		stateDB.logError(err)
		return
	}
	state.SubBalance(amount)
	if err := account.StoreAccount(stateDB.sm, addr.Bech32(), state); err != nil {
		log.L().Error("Failed to update pending account changes to trie.", zap.Error(err))
	}
	// stateDB.GetBalance(evmAddr)
}

// AddBalance adds balance to account
func (stateDB *StateDBAdapter) AddBalance(evmAddr common.Address, amount *big.Int) {
	if amount.Cmp(big.NewInt(int64(0))) == 0 {
		return
	}
	// stateDB.GetBalance(evmAddr)
	log.L().Debug(fmt.Sprintf("AddBalance %v from %s", amount, evmAddr.Hex()))

	addr := address.New(evmAddr.Bytes())
	state, err := account.LoadOrCreateAccount(stateDB.sm, addr.Bech32(), big.NewInt(0))
	if err != nil {
		log.L().Error("Failed to add balance.", log.Hex("addrHash", evmAddr[:]))
		stateDB.logError(err)
		return
	}
	state.AddBalance(amount)
	if err := account.StoreAccount(stateDB.sm, addr.Bech32(), state); err != nil {
		log.L().Error("Failed to update pending account changes to trie.", zap.Error(err))
	}
}

// GetBalance gets the balance of account
func (stateDB *StateDBAdapter) GetBalance(evmAddr common.Address) *big.Int {
	addr := address.New(evmAddr.Bytes())
	state, err := stateDB.AccountState(addr.Bech32())
	if err != nil {
		log.L().Error("Failed to get balance.", zap.Error(err))
		return big.NewInt(0)
	}
	log.L().Debug(fmt.Sprintf("Balance of %s is %v", evmAddr.Hex(), state.Balance))

	return state.Balance
}

// GetNonce gets the nonce of account
func (stateDB *StateDBAdapter) GetNonce(evmAddr common.Address) uint64 {
	addr := address.New(evmAddr.Bytes())
	state, err := stateDB.AccountState(addr.Bech32())
	if err != nil {
		log.L().Error("Failed to get nonce.", zap.Error(err))
		// stateDB.logError(err)
		return 0
	}
	log.L().Debug("Called GetNonce.",
		zap.String("address", addr.Bech32()),
		zap.Uint64("nonce", state.Nonce))
	return state.Nonce
}

// SetNonce sets the nonce of account
func (stateDB *StateDBAdapter) SetNonce(evmAddr common.Address, nonce uint64) {
	addr := address.New(evmAddr.Bytes())
	s, err := stateDB.AccountState(addr.Bech32())
	if err != nil {
		log.L().Error("Failed to set nonce.", zap.Error(err))
		// stateDB.logError(err)
		return
	}
	log.L().Debug("Called SetNonce.",
		zap.String("address", addr.Bech32()),
		zap.Uint64("nonce", nonce))
	s.Nonce = nonce
	if err := account.StoreAccount(stateDB.sm, addr.Bech32(), s); err != nil {
		log.L().Error("Failed to set nonce.", zap.Error(err))
		// stateDB.logError(err)
	}
}

// AddRefund adds refund
func (stateDB *StateDBAdapter) AddRefund(gas uint64) {
	log.L().Debug("Called AddRefund.", zap.Uint64("gas", gas))
	// stateDB.journal.append(refundChange{prev: self.refund})
	stateDB.refund += gas
}

// GetRefund gets refund
func (stateDB *StateDBAdapter) GetRefund() uint64 {
	log.L().Debug("Called GetRefund.")
	return stateDB.refund
}

// Suicide kills the contract
func (stateDB *StateDBAdapter) Suicide(evmAddr common.Address) bool {
	addr := address.New(evmAddr.Bytes())
	s, err := stateDB.AccountState(addr.Bech32())
	if err != nil || s == state.EmptyAccount {
		log.L().Debug("Account does not exist.", zap.String("address", addr.Bech32()))
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
	addr := address.New(evmAddr.Bytes())
	log.L().Debug("Check existence.", zap.String("address", addr.Bech32()), log.Hex("addrHash", evmAddr[:]))
	if s, err := stateDB.AccountState(addr.Bech32()); err != nil || s == state.EmptyAccount {
		log.L().Debug("Account does not exist.", zap.String("address", addr.Bech32()))
		return false
	}
	return true
}

// Empty returns true if the the contract is empty
func (stateDB *StateDBAdapter) Empty(evmAddr common.Address) bool {
	addr := address.New(evmAddr.Bytes())
	log.L().Debug("Check whether the contract is empty.")
	s, err := stateDB.AccountState(addr.Bech32())
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
	if err := stateDB.sm.Revert(snapshot); err != nil {
		err := errors.New("unexpected error: state manager's Revert() failed")
		log.L().Error("Failed to revert to snapshot.", zap.Error(err))
		// stateDB.err = err
		return
	}
	ds, ok := stateDB.suicideSnapshot[snapshot]
	if !ok {
		// this should not happen, b/c we save the suicide accounts on a successful return of Snapshot(), but check anyway
		log.L().Error("Failed to get snapshot.", zap.Int("snapshot", snapshot))
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
			log.L().Error("Failed to load root for contract.", log.Hex("addrHash", addr[:]))
			return
		}
	}
	// restore preimages
	stateDB.preimages = nil
	stateDB.preimages = stateDB.preimageSnapshot[snapshot]
}

// Snapshot returns the snapshot id
func (stateDB *StateDBAdapter) Snapshot() int {
	sn := stateDB.sm.Snapshot()
	if _, ok := stateDB.suicideSnapshot[sn]; ok {
		err := errors.New("unexpected error: duplicate snapshot version")
		log.L().Error("Failed to snapshot.", zap.Error(err))
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
	// save a copy of preimages
	p := make(preimageMap)
	for k, v := range stateDB.preimages {
		p[k] = v
	}
	stateDB.preimageSnapshot[sn] = p
	return sn
}

// AddLog adds log
func (stateDB *StateDBAdapter) AddLog(evmLog *types.Log) {
	log.L().Debug("Called AddLog.", zap.Any("log", evmLog))
	addr := address.New(evmLog.Address.Bytes())
	var topics []hash.Hash32B
	for _, evmTopic := range evmLog.Topics {
		var topic hash.Hash32B
		copy(topic[:], evmTopic.Bytes())
		topics = append(topics, topic)
	}
	log := &action.Log{
		Address:     addr.Bech32(),
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

// AddPreimage adds the preimage of a hash
func (stateDB *StateDBAdapter) AddPreimage(hash common.Hash, preimage []byte) {
	if _, ok := stateDB.preimages[hash]; !ok {
		b := make([]byte, len(preimage))
		copy(b, preimage)
		stateDB.preimages[hash] = b
	}
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
	addrHash, err := address.Bech32ToPKHash(addr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get public key hash from encoded address")
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
		log.L().Error("Failed to get code hash.", zap.Error(err))
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
			log.L().Error("Failed to get code hash.", zap.Error(err))
			return nil
		}
		return code
	}
	account, err := account.LoadAccount(stateDB.sm, addr)
	if err != nil {
		log.L().Error("Failed to load account state for address.", log.Hex("addrHash", addr[:]))
		return nil
	}
	code, err := stateDB.dao.Get(CodeKVNameSpace, account.CodeHash[:])
	if err != nil {
		// TODO: Suppress the as it's too much now
		//log.L().Error("Failed to get code from trie.", zap.Error(err))
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
	log.L().Debug("Called GetCodeSize.", log.Hex("addrHash", evmAddr[:]))
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
		log.L().Error("Failed to get contract.", zap.Error(err), log.Hex("addrHash", addr[:]))
	}
	contract.SetCode(byteutil.BytesTo32B(hash.Hash256b(code)), code)
}

// GetState gets state
func (stateDB *StateDBAdapter) GetState(evmAddr common.Address, k common.Hash) common.Hash {
	storage := common.Hash{}
	v, err := stateDB.getContractState(byteutil.BytesTo20B(evmAddr[:]), byteutil.BytesTo32B(k[:]))
	if err != nil {
		log.L().Error("Failed to get state.", zap.Error(err))
		return storage
	}
	log.L().Debug("Called GetState", log.Hex("addrHash", evmAddr[:]), log.Hex("k", k[:]))
	copy(storage[:], v[:])
	return storage
}

// SetState sets state
func (stateDB *StateDBAdapter) SetState(evmAddr common.Address, k, v common.Hash) {
	if err := stateDB.setContractState(byteutil.BytesTo20B(evmAddr[:]), byteutil.BytesTo32B(k[:]), byteutil.BytesTo32B(v[:])); err != nil {
		log.L().Error("Failed to set state.", zap.Error(err))
		return
	}
	log.L().Debug("Called SetState", log.Hex("addrHash", evmAddr[:]), log.Hex("k", k[:]))
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
	// write preimages to DB
	for k, v := range stateDB.preimages {
		h := make([]byte, len(k))
		copy(h, k[:])
		stateDB.cb.Put(PreimageKVNameSpace, h, v, "failed to put hash %x preimage %x", k, v)
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
	stateDB.preimages = nil
	stateDB.preimageSnapshot = nil
	stateDB.cachedContract = make(contractMap)
	stateDB.contractSnapshot = make(map[int]contractMap)
	stateDB.suicided = make(deleteAccount)
	stateDB.suicideSnapshot = make(map[int]deleteAccount)
	stateDB.preimages = make(preimageMap)
	stateDB.preimageSnapshot = make(map[int]preimageMap)
}
