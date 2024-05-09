// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package evm

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/big"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
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
	deleteAccount map[common.Address]struct{}

	// contractMap records the contracts being changed
	contractMap map[common.Address]Contract

	// preimageMap records the preimage of hash reported by VM
	preimageMap map[common.Hash]protocol.SerializableBytes

	// StateDBAdapter represents the state db adapter for evm to access iotx blockchain
	StateDBAdapter struct {
		sm                     protocol.StateManager
		logs                   []*action.Log
		transactionLogs        []*action.TransactionLog
		err                    error
		blockHeight            uint64
		executionHash          hash.Hash256
		lastAddBalanceAddr     string
		lastAddBalanceAmount   *big.Int
		refund                 uint64
		refundSnapshot         map[int]uint64
		cachedContract         contractMap
		contractSnapshot       map[int]contractMap   // snapshots of contracts
		selfDestructed         deleteAccount         // account/contract calling SelfDestruct
		selfDestructedSnapshot map[int]deleteAccount // snapshots of SelfDestruct accounts
		preimages              preimageMap
		preimageSnapshot       map[int]preimageMap
		accessList             *accessList // per-transaction access list
		accessListSnapshot     map[int]*accessList
		// Transient storage
		transientStorage           transientStorage
		transientStorageSnapshot   map[int]transientStorage
		logsSnapshot               map[int]int // logs is an array, save len(logs) at time of snapshot suffices
		txLogsSnapshot             map[int]int
		notFixTopicCopyBug         bool
		asyncContractTrie          bool
		disableSortCachedContracts bool
		useConfirmedNonce          bool
		legacyNonceAccount         bool
		fixSnapshotOrder           bool
		revertLog                  bool
		manualCorrectGasRefund     bool
		suicideTxLogMismatchPanic  bool
		zeroNonceForFreshAccount   bool
		panicUnrecoverableError    bool
	}
)

// StateDBAdapterOption set StateDBAdapter construction param
type StateDBAdapterOption func(*StateDBAdapter) error

// DisableSortCachedContractsOption set disable sort cached contracts as true
func DisableSortCachedContractsOption() StateDBAdapterOption {
	return func(adapter *StateDBAdapter) error {
		adapter.disableSortCachedContracts = true
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

// UseConfirmedNonceOption set usePendingNonce as true
func UseConfirmedNonceOption() StateDBAdapterOption {
	return func(adapter *StateDBAdapter) error {
		adapter.useConfirmedNonce = true
		return nil
	}
}

// LegacyNonceAccountOption set legacyNonceAccount as true
func LegacyNonceAccountOption() StateDBAdapterOption {
	return func(adapter *StateDBAdapter) error {
		adapter.legacyNonceAccount = true
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

// ManualCorrectGasRefundOption set manualCorrectGasRefund as true
func ManualCorrectGasRefundOption() StateDBAdapterOption {
	return func(adapter *StateDBAdapter) error {
		// before London EVM activation (at Okhotsk height), in certain cases dynamicGas
		// has caused gas refund to change, which needs to be manually adjusted after
		// the tx is reverted. After Okhotsk height, it is fixed inside RevertToSnapshot()
		adapter.manualCorrectGasRefund = true
		return nil
	}
}

// SuicideTxLogMismatchPanicOption set suicideTxLogMismatchPanic as true
func SuicideTxLogMismatchPanicOption() StateDBAdapterOption {
	return func(adapter *StateDBAdapter) error {
		adapter.suicideTxLogMismatchPanic = true
		return nil
	}
}

// ZeroNonceForFreshAccountOption set zeroNonceForFreshAccount as true
func ZeroNonceForFreshAccountOption() StateDBAdapterOption {
	return func(adapter *StateDBAdapter) error {
		adapter.zeroNonceForFreshAccount = true
		return nil
	}
}

// PanicUnrecoverableErrorOption set panicUnrecoverableError as true
func PanicUnrecoverableErrorOption() StateDBAdapterOption {
	return func(adapter *StateDBAdapter) error {
		adapter.panicUnrecoverableError = true
		return nil
	}
}

// NewStateDBAdapter creates a new state db with iotex blockchain
func NewStateDBAdapter(
	sm protocol.StateManager,
	blockHeight uint64,
	executionHash hash.Hash256,
	opts ...StateDBAdapterOption,
) (*StateDBAdapter, error) {
	s := &StateDBAdapter{
		sm:                       sm,
		logs:                     []*action.Log{},
		err:                      nil,
		blockHeight:              blockHeight,
		executionHash:            executionHash,
		lastAddBalanceAmount:     new(big.Int),
		refundSnapshot:           make(map[int]uint64),
		cachedContract:           make(contractMap),
		contractSnapshot:         make(map[int]contractMap),
		selfDestructed:           make(deleteAccount),
		selfDestructedSnapshot:   make(map[int]deleteAccount),
		preimages:                make(preimageMap),
		preimageSnapshot:         make(map[int]preimageMap),
		accessList:               newAccessList(),
		accessListSnapshot:       make(map[int]*accessList),
		transientStorage:         newTransientStorage(),
		transientStorageSnapshot: make(map[int]transientStorage),
		logsSnapshot:             make(map[int]int),
		txLogsSnapshot:           make(map[int]int),
	}
	for _, opt := range opts {
		if err := opt(s); err != nil {
			return nil, errors.Wrap(err, "failed to execute stateDB creation option")
		}
	}
	// TODO: add combination limitation for useZeroNonceForFreshAccount
	if !s.legacyNonceAccount && s.useConfirmedNonce {
		return nil, errors.New("invalid parameter combination")
	}
	return s, nil
}

func (stateDB *StateDBAdapter) logError(err error) {
	if stateDB.err == nil {
		stateDB.err = err
	}
}

func (stateDB *StateDBAdapter) assertError(err error, msg string, fields ...zap.Field) bool {
	if err == nil {
		return false
	}
	if stateDB.panicUnrecoverableError {
		log.L().Panic(msg, fields...)
	}
	log.L().Error(msg, fields...)
	stateDB.logError(err)
	return true
}

// Error returns the first stored error during evm contract execution
func (stateDB *StateDBAdapter) Error() error {
	return stateDB.err
}

func (stateDB *StateDBAdapter) accountCreationOpts() []state.AccountCreationOption {
	if stateDB.legacyNonceAccount {
		return []state.AccountCreationOption{state.LegacyNonceAccountTypeOption()}
	}
	return nil
}

// CreateAccount creates an account in iotx blockchain
func (stateDB *StateDBAdapter) CreateAccount(evmAddr common.Address) {
	addr, err := address.FromBytes(evmAddr.Bytes())
	if stateDB.assertError(err, "Failed to convert evm address.", zap.Error(err)) {
		return
	}
	_, err = accountutil.LoadOrCreateAccount(stateDB.sm, addr, stateDB.accountCreationOpts()...)
	if stateDB.assertError(err, "Failed to create account.", zap.Error(err), zap.String("address", evmAddr.Hex())) {
		return
	}
	log.L().Debug("Called CreateAccount.", log.Hex("addrHash", evmAddr[:]))
}

// SubBalance subtracts balance from account
func (stateDB *StateDBAdapter) SubBalance(evmAddr common.Address, a256 *uint256.Int) {
	amount := a256.ToBig()
	if amount.Cmp(big.NewInt(int64(0))) == 0 {
		return
	}
	// stateDB.GetBalance(evmAddr)
	log.L().Debug(fmt.Sprintf("SubBalance %v from %s", amount, evmAddr.Hex()))
	addr, err := address.FromBytes(evmAddr.Bytes())
	if stateDB.assertError(err, "Failed to convert evm address.", zap.Error(err)) {
		return
	}
	state, err := stateDB.accountState(evmAddr)
	if stateDB.assertError(err, "Failed to get account.", zap.Error(err), zap.String("address", evmAddr.Hex())) {
		return
	}
	err = state.SubBalance(amount)
	if stateDB.assertError(err, "Failed to sub balance.", zap.Error(err), zap.String("amount", amount.String())) {
		return
	}
	err = accountutil.StoreAccount(stateDB.sm, addr, state)
	if stateDB.assertError(err, "Failed to store account.", zap.Error(err), zap.String("address", evmAddr.Hex())) {
		return
	}
}

// AddBalance adds balance to account
func (stateDB *StateDBAdapter) AddBalance(evmAddr common.Address, a256 *uint256.Int) {
	amount := a256.ToBig()
	stateDB.lastAddBalanceAmount.SetUint64(0)
	if amount.Cmp(big.NewInt(int64(0))) == 0 {
		return
	}
	// stateDB.GetBalance(evmAddr)
	log.L().Debug(fmt.Sprintf("AddBalance %v to %s", amount, evmAddr.Hex()))

	addr, err := address.FromBytes(evmAddr.Bytes())
	if stateDB.assertError(err, "Failed to convert evm address.", zap.Error(err)) {
		return
	}
	var (
		state *state.Account
	)
	if contract, ok := stateDB.cachedContract[evmAddr]; ok {
		state = contract.SelfState()
	} else {
		state, err = accountutil.LoadOrCreateAccount(stateDB.sm, addr, stateDB.accountCreationOpts()...)
		if stateDB.assertError(err, "Failed to get account.", zap.Error(err), zap.String("address", evmAddr.Hex())) {
			return
		}
	}
	err = state.AddBalance(amount)
	if stateDB.assertError(err, "Failed to add balance.", zap.Error(err), zap.String("amount", amount.String())) {
		return
	}
	err = accountutil.StoreAccount(stateDB.sm, addr, state)
	if stateDB.assertError(err, "Failed to store account.", zap.Error(err), zap.String("address", evmAddr.Hex())) {
		return
	} else {
		// keep a record of latest add balance
		stateDB.lastAddBalanceAddr = addr.String()
		stateDB.lastAddBalanceAmount.SetBytes(amount.Bytes())
	}
}

// GetBalance gets the balance of account
func (stateDB *StateDBAdapter) GetBalance(evmAddr common.Address) *uint256.Int {
	state, err := stateDB.accountState(evmAddr)
	if stateDB.assertError(err, "Failed to get balance.", zap.Error(err), zap.String("address", evmAddr.Hex())) {
		return common.U2560
	}
	log.L().Debug(fmt.Sprintf("Balance of %s is %v", evmAddr.Hex(), state.Balance))

	return uint256.MustFromBig(state.Balance)
}

// IsNewAccount returns true if this is a new account
func (stateDB *StateDBAdapter) IsNewAccount(evmAddr common.Address) bool {
	state, err := stateDB.accountState(evmAddr)
	if stateDB.assertError(err, "Failed to get account.", zap.Error(err), zap.String("address", evmAddr.Hex())) {
		return false
	}

	return state.IsNewbieAccount()
}

// GetNonce gets the nonce of account
func (stateDB *StateDBAdapter) GetNonce(evmAddr common.Address) uint64 {
	var pendingNonce uint64
	if stateDB.legacyNonceAccount {
		pendingNonce = uint64(1)
	} else {
		pendingNonce = uint64(0)
	}
	state, err := stateDB.accountState(evmAddr)
	if err != nil {
		if stateDB.panicUnrecoverableError {
			log.L().Panic("Failed to get nonce.", zap.Error(err))
		} else {
			log.L().Error("Failed to get nonce.", zap.Error(err))
			stateDB.logError(err)
		}
	} else {
		if stateDB.zeroNonceForFreshAccount {
			pendingNonce = state.PendingNonceConsideringFreshAccount()
		} else {
			pendingNonce = state.PendingNonce()
		}
	}
	if stateDB.useConfirmedNonce {
		if pendingNonce == 0 {
			panic("invalid pending nonce")
		}
		pendingNonce--
	}
	log.L().Debug("Called GetNonce.",
		zap.String("address", evmAddr.Hex()),
		zap.Uint64("pendingNonce", pendingNonce))

	return pendingNonce
}

// SetNonce sets the nonce of account
func (stateDB *StateDBAdapter) SetNonce(evmAddr common.Address, nonce uint64) {
	addr, err := address.FromBytes(evmAddr.Bytes())
	if stateDB.assertError(err, "Failed to convert evm address.", zap.Error(err)) {
		return
	}
	s, err := stateDB.accountState(evmAddr)
	if stateDB.assertError(err, "Failed to get account.", zap.Error(err), zap.String("address", evmAddr.Hex())) {
		return
	}
	if !stateDB.useConfirmedNonce {
		if nonce == 0 {
			panic("invalid nonce zero")
		}
		nonce--
	}
	log.L().Debug("Called SetNonce.",
		zap.String("address", addr.String()),
		zap.Uint64("nonce", nonce))
	if !s.IsNewbieAccount() || s.AccountType() != 0 || nonce != 0 || stateDB.zeroNonceForFreshAccount {
		if err := s.SetPendingNonce(nonce + 1); err != nil {
			log.L().Panic("Failed to set nonce.", zap.Error(err), zap.String("addr", addr.Hex()), zap.Uint64("pendingNonce", s.PendingNonce()), zap.Uint64("nonce", nonce), zap.String("execution", hex.EncodeToString(stateDB.executionHash[:])))
			stateDB.logError(err)
		}
	}
	err = accountutil.StoreAccount(stateDB.sm, addr, s)
	stateDB.assertError(err, "Failed to store account.", zap.Error(err), zap.String("address", evmAddr.Hex()))
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

// SelfDestruct kills the contract
func (stateDB *StateDBAdapter) SelfDestruct(evmAddr common.Address) {
	if !stateDB.Exist(evmAddr) {
		log.L().Debug("Account does not exist.", zap.String("address", evmAddr.Hex()))
		return
	}
	s, err := stateDB.accountState(evmAddr)
	if stateDB.assertError(err, "Failed to get account.", zap.Error(err), zap.String("address", evmAddr.Hex())) {
		return
	}
	// clears the account balance
	actBalance := new(big.Int).Set(s.Balance)
	err = s.SubBalance(s.Balance)
	if stateDB.assertError(err, "Failed to clear balance.", zap.Error(err), zap.String("address", evmAddr.Hex())) {
		return
	}
	_, err = stateDB.sm.PutState(s, protocol.KeyOption(evmAddr[:]))
	if stateDB.assertError(err, "Failed to kill contract.", zap.Error(err), zap.String("address", evmAddr.Hex())) {
		return
	}
	// To ensure data consistency, generate this log after the hard-fork
	// a separate patch file will be created later to provide missing logs before the hard-fork
	// TODO: remove this gating once the hard-fork has passed
	if stateDB.suicideTxLogMismatchPanic {
		// before calling SelfDestruct, EVM will transfer the contract's balance to beneficiary
		// need to create a transaction log on successful SelfDestruct
		if stateDB.lastAddBalanceAmount.Cmp(actBalance) == 0 {
			if stateDB.lastAddBalanceAmount.Cmp(big.NewInt(0)) > 0 {
				from, _ := address.FromBytes(evmAddr[:])
				stateDB.addTransactionLogs(&action.TransactionLog{
					Type:      iotextypes.TransactionLogType_IN_CONTRACT_TRANSFER,
					Sender:    from.String(),
					Recipient: stateDB.lastAddBalanceAddr,
					Amount:    stateDB.lastAddBalanceAmount,
				})
			}
		} else {
			log.L().Panic("SelfDestruct contract's balance does not match",
				zap.String("SelfDestruct", actBalance.String()),
				zap.String("beneficiary", stateDB.lastAddBalanceAmount.String()))
		}
	}
	// mark it as deleted
	stateDB.selfDestructed[evmAddr] = struct{}{}
}

// HasSelfDestructed returns whether the contract has been killed
func (stateDB *StateDBAdapter) HasSelfDestructed(evmAddr common.Address) bool {
	_, ok := stateDB.selfDestructed[evmAddr]
	return ok
}

// SetTransientState sets transient storage for a given account
func (stateDB *StateDBAdapter) SetTransientState(addr common.Address, key, value common.Hash) {
	prev := stateDB.transientStorage.Get(addr, key)
	if prev == value {
		return
	}
	stateDB.transientStorage.Set(addr, key, value)
}

// GetTransientState gets transient storage for a given account.
func (stateDB *StateDBAdapter) GetTransientState(addr common.Address, key common.Hash) common.Hash {
	return stateDB.transientStorage.Get(addr, key)
}

// Selfdestruct6780 implements EIP-6780
func (stateDB *StateDBAdapter) Selfdestruct6780(evmAddr common.Address) {
	//Todo: implement EIP-6780
	log.S().Panic("Selfdestruct6780 not implemented")
}

// Exist checks the existence of an address
func (stateDB *StateDBAdapter) Exist(evmAddr common.Address) bool {
	addr, err := address.FromBytes(evmAddr.Bytes())
	if stateDB.assertError(err, "Failed to convert evm address.", zap.Error(err)) {
		return false
	}
	log.L().Debug("Check existence.", zap.String("address", addr.String()), log.Hex("addrHash", evmAddr[:]))
	if _, ok := stateDB.cachedContract[evmAddr]; ok {
		return true
	}
	recorded, err := accountutil.Recorded(stateDB.sm, addr)
	if stateDB.assertError(err, "Account does not exist.", zap.Error(err), zap.String("address", evmAddr.Hex())) {
		return false
	}
	if !recorded {
		log.L().Debug("Account does not exist.", zap.String("address", addr.String()))
		return false
	}
	return true
}

// Prepare handles the preparatory steps for executing a state transition with.
// This method must be invoked before state transition.
//
// Berlin fork:
// - Add sender to access list (2929)
// - Add destination to access list (2929)
// - Add precompiles to access list (2929)
// - Add the contents of the optional tx access list (2930)
//
// Potential EIPs:
// - Reset access list (Berlin)
// - Add coinbase to access list (EIP-3651)
// - Reset transient storage (EIP-1153)
func (stateDB *StateDBAdapter) Prepare(rules params.Rules, sender, coinbase common.Address, dst *common.Address, precompiles []common.Address, list types.AccessList) {
	if !rules.IsBerlin {
		return
	}
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
	if rules.IsShanghai { // EIP-3651: warm coinbase
		stateDB.AddAddressToAccessList(coinbase)
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
	log.L().Debug("Check whether the contract is empty.")
	s, err := stateDB.accountState(evmAddr)
	if stateDB.assertError(err, "Failed to get account.", zap.Error(err), zap.String("address", evmAddr.Hex())) {
		return true
	}
	// TODO: delete hash.ZeroHash256
	return s.IsNewbieAccount() &&
		s.Balance.Sign() == 0 &&
		(len(s.CodeHash) == 0 || bytes.Equal(s.CodeHash, hash.ZeroHash256[:]))
}

// RevertToSnapshot reverts the state factory to the state at a given snapshot
func (stateDB *StateDBAdapter) RevertToSnapshot(snapshot int) {
	ds, ok := stateDB.selfDestructedSnapshot[snapshot]
	if !ok && stateDB.panicUnrecoverableError {
		log.L().Panic("Failed to revert to snapshot.", zap.Int("snapshot", snapshot))
	}
	err := stateDB.sm.Revert(snapshot)
	if stateDB.assertError(err, "state manager's Revert() failed.", zap.Error(err), zap.Int("snapshot", snapshot)) {
		return
	}
	if !ok {
		// this should not happen, b/c we save the SelfDestruct accounts on a successful return of Snapshot(), but check anyway
		log.L().Error("Failed to revert to snapshot.", zap.Int("snapshot", snapshot))
		return
	}
	// restore gas refund
	if !stateDB.manualCorrectGasRefund {
		stateDB.refund = stateDB.refundSnapshot[snapshot]
		for i := snapshot; ; i++ {
			if _, ok := stateDB.refundSnapshot[i]; ok {
				delete(stateDB.refundSnapshot, i)
			} else {
				break
			}
		}
	}
	// restore access list
	stateDB.accessList = stateDB.accessListSnapshot[snapshot]
	{
		for i := snapshot; ; i++ {
			if _, ok := stateDB.accessListSnapshot[i]; ok {
				delete(stateDB.accessListSnapshot, i)
			} else {
				break
			}
		}
	}
	//restore transientStorage
	stateDB.transientStorage = stateDB.transientStorageSnapshot[snapshot]
	{
		for i := snapshot; ; i++ {
			if _, ok := stateDB.transientStorageSnapshot[i]; ok {
				delete(stateDB.transientStorageSnapshot, i)
			} else {
				break
			}
		}
	}
	// restore logs and txLogs
	if stateDB.revertLog {
		stateDB.logs = stateDB.logs[:stateDB.logsSnapshot[snapshot]]
		for i := snapshot; ; i++ {
			if _, ok := stateDB.logsSnapshot[i]; ok {
				delete(stateDB.logsSnapshot, i)
			} else {
				break
			}
		}
		stateDB.transactionLogs = stateDB.transactionLogs[:stateDB.txLogsSnapshot[snapshot]]
		for i := snapshot; ; i++ {
			if _, ok := stateDB.txLogsSnapshot[i]; ok {
				delete(stateDB.txLogsSnapshot, i)
			} else {
				break
			}
		}
	}
	// restore the SelfDestruct accounts
	stateDB.selfDestructed = ds
	if stateDB.fixSnapshotOrder {
		for i := snapshot; ; i++ {
			if _, ok := stateDB.selfDestructedSnapshot[i]; ok {
				delete(stateDB.selfDestructedSnapshot, i)
			} else {
				break
			}
		}
	}
	// restore modified contracts
	stateDB.cachedContract = stateDB.contractSnapshot[snapshot]
	for _, addr := range stateDB.cachedContractAddrs() {
		c := stateDB.cachedContract[addr]
		if err := c.LoadRoot(); err != nil {
			log.L().Debug("Failed to load root for contract.", zap.Error(err), log.Hex("addrHash", addr[:]))
			return
		}
	}
	if stateDB.fixSnapshotOrder {
		for i := snapshot; ; i++ {
			if _, ok := stateDB.contractSnapshot[i]; ok {
				delete(stateDB.contractSnapshot, i)
			} else {
				break
			}
		}
	}
	// restore preimages
	stateDB.preimages = stateDB.preimageSnapshot[snapshot]
	if stateDB.fixSnapshotOrder {
		for i := snapshot; ; i++ {
			if _, ok := stateDB.preimageSnapshot[i]; ok {
				delete(stateDB.preimageSnapshot, i)
			} else {
				break
			}
		}
	}
}

func (stateDB *StateDBAdapter) cachedContractAddrs() []common.Address {
	addrs := make([]common.Address, 0, len(stateDB.cachedContract))
	for addr := range stateDB.cachedContract {
		addrs = append(addrs, addr)
	}
	if !stateDB.disableSortCachedContracts {
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
	if _, ok := stateDB.selfDestructedSnapshot[sn]; ok {
		err := errors.New("unexpected error: duplicate snapshot version")
		if stateDB.fixSnapshotOrder {
			log.L().Panic("Failed to snapshot.", zap.Error(err))
		} else {
			log.L().Debug("Failed to snapshot.", zap.Error(err))
		}
		// stateDB.err = err
		return sn
	}
	// record the current gas refund
	stateDB.refundSnapshot[sn] = stateDB.refund
	// record the current log size
	if stateDB.revertLog {
		stateDB.logsSnapshot[sn] = len(stateDB.logs)
		stateDB.txLogsSnapshot[sn] = len(stateDB.transactionLogs)
	}
	// save a copy of current SelfDestruct accounts
	sa := make(deleteAccount)
	for k, v := range stateDB.selfDestructed {
		sa[k] = v
	}
	stateDB.selfDestructedSnapshot[sn] = sa
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
	// save a copy of transient storage
	stateDB.transientStorageSnapshot[sn] = stateDB.transientStorage.Copy()
	return sn
}

// AddLog adds log whose transaction amount is larger than 0
func (stateDB *StateDBAdapter) AddLog(evmLog *types.Log) {
	log.L().Debug("Called AddLog.", zap.Any("log", evmLog))
	addr, err := address.FromBytes(evmLog.Address.Bytes())
	if stateDB.assertError(err, "Failed to convert evm address.", zap.Error(err)) {
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
			stateDB.addTransactionLogs(&action.TransactionLog{
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
	ctt, err := stateDB.getContract(addr)
	if stateDB.assertError(err, "Failed to get contract.", zap.Error(err), zap.String("address", addr.Hex())) {
		return err
	}
	iter, err := ctt.Iterator()
	if stateDB.assertError(err, "Failed to get Iterator.", zap.Error(err), zap.String("address", addr.Hex())) {
		return err
	}

	for {
		key, value, err := iter.Next()
		if err == trie.ErrEndOfIterator {
			// hit the end of the iterator, exit now
			return nil
		}
		if stateDB.assertError(err, "Failed to get next storage.", zap.Error(err), zap.String("address", addr.Hex())) {
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
}

// accountState returns an account state
func (stateDB *StateDBAdapter) accountState(evmAddr common.Address) (*state.Account, error) {
	if contract, ok := stateDB.cachedContract[evmAddr]; ok {
		return contract.SelfState(), nil
	}
	return accountutil.LoadAccountByHash160(stateDB.sm, hash.BytesToHash160(evmAddr[:]), stateDB.accountCreationOpts()...)
}

func (stateDB *StateDBAdapter) addTransactionLogs(tlog *action.TransactionLog) {
	stateDB.transactionLogs = append(stateDB.transactionLogs, tlog)
}

//======================================
// Contract functions
//======================================

// GetCodeHash returns contract's code hash
func (stateDB *StateDBAdapter) GetCodeHash(evmAddr common.Address) common.Hash {
	codeHash := common.Hash{}
	if contract, ok := stateDB.cachedContract[evmAddr]; ok {
		copy(codeHash[:], contract.SelfState().CodeHash)
		return codeHash
	}
	account, err := accountutil.LoadAccountByHash160(stateDB.sm, hash.BytesToHash160(evmAddr[:]), stateDB.accountCreationOpts()...)
	if stateDB.assertError(err, "Failed to load account.", zap.Error(err), zap.String("address", evmAddr.Hex())) {
		return codeHash
	}
	copy(codeHash[:], account.CodeHash)
	return codeHash
}

// GetCode returns contract's code
func (stateDB *StateDBAdapter) GetCode(evmAddr common.Address) []byte {
	if contract, ok := stateDB.cachedContract[evmAddr]; ok {
		code, err := contract.GetCode()
		if err != nil {
			log.L().Debug("Failed to get code hash.", zap.Error(err))
			return nil
		}
		return code
	}
	account, err := accountutil.LoadAccountByHash160(stateDB.sm, hash.BytesToHash160(evmAddr[:]), stateDB.accountCreationOpts()...)
	if stateDB.assertError(err, "Failed to load account.", zap.Error(err), zap.String("address", evmAddr.Hex())) {
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
	log.L().Debug("Called GetCodeSize.", log.Hex("addrHash", evmAddr[:]))
	return len(code)
}

// SetCode sets contract's code
func (stateDB *StateDBAdapter) SetCode(evmAddr common.Address, code []byte) {
	contract, err := stateDB.getContract(evmAddr)
	if err != nil {
		log.L().Error("Failed to get contract.", zap.Error(err), zap.String("address", evmAddr.Hex()))
		stateDB.logError(err)
		return
	}
	contract.SetCode(hash.Hash256b(code), code)
}

// GetCommittedState gets committed state
func (stateDB *StateDBAdapter) GetCommittedState(evmAddr common.Address, k common.Hash) common.Hash {
	contract, err := stateDB.getContract(evmAddr)
	if err != nil {
		log.L().Debug("Failed to get contract.", zap.Error(err), zap.String("address", evmAddr.Hex()))
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
	contract, err := stateDB.getContract(evmAddr)
	if err != nil {
		log.L().Debug("Failed to get contract.", zap.Error(err), zap.String("address", evmAddr.Hex()))
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
	contract, err := stateDB.getContract(evmAddr)
	if err != nil {
		log.L().Debug("Failed to get contract.", zap.Error(err), zap.String("address", evmAddr.Hex()))
		stateDB.logError(err)
		return
	}
	log.L().Debug("Called SetState", log.Hex("addrHash", evmAddr[:]), log.Hex("k", k[:]))
	err = contract.SetState(hash.BytesToHash256(k[:]), v[:])
	stateDB.assertError(err, "Failed to set state.", zap.Error(err), zap.String("address", evmAddr.Hex()))
}

// CommitContracts commits contract code to db and update pending contract account changes to trie
func (stateDB *StateDBAdapter) CommitContracts() error {
	contractAddrs := make([]common.Address, 0)
	for addr := range stateDB.cachedContract {
		contractAddrs = append(contractAddrs, addr)
	}
	sort.Slice(contractAddrs, func(i, j int) bool { return bytes.Compare(contractAddrs[i][:], contractAddrs[j][:]) < 0 })

	for _, addr := range contractAddrs {
		if _, ok := stateDB.selfDestructed[addr]; ok {
			// no need to update a SelfDestruct account/contract
			continue
		}
		contract := stateDB.cachedContract[addr]
		err := contract.Commit()
		if stateDB.assertError(err, "failed to commit contract", zap.Error(err), zap.String("address", addr.Hex())) {
			return errors.Wrap(err, "failed to commit contract")
		}
		// store the account (with new storage trie root) into account trie
		_, err = stateDB.sm.PutState(contract.SelfState(), protocol.KeyOption(addr[:]))
		if stateDB.assertError(err, "failed to store contract", zap.Error(err), zap.String("address", addr.Hex())) {
			return errors.Wrap(err, "failed to store contract")
		}
	}
	// delete suicided accounts/contract
	contractAddrs = contractAddrs[:0]
	for addr := range stateDB.selfDestructed {
		contractAddrs = append(contractAddrs, addr)
	}
	sort.Slice(contractAddrs, func(i, j int) bool { return bytes.Compare(contractAddrs[i][:], contractAddrs[j][:]) < 0 })

	for _, addr := range contractAddrs {
		_, err := stateDB.sm.DelState(protocol.KeyOption(addr[:]))
		if stateDB.assertError(err, "failed to delete SelfDestruct account/contract", zap.Error(err), zap.String("address", addr.Hex())) {
			return errors.Wrapf(err, "failed to delete SelfDestruct account/contract %x", addr[:])
		}
	}
	// write preimages to DB
	addrStrs := make([]string, 0)
	for addr := range stateDB.preimages {
		addrStrs = append(addrStrs, hex.EncodeToString(addr[:]))
	}
	sort.Strings(addrStrs)

	for _, addrStr := range addrStrs {
		var k common.Hash
		addrBytes, err := hex.DecodeString(addrStr)
		if stateDB.assertError(err, "failed to decode address hash", zap.Error(err), zap.String("address", addrStr)) {
			return errors.Wrap(err, "failed to decode address hash")
		}
		copy(k[:], addrBytes)
		_, err = stateDB.sm.PutState(stateDB.preimages[k], protocol.NamespaceOption(PreimageKVNameSpace), protocol.KeyOption(k[:]))
		if stateDB.assertError(err, "failed to update preimage to db", zap.Error(err), zap.String("address", addrStr)) {
			return errors.Wrap(err, "failed to update preimage to db")
		}
	}
	return nil
}

// getContract returns the contract of addr
func (stateDB *StateDBAdapter) getContract(addr common.Address) (Contract, error) {
	if contract, ok := stateDB.cachedContract[addr]; ok {
		return contract, nil
	}
	return stateDB.getNewContract(addr)
}

func (stateDB *StateDBAdapter) getNewContract(evmAddr common.Address) (Contract, error) {
	addr := hash.BytesToHash160(evmAddr.Bytes())
	account, err := accountutil.LoadAccountByHash160(stateDB.sm, addr, stateDB.accountCreationOpts()...)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load account state for address %x", addr)
	}
	contract, err := newContract(addr, account, stateDB.sm, stateDB.asyncContractTrie)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create storage trie for new contract %x", addr)
	}
	// add to contract cache
	stateDB.cachedContract[evmAddr] = contract
	return contract, nil
}

// clear clears local changes
func (stateDB *StateDBAdapter) clear() {
	stateDB.refundSnapshot = make(map[int]uint64)
	stateDB.cachedContract = make(contractMap)
	stateDB.contractSnapshot = make(map[int]contractMap)
	stateDB.selfDestructed = make(deleteAccount)
	stateDB.selfDestructedSnapshot = make(map[int]deleteAccount)
	stateDB.preimages = make(preimageMap)
	stateDB.preimageSnapshot = make(map[int]preimageMap)
	stateDB.accessList = newAccessList()
	stateDB.accessListSnapshot = make(map[int]*accessList)
	stateDB.transientStorage = newTransientStorage()
	stateDB.transientStorageSnapshot = make(map[int]transientStorage)
	stateDB.logsSnapshot = make(map[int]int)
	stateDB.txLogsSnapshot = make(map[int]int)
	stateDB.logs = []*action.Log{}
	stateDB.transactionLogs = []*action.TransactionLog{}
}
