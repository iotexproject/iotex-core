// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package evm

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/db/trie"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/state"
)

type (
	// deleteAccount records the account/contract to be deleted
	deleteAccount map[hash.Hash160]struct{}

	// contractMap records the contracts being changed
	contractMap map[hash.Hash160]Contract

	// preimageMap records the preimage of hash reported by VM
	preimageMap map[common.Hash]protocol.SerializableBytes

	// GetBlockHash gets block hash by height
	GetBlockHash func(uint64) (hash.Hash256, error)

	// DepositGas deposits gas
	DepositGas func(context.Context, protocol.StateManager, *big.Int) (*action.TransactionLog, error)

	// StateDBAdapter represents the state db adapter for evm to access iotx blockchain
	StateDBAdapter struct {
		sm                  protocol.StateManager
		logs                []*action.Log
		transactionLogs     []*action.TransactionLog
		err                 error
		blockHeight         uint64
		executionHash       hash.Hash256
		refund              uint64
		cachedContract      contractMap
		contractSnapshot    map[int]contractMap   // snapshots of contracts
		suicided            deleteAccount         // account/contract calling Suicide
		suicideSnapshot     map[int]deleteAccount // snapshots of suicide accounts
		preimages           preimageMap
		preimageSnapshot    map[int]preimageMap
		accessList          *accessList // per-transaction access list
		accessListSnapshot  map[int]*accessList
		logsSnapshot        map[int]int // logs is an array, save len(logs) at time of snapshot suffices
		txLogsSnapshot      map[int]int
		notFixTopicCopyBug  bool
		asyncContractTrie   bool
		sortCachedContracts bool
		usePendingNonce     bool
		zeroNonceAccount    bool
		fixSnapshotOrder    bool
		revertLog           bool
	}
)

// StateDBAdapterOption set StateDBAdapter construction param
type StateDBAdapterOption func(*StateDBAdapter) error

// SortCachedContractsOption set sort cached contracts as true
func SortCachedContractsOption() StateDBAdapterOption {
	return func(adapter *StateDBAdapter) error {
		adapter.sortCachedContracts = true
		return nil
	}
}

// NotFixTopicCopyBugOption set notFixTopicCopyBug as true
func NotFixTopicCopyBugOption() StateDBAdapterOption {
	return func(adapter *StateDBAdapter) error {
		adapter.notFixTopicCopyBug = true
		return nil
	}
}

// AsyncContractTrieOption set asyncContractTrie as true
func AsyncContractTrieOption() StateDBAdapterOption {
	return func(adapter *StateDBAdapter) error {
		adapter.asyncContractTrie = true
		return nil
	}
}

// UsePendingNonceOption set usePendingNonce as true
func UsePendingNonceOption() StateDBAdapterOption {
	return func(adapter *StateDBAdapter) error {
		adapter.usePendingNonce = true
		return nil
	}
}

// ZeroNonceAccountOption set fixPendingNonce as true
func ZeroNonceAccountOption() StateDBAdapterOption {
	return func(adapter *StateDBAdapter) error {
		adapter.zeroNonceAccount = true
		return nil
	}
}

// FixSnapshotOrderOption set fixSnapshotOrder as true
func FixSnapshotOrderOption() StateDBAdapterOption {
	return func(adapter *StateDBAdapter) error {
		adapter.fixSnapshotOrder = true
		return nil
	}
}

// RevertLogOption set revertLog as true
func RevertLogOption() StateDBAdapterOption {
	return func(adapter *StateDBAdapter) error {
		adapter.revertLog = true
		return nil
	}
}

// NewStateDBAdapter creates a new state db with iotex blockchain
func NewStateDBAdapter(
	sm protocol.StateManager,
	blockHeight uint64,
	executionHash hash.Hash256,
	opts ...StateDBAdapterOption,
) *StateDBAdapter {
	s := &StateDBAdapter{
		sm:                 sm,
		logs:               []*action.Log{},
		err:                nil,
		blockHeight:        blockHeight,
		executionHash:      executionHash,
		cachedContract:     make(contractMap),
		contractSnapshot:   make(map[int]contractMap),
		suicided:           make(deleteAccount),
		suicideSnapshot:    make(map[int]deleteAccount),
		preimages:          make(preimageMap),
		preimageSnapshot:   make(map[int]preimageMap),
		accessList:         newAccessList(),
		accessListSnapshot: make(map[int]*accessList),
		logsSnapshot:       make(map[int]int),
		txLogsSnapshot:     make(map[int]int),
	}
	for _, opt := range opts {
		if err := opt(s); err != nil {
			log.L().Panic("failed to execute stateDB creation option")
		}
	}
	return s
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

func (stateDB *StateDBAdapter) accountCreationOpts() []state.AccountCreationOption {
	if stateDB.zeroNonceAccount {
		return []state.AccountCreationOption{state.ZeroNonceAccountTypeOption()}
	}
	return nil
}

// CreateAccount creates an account in iotx blockchain
func (stateDB *StateDBAdapter) CreateAccount(evmAddr common.Address) {
	addr, err := address.FromBytes(evmAddr.Bytes())
	if err != nil {
		log.L().Error("Failed to convert evm address.", zap.Error(err))
		return
	}
	_, err = accountutil.LoadOrCreateAccount(stateDB.sm, addr, stateDB.accountCreationOpts()...)
	if err != nil {
		log.L().Error("Failed to create account.", zap.Error(err))
		stateDB.logError(err)
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
	addr, err := address.FromBytes(evmAddr.Bytes())
	if err != nil {
		log.L().Error("Failed to convert evm address.", zap.Error(err))
		return
	}
	state, err := stateDB.AccountState(addr.String())
	if err != nil {
		log.L().Error("Failed to sub balance.", zap.Error(err))
		stateDB.logError(err)
		return
	}
	if err := state.SubBalance(amount); err != nil {
		log.L().Error("Failed to sub balance.", zap.Error(err))
		stateDB.logError(err)
		return
	}
	if err := accountutil.StoreAccount(stateDB.sm, addr, state); err != nil {
		log.L().Error("Failed to update pending account changes to trie.", zap.Error(err))
		stateDB.logError(err)
	}
}

// AddBalance adds balance to account
func (stateDB *StateDBAdapter) AddBalance(evmAddr common.Address, amount *big.Int) {
	if amount.Cmp(big.NewInt(int64(0))) == 0 {
		return
	}
	// stateDB.GetBalance(evmAddr)
	log.L().Debug(fmt.Sprintf("AddBalance %v to %s", amount, evmAddr.Hex()))

	addr, err := address.FromBytes(evmAddr.Bytes())
	if err != nil {
		log.L().Error("Failed to convert evm address.", zap.Error(err))
		return
	}
	var state *state.Account
	addrHash := hash.BytesToHash160(evmAddr[:])
	if contract, ok := stateDB.cachedContract[addrHash]; ok {
		state = contract.SelfState()
	} else {
		state, err = accountutil.LoadOrCreateAccount(stateDB.sm, addr, stateDB.accountCreationOpts()...)
		if err != nil {
			log.L().Error("Failed to add balance.", log.Hex("addrHash", evmAddr[:]))
			stateDB.logError(err)
			return
		}
	}
	if err := state.AddBalance(amount); err != nil {
		log.L().Error("failed to add balance", zap.Error(err), zap.String("amount", amount.String()))
		stateDB.logError(err)
		return
	}
	if err := accountutil.StoreAccount(stateDB.sm, addr, state); err != nil {
		log.L().Error("Failed to update pending account changes to trie.", zap.Error(err))
		stateDB.logError(err)
	}
}

// GetBalance gets the balance of account
func (stateDB *StateDBAdapter) GetBalance(evmAddr common.Address) *big.Int {
	addr, err := address.FromBytes(evmAddr.Bytes())
	if err != nil {
		log.L().Error("Failed to convert evm address.", zap.Error(err))
		return big.NewInt(0)
	}
	state, err := stateDB.AccountState(addr.String())
	if err != nil {
		log.L().Error("Failed to get balance.", zap.Error(err))
		return big.NewInt(0)
	}
	log.L().Debug(fmt.Sprintf("Balance of %s is %v", evmAddr.Hex(), state.Balance))

	return state.Balance
}

// IsNewAccount returns true if this is a new account
func (stateDB *StateDBAdapter) IsNewAccount(evmAddr common.Address) bool {
	addr, err := address.FromBytes(evmAddr.Bytes())
	if err != nil {
		log.L().Error("Failed to convert evm address.", zap.Error(err))
		return false
	}
	state, err := stateDB.AccountState(addr.String())
	if err != nil {
		log.L().Error("failed to load account.", zap.Error(err), zap.String("address", addr.String()))
		return false
	}

	return state.IsNewbieAccount()
}

// GetNonce gets the nonce of account
func (stateDB *StateDBAdapter) GetNonce(evmAddr common.Address) uint64 {
	addr, err := address.FromBytes(evmAddr.Bytes())
	if err != nil {
		log.L().Error("Failed to convert evm address.", zap.Error(err))
		return 0
	}
	var pendingNonce uint64
	if stateDB.zeroNonceAccount {
		pendingNonce = uint64(0)
	} else {
		pendingNonce = uint64(1)
	}
	state, err := stateDB.AccountState(addr.String())
	if err != nil {
		log.L().Error("Failed to get nonce.", zap.Error(err))
		// stateDB.logError(err)
	} else {
		pendingNonce = state.PendingNonce()
	}
	if !stateDB.usePendingNonce {
		if pendingNonce == 0 {
			panic("invalid pending nonce")
		}
		pendingNonce--
	}
	log.L().Debug("Called GetNonce.",
		zap.String("address", addr.String()),
		zap.Uint64("pendingNonce", pendingNonce))

	return pendingNonce
}

// SetNonce sets the nonce of account
func (stateDB *StateDBAdapter) SetNonce(evmAddr common.Address, nonce uint64) {
	addr, err := address.FromBytes(evmAddr.Bytes())
	if err != nil {
		log.L().Error("Failed to convert evm address.", zap.Error(err))
		return
	}
	s, err := stateDB.AccountState(addr.String())
	if err != nil {
		log.L().Error("Failed to set nonce.", zap.Error(err))
		// stateDB.logError(err)
		return
	}
	if stateDB.usePendingNonce {
		if nonce == 0 {
			panic("invalid nonce zero")
		}
		nonce--
	}
	log.L().Debug("Called SetNonce.",
		zap.String("address", addr.String()),
		zap.Uint64("nonce", nonce))
	if err := s.SetNonce(nonce); err != nil {
		log.L().Error("Failed to set nonce.", zap.Error(err))
		stateDB.logError(err)
	}
	if err := accountutil.StoreAccount(stateDB.sm, addr, s); err != nil {
		log.L().Error("Failed to store account.", zap.Error(err))
		stateDB.logError(err)
	}
}

// SubRefund subtracts refund
func (stateDB *StateDBAdapter) SubRefund(gas uint64) {
	log.L().Debug("Called SubRefund.", zap.Uint64("gas", gas))
	// stateDB.journal.append(refundChange{prev: self.refund})
	if gas > stateDB.refund {
		panic("Refund counter not enough!")
	}
	stateDB.refund -= gas
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
	addr, err := address.FromBytes(evmAddr.Bytes())
	if err != nil {
		log.L().Error("Failed to convert evm address.", zap.Error(err))
		return false
	}
	if !stateDB.Exist(evmAddr) {
		log.L().Debug("Account does not exist.", zap.String("address", addr.String()))
		return false
	}
	s, err := stateDB.AccountState(addr.String())
	if err != nil {
		log.L().Debug("Failed to get account.", zap.String("address", addr.String()))
		return false
	}
	// clears the account balance
	if err := s.SubBalance(s.Balance); err != nil {
		log.L().Debug("failed to clear balance", zap.Error(err), zap.String("address", addr.String()))
		return false
	}
	addrHash := hash.BytesToHash160(evmAddr.Bytes())
	if _, err := stateDB.sm.PutState(s, protocol.LegacyKeyOption(addrHash)); err != nil {
		log.L().Error("Failed to kill contract.", zap.Error(err))
		stateDB.logError(err)
		return false
	}
	// mark it as deleted
	stateDB.suicided[addrHash] = struct{}{}
	return true
}

// HasSuicided returns whether the contract has been killed
func (stateDB *StateDBAdapter) HasSuicided(evmAddr common.Address) bool {
	addrHash := hash.BytesToHash160(evmAddr.Bytes())
	_, ok := stateDB.suicided[addrHash]
	return ok
}

// Exist checks the existence of an address
func (stateDB *StateDBAdapter) Exist(evmAddr common.Address) bool {
	addr, err := address.FromBytes(evmAddr.Bytes())
	if err != nil {
		log.L().Error("Failed to convert evm address.", zap.Error(err))
		return false
	}
	log.L().Debug("Check existence.", zap.String("address", addr.String()), log.Hex("addrHash", evmAddr[:]))
	addrHash := hash.BytesToHash160(addr.Bytes())
	if _, ok := stateDB.cachedContract[addrHash]; ok {
		return true
	}
	recorded, err := accountutil.Recorded(stateDB.sm, addr)
	if !recorded || err != nil {
		log.L().Debug("Account does not exist.", zap.String("address", addr.String()))
		return false
	}
	return true
}

// PrepareAccessList handles the preparatory steps for executing a state transition with
// regards to both EIP-2929 and EIP-2930:
//
// - Add sender to access list (2929)
// - Add destination to access list (2929)
// - Add precompiles to access list (2929)
// - Add the contents of the optional tx access list (2930)
//
// This method should only be called if Berlin/2929+2930 is applicable at the current number.
func (stateDB *StateDBAdapter) PrepareAccessList(sender common.Address, dst *common.Address, precompiles []common.Address, list types.AccessList) {
	stateDB.AddAddressToAccessList(sender)
	if dst != nil {
		stateDB.AddAddressToAccessList(*dst)
		// If it's a create-tx, the destination will be added inside evm.create
	}
	for _, addr := range precompiles {
		stateDB.AddAddressToAccessList(addr)
	}
	for _, el := range list {
		stateDB.AddAddressToAccessList(el.Address)
		for _, key := range el.StorageKeys {
			stateDB.AddSlotToAccessList(el.Address, key)
		}
	}
}

// AddressInAccessList returns true if the given address is in the access list
func (stateDB *StateDBAdapter) AddressInAccessList(addr common.Address) bool {
	return stateDB.accessList.ContainsAddress(addr)
}

// SlotInAccessList returns true if the given (address, slot)-tuple is in the access list
func (stateDB *StateDBAdapter) SlotInAccessList(addr common.Address, slot common.Hash) (addressOk bool, slotOk bool) {
	return stateDB.accessList.Contains(addr, slot)
}

// AddAddressToAccessList adds the given address to the access list. This operation is safe to perform
// even if the feature/fork is not active yet
func (stateDB *StateDBAdapter) AddAddressToAccessList(addr common.Address) {
	stateDB.accessList.AddAddress(addr)
}

// AddSlotToAccessList adds the given (address,slot) to the access list. This operation is safe to perform
// even if the feature/fork is not active yet
func (stateDB *StateDBAdapter) AddSlotToAccessList(addr common.Address, slot common.Hash) {
	stateDB.accessList.AddSlot(addr, slot)
}

// Empty returns true if the the contract is empty
func (stateDB *StateDBAdapter) Empty(evmAddr common.Address) bool {
	addr, err := address.FromBytes(evmAddr.Bytes())
	if err != nil {
		log.L().Error("Failed to convert evm address.", zap.Error(err))
		return true
	}
	log.L().Debug("Check whether the contract is empty.")
	s, err := stateDB.AccountState(addr.String())
	if err != nil {
		return true
	}
	// TODO: delete hash.ZeroHash256
	return s.IsNewbieAccount() &&
		s.Balance.Sign() == 0 &&
		(len(s.CodeHash) == 0 || bytes.Equal(s.CodeHash, hash.ZeroHash256[:]))
}

// RevertToSnapshot reverts the state factory to the state at a given snapshot
func (stateDB *StateDBAdapter) RevertToSnapshot(snapshot int) {
	if err := stateDB.sm.Revert(snapshot); err != nil {
		err := errors.New("unexpected error: state manager's Revert() failed")
		log.L().Error("Failed to revert to snapshot.", zap.Error(err))
		stateDB.logError(err)
		return
	}
	ds, ok := stateDB.suicideSnapshot[snapshot]
	if !ok {
		// this should not happen, b/c we save the suicide accounts on a successful return of Snapshot(), but check anyway
		log.L().Error("Failed to get snapshot.", zap.Int("snapshot", snapshot))
		return
	}
	// restore logs and txLogs
	if stateDB.revertLog {
		stateDB.logs = stateDB.logs[:stateDB.logsSnapshot[snapshot]]
		delete(stateDB.logsSnapshot, snapshot)
		for i := snapshot + 1; ; i++ {
			if _, ok := stateDB.logsSnapshot[i]; ok {
				delete(stateDB.logsSnapshot, i)
			} else {
				break
			}
		}
		stateDB.transactionLogs = stateDB.transactionLogs[:stateDB.txLogsSnapshot[snapshot]]
		delete(stateDB.txLogsSnapshot, snapshot)
		for i := snapshot + 1; ; i++ {
			if _, ok := stateDB.txLogsSnapshot[i]; ok {
				delete(stateDB.txLogsSnapshot, i)
			} else {
				break
			}
		}
	}
	// restore the suicide accounts
	stateDB.suicided = nil
	stateDB.suicided = ds
	if stateDB.fixSnapshotOrder {
		delete(stateDB.suicideSnapshot, snapshot)
		for i := snapshot + 1; ; i++ {
			if _, ok := stateDB.suicideSnapshot[i]; ok {
				delete(stateDB.suicideSnapshot, i)
			} else {
				break
			}
		}
	}
	// restore modified contracts
	stateDB.cachedContract = nil
	stateDB.cachedContract = stateDB.contractSnapshot[snapshot]
	for _, addr := range stateDB.cachedContractAddrs() {
		c := stateDB.cachedContract[addr]
		if err := c.LoadRoot(); err != nil {
			log.L().Error("Failed to load root for contract.", zap.Error(err), log.Hex("addrHash", addr[:]))
			return
		}
	}
	if stateDB.fixSnapshotOrder {
		delete(stateDB.contractSnapshot, snapshot)
		for i := snapshot + 1; ; i++ {
			if _, ok := stateDB.contractSnapshot[i]; ok {
				delete(stateDB.contractSnapshot, i)
			} else {
				break
			}
		}
	}
	// restore preimages
	stateDB.preimages = nil
	stateDB.preimages = stateDB.preimageSnapshot[snapshot]
	if stateDB.fixSnapshotOrder {
		delete(stateDB.preimageSnapshot, snapshot)
		for i := snapshot + 1; ; i++ {
			if _, ok := stateDB.preimageSnapshot[i]; ok {
				delete(stateDB.preimageSnapshot, i)
			} else {
				break
			}
		}
	}
	// restore access list
	stateDB.accessList = nil
	stateDB.accessList = stateDB.accessListSnapshot[snapshot]
	{
		delete(stateDB.accessListSnapshot, snapshot)
		for i := snapshot + 1; ; i++ {
			if _, ok := stateDB.accessListSnapshot[i]; ok {
				delete(stateDB.accessListSnapshot, i)
			} else {
				break
			}
		}
	}
}

func (stateDB *StateDBAdapter) cachedContractAddrs() []hash.Hash160 {
	addrs := make([]hash.Hash160, 0, len(stateDB.cachedContract))
	for addr := range stateDB.cachedContract {
		addrs = append(addrs, addr)
	}
	if stateDB.sortCachedContracts {
		sort.Slice(addrs, func(i, j int) bool { return bytes.Compare(addrs[i][:], addrs[j][:]) < 0 })
	}
	return addrs
}

// Snapshot returns the snapshot id
func (stateDB *StateDBAdapter) Snapshot() int {
	// save a copy of modified contracts
	c := make(contractMap)
	if stateDB.fixSnapshotOrder {
		for _, addr := range stateDB.cachedContractAddrs() {
			c[addr] = stateDB.cachedContract[addr].Snapshot()
		}
	}
	sn := stateDB.sm.Snapshot()
	if _, ok := stateDB.suicideSnapshot[sn]; ok {
		err := errors.New("unexpected error: duplicate snapshot version")
		if stateDB.fixSnapshotOrder {
			log.L().Panic("Failed to snapshot.", zap.Error(err))
		} else {
			log.L().Error("Failed to snapshot.", zap.Error(err))
		}
		// stateDB.err = err
		return sn
	}
	// record the current log size
	if stateDB.revertLog {
		stateDB.logsSnapshot[sn] = len(stateDB.logs)
		stateDB.txLogsSnapshot[sn] = len(stateDB.transactionLogs)
	}
	// save a copy of current suicide accounts
	sa := make(deleteAccount)
	for k, v := range stateDB.suicided {
		sa[k] = v
	}
	stateDB.suicideSnapshot[sn] = sa
	if !stateDB.fixSnapshotOrder {
		for _, addr := range stateDB.cachedContractAddrs() {
			c[addr] = stateDB.cachedContract[addr].Snapshot()
		}
	}
	stateDB.contractSnapshot[sn] = c
	// save a copy of preimages
	p := make(preimageMap)
	for k, v := range stateDB.preimages {
		p[k] = v
	}
	stateDB.preimageSnapshot[sn] = p
	// save a copy of access list
	stateDB.accessListSnapshot[sn] = stateDB.accessList.Copy()
	return sn
}

// AddLog adds log whose transaction amount is larger than 0
func (stateDB *StateDBAdapter) AddLog(evmLog *types.Log) {
	log.L().Debug("Called AddLog.", zap.Any("log", evmLog))
	addr, err := address.FromBytes(evmLog.Address.Bytes())
	if err != nil {
		log.L().Error("Failed to convert evm address.", zap.Error(err))
		return
	}
	var topics []hash.Hash256
	for _, evmTopic := range evmLog.Topics {
		var topic hash.Hash256
		copy(topic[:], evmTopic.Bytes())
		topics = append(topics, topic)
	}
	if topics[0] == _inContractTransfer {
		if len(topics) != 3 {
			panic("Invalid in contract transfer topics")
		}
		if amount, zero := new(big.Int).SetBytes(evmLog.Data), big.NewInt(0); amount.Cmp(zero) == 1 {
			from, _ := address.FromBytes(topics[1][12:])
			to, _ := address.FromBytes(topics[2][12:])
			stateDB.transactionLogs = append(stateDB.transactionLogs, &action.TransactionLog{
				Type:      iotextypes.TransactionLogType_IN_CONTRACT_TRANSFER,
				Sender:    from.String(),
				Recipient: to.String(),
				Amount:    amount,
			})
		}
		return
	}

	stateDB.logs = append(stateDB.logs, &action.Log{
		Address:            addr.String(),
		Topics:             topics,
		Data:               evmLog.Data,
		BlockHeight:        stateDB.blockHeight,
		ActionHash:         stateDB.executionHash,
		NotFixTopicCopyBug: stateDB.notFixTopicCopyBug,
	})
}

// Logs returns the logs
func (stateDB *StateDBAdapter) Logs() []*action.Log {
	return stateDB.logs
}

// TransactionLogs returns the transaction logs
func (stateDB *StateDBAdapter) TransactionLogs() []*action.TransactionLog {
	return stateDB.transactionLogs
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
func (stateDB *StateDBAdapter) ForEachStorage(addr common.Address, cb func(common.Hash, common.Hash) bool) error {
	ctt, err := stateDB.getContract(hash.BytesToHash160(addr[:]))
	if err != nil {
		// stateDB.err = err
		return err
	}
	iter, err := ctt.Iterator()
	if err != nil {
		// stateDB.err = err
		return err
	}

	for {
		key, value, err := iter.Next()
		if err == trie.ErrEndOfIterator {
			// hit the end of the iterator, exit now
			return nil
		}
		if err != nil {
			return err
		}
		ckey := common.Hash{}
		copy(ckey[:], key[:])
		cvalue := common.Hash{}
		copy(cvalue[:], value[:])
		if !cb(ckey, cvalue) {
			return nil
		}
	}
	return nil
}

// AccountState returns an account state
func (stateDB *StateDBAdapter) AccountState(encodedAddr string) (*state.Account, error) {
	addr, err := address.FromString(encodedAddr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get public key hash from encoded address")
	}
	addrHash := hash.BytesToHash160(addr.Bytes())
	if contract, ok := stateDB.cachedContract[addrHash]; ok {
		return contract.SelfState(), nil
	}
	return accountutil.LoadAccountByHash160(stateDB.sm, addrHash, stateDB.accountCreationOpts()...)
}

//======================================
// Contract functions
//======================================

// GetCodeHash returns contract's code hash
func (stateDB *StateDBAdapter) GetCodeHash(evmAddr common.Address) common.Hash {
	addr := hash.BytesToHash160(evmAddr[:])
	codeHash := common.Hash{}
	if contract, ok := stateDB.cachedContract[addr]; ok {
		copy(codeHash[:], contract.SelfState().CodeHash)
		return codeHash
	}
	account, err := accountutil.LoadAccountByHash160(stateDB.sm, addr, stateDB.accountCreationOpts()...)
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
	addr := hash.BytesToHash160(evmAddr[:])
	if contract, ok := stateDB.cachedContract[addr]; ok {
		code, err := contract.GetCode()
		if err != nil {
			log.L().Error("Failed to get code hash.", zap.Error(err))
			return nil
		}
		return code
	}
	account, err := accountutil.LoadAccountByHash160(stateDB.sm, addr, stateDB.accountCreationOpts()...)
	if err != nil {
		log.L().Error("Failed to load account state for address.", log.Hex("addrHash", addr[:]))
		return nil
	}
	var code protocol.SerializableBytes
	if _, err = stateDB.sm.State(&code, protocol.NamespaceOption(CodeKVNameSpace), protocol.KeyOption(account.CodeHash[:])); err != nil {
		// TODO: Suppress the as it's too much now
		//log.L().Error("Failed to get code from trie.", zap.Error(err))
		return nil
	}
	return code[:]
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
	addr := hash.BytesToHash160(evmAddr[:])
	contract, err := stateDB.getContract(addr)
	if err != nil {
		log.L().Error("Failed to get contract.", zap.Error(err), log.Hex("addrHash", addr[:]))
		stateDB.logError(err)
		return
	}
	contract.SetCode(hash.Hash256b(code), code)
}

// GetCommittedState gets committed state
func (stateDB *StateDBAdapter) GetCommittedState(evmAddr common.Address, k common.Hash) common.Hash {
	addr := hash.BytesToHash160(evmAddr[:])
	contract, err := stateDB.getContract(addr)
	if err != nil {
		log.L().Error("Failed to get contract.", zap.Error(err), log.Hex("addrHash", addr[:]))
		stateDB.logError(err)
		return common.Hash{}
	}
	v, err := contract.GetCommittedState(hash.BytesToHash256(k[:]))
	if err != nil {
		log.L().Debug("Failed to get committed state.", zap.Error(err))
		stateDB.logError(err)
		return common.Hash{}
	}
	return common.BytesToHash(v)
}

// GetState gets state
func (stateDB *StateDBAdapter) GetState(evmAddr common.Address, k common.Hash) common.Hash {
	addr := hash.BytesToHash160(evmAddr[:])
	contract, err := stateDB.getContract(addr)
	if err != nil {
		log.L().Error("Failed to get contract.", zap.Error(err), log.Hex("addrHash", addr[:]))
		stateDB.logError(err)
		return common.Hash{}
	}
	v, err := contract.GetState(hash.BytesToHash256(k[:]))
	if err != nil {
		log.L().Debug("Failed to get state.", zap.Error(err))
		stateDB.logError(err)
		return common.Hash{}
	}
	return common.BytesToHash(v)
}

// SetState sets state
func (stateDB *StateDBAdapter) SetState(evmAddr common.Address, k, v common.Hash) {
	addr := hash.BytesToHash160(evmAddr[:])
	contract, err := stateDB.getContract(addr)
	if err != nil {
		log.L().Error("Failed to get contract.", zap.Error(err), log.Hex("addrHash", addr[:]))
		stateDB.logError(err)
		return
	}
	log.L().Debug("Called SetState", log.Hex("addrHash", evmAddr[:]), log.Hex("k", k[:]))
	if err := contract.SetState(hash.BytesToHash256(k[:]), v[:]); err != nil {
		log.L().Error("Failed to set state.", zap.Error(err), log.Hex("addrHash", addr[:]))
		stateDB.logError(err)
		return
	}
}

// CommitContracts commits contract code to db and update pending contract account changes to trie
func (stateDB *StateDBAdapter) CommitContracts() error {
	addrStrs := make([]string, 0)
	for addr := range stateDB.cachedContract {
		addrStrs = append(addrStrs, hex.EncodeToString(addr[:]))
	}
	sort.Strings(addrStrs)

	for _, addrStr := range addrStrs {
		var addr hash.Hash160
		addrBytes, err := hex.DecodeString(addrStr)
		if err != nil {
			return errors.Wrap(err, "failed to decode address hash")
		}
		copy(addr[:], addrBytes)
		if _, ok := stateDB.suicided[addr]; ok {
			// no need to update a suicide account/contract
			continue
		}
		contract := stateDB.cachedContract[addr]
		if err := contract.Commit(); err != nil {
			stateDB.logError(err)
			return errors.Wrap(err, "failed to commit contract")
		}
		state := contract.SelfState()
		// store the account (with new storage trie root) into account trie
		if _, err := stateDB.sm.PutState(state, protocol.LegacyKeyOption(addr)); err != nil {
			stateDB.logError(err)
			return errors.Wrap(err, "failed to update pending account changes to trie")
		}
	}
	// delete suicided accounts/contract
	addrStrs = make([]string, 0)
	for addr := range stateDB.suicided {
		addrStrs = append(addrStrs, hex.EncodeToString(addr[:]))
	}
	sort.Strings(addrStrs)

	for _, addrStr := range addrStrs {
		var addr hash.Hash160
		addrBytes, err := hex.DecodeString(addrStr)
		if err != nil {
			return errors.Wrap(err, "failed to decode address hash")
		}
		copy(addr[:], addrBytes)
		if _, err := stateDB.sm.DelState(protocol.LegacyKeyOption(addr)); err != nil {
			stateDB.logError(err)
			return errors.Wrapf(err, "failed to delete suicide account/contract %x", addr[:])
		}
	}
	// write preimages to DB
	addrStrs = make([]string, 0)
	for addr := range stateDB.preimages {
		addrStrs = append(addrStrs, hex.EncodeToString(addr[:]))
	}
	sort.Strings(addrStrs)

	for _, addrStr := range addrStrs {
		var k common.Hash
		addrBytes, err := hex.DecodeString(addrStr)
		if err != nil {
			return errors.Wrap(err, "failed to decode address hash")
		}
		copy(k[:], addrBytes)
		v := stateDB.preimages[k]
		h := make([]byte, len(k))
		copy(h, k[:])
		stateDB.sm.PutState(v, protocol.NamespaceOption(PreimageKVNameSpace), protocol.KeyOption(h))
	}
	return nil
}

// getContract returns the contract of addr
func (stateDB *StateDBAdapter) getContract(addr hash.Hash160) (Contract, error) {
	if contract, ok := stateDB.cachedContract[addr]; ok {
		return contract, nil
	}
	return stateDB.getNewContract(addr)
}

func (stateDB *StateDBAdapter) getNewContract(addr hash.Hash160) (Contract, error) {
	account, err := accountutil.LoadAccountByHash160(stateDB.sm, addr, stateDB.accountCreationOpts()...)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load account state for address %x", addr)
	}
	contract, err := newContract(addr, account, stateDB.sm, stateDB.asyncContractTrie)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create storage trie for new contract %x", addr)
	}
	// add to contract cache
	stateDB.cachedContract[addr] = contract
	return contract, nil
}

// clear clears local changes
func (stateDB *StateDBAdapter) clear() {
	stateDB.cachedContract = make(contractMap)
	stateDB.contractSnapshot = make(map[int]contractMap)
	stateDB.suicided = make(deleteAccount)
	stateDB.suicideSnapshot = make(map[int]deleteAccount)
	stateDB.preimages = make(preimageMap)
	stateDB.preimageSnapshot = make(map[int]preimageMap)
	stateDB.accessList = newAccessList()
	stateDB.accessListSnapshot = make(map[int]*accessList)
	stateDB.logsSnapshot = make(map[int]int)
	stateDB.txLogsSnapshot = make(map[int]int)
	stateDB.logs = []*action.Log{}
	stateDB.transactionLogs = []*action.TransactionLog{}
}
