// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package evm

import (
	"bytes"
	"context"
	"encoding/hex"
	stderrors "errors"
	"fmt"
	"math/big"
	"sort"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/db/trie"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/state"
)

type (
	// deleteAccount records the account/contract to be deleted
	deleteAccount map[common.Address]struct{}

	// createdAccount contains new accounts created in this tx
	createdAccount map[common.Address]struct{}

	// contractMap records the contracts being changed
	contractMap map[common.Address]Contract

	// preimageMap records the preimage of hash reported by VM
	preimageMap map[common.Hash]protocol.SerializableBytes

	// StateDBAdapter represents the state db adapter for evm to access iotx blockchain
	StateDBAdapter struct {
		ctx                        context.Context
		sm                         protocol.StateManager
		logs                       []*action.Log
		transactionLogs            []*action.TransactionLog
		err                        error
		blockHeight                uint64
		executionHash              hash.Hash256
		lastAddBalanceAddr         string
		lastAddBalanceAmount       *big.Int
		refund                     uint64
		refundSnapshot             map[int]uint64
		cachedContract             contractMap
		contractSnapshot           map[int]contractMap   // snapshots of contracts
		selfDestructed             deleteAccount         // account/contract calling SelfDestruct
		selfDestructedSnapshot     map[int]deleteAccount // snapshots of SelfDestruct accounts
		preimages                  preimageMap
		preimageSnapshot           map[int]preimageMap
		accessList                 *accessList // per-transaction access list
		accessListSnapshot         map[int]*accessList
		transientStorage           transientStorage // Transient storage
		transientStorageSnapshot   map[int]transientStorage
		createdAccount             createdAccount
		createdAccountSnapshot     map[int]createdAccount
		logsSnapshot               map[int]int // logs is an array, save len(logs) at time of snapshot suffices
		txLogsSnapshot             map[int]int
		newContract                func(addr hash.Hash160, account *state.Account) (Contract, error)
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
		enableCancun               bool
		fixRevertSnapshot          bool
		statelessWitnesses         map[common.Address]*ContractStorageWitness
		storageOps                 []StorageOp
		kvStoreWrapper             func(hash.Hash160) func(trie.KVStore) trie.KVStore // per-contract KVStore wrapper for proof recording
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

// EnableCancunEVMOption indicates that Cancun EVM is activated
func EnableCancunEVMOption() StateDBAdapterOption {
	return func(adapter *StateDBAdapter) error {
		adapter.enableCancun = true
		return nil
	}
}

// FixRevertSnapshotOption set fixRevertSnapshot as true
func FixRevertSnapshotOption() StateDBAdapterOption {
	return func(adapter *StateDBAdapter) error {
		adapter.fixRevertSnapshot = true
		return nil
	}
}

// KVStoreWrapperOption sets a per-contract KVStore wrapper function.
// The wrapper is applied to the contract's storage trie KVStore when a new
// contract is created, enabling external proof node recording.
func KVStoreWrapperOption(fn func(hash.Hash160) func(trie.KVStore) trie.KVStore) StateDBAdapterOption {
	return func(adapter *StateDBAdapter) error {
		adapter.kvStoreWrapper = fn
		return nil
	}
}

// StatelessValidationOption enables stateless validation mode.
// Contracts listed in witnesses will use stateless tries backed by witness
// entries instead of DB-backed MPT tries.
func StatelessValidationOption(witnesses map[common.Address]*ContractStorageWitness) StateDBAdapterOption {
	return func(adapter *StateDBAdapter) error {
		adapter.statelessWitnesses = witnesses
		return nil
	}
}

func WithContext(ctx context.Context) StateDBAdapterOption {
	return func(adapter *StateDBAdapter) error {
		adapter.ctx = ctx
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
		sm:                     sm,
		logs:                   []*action.Log{},
		err:                    nil,
		blockHeight:            blockHeight,
		executionHash:          executionHash,
		lastAddBalanceAmount:   new(big.Int),
		refundSnapshot:         make(map[int]uint64),
		cachedContract:         make(contractMap),
		contractSnapshot:       make(map[int]contractMap),
		selfDestructed:         make(deleteAccount),
		selfDestructedSnapshot: make(map[int]deleteAccount),
		preimages:              make(preimageMap),
		preimageSnapshot:       make(map[int]preimageMap),
		accessList:             newAccessList(),
		accessListSnapshot:     make(map[int]*accessList),
		logsSnapshot:           make(map[int]int),
		txLogsSnapshot:         make(map[int]int),
		ctx:                    context.Background(),
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
	if s.enableCancun {
		s.transientStorage = newTransientStorage()
		s.transientStorageSnapshot = make(map[int]transientStorage)
		s.createdAccount = make(createdAccount)
		s.createdAccountSnapshot = make(map[int]createdAccount)
	}
	s.newContract = func(addr hash.Hash160, account *state.Account) (Contract, error) {
		if s.statelessWitnesses != nil {
			return newStatelessContract(addr, account, s.sm, s.statelessWitnesses, s.asyncContractTrie)
		}
		c, err := newContract(addr, account, s.sm, s.asyncContractTrie)
		if err != nil {
			return nil, err
		}
		if s.kvStoreWrapper != nil {
			if err := applyKVStoreWrapper(c, addr, s.kvStoreWrapper(addr)); err != nil {
				return nil, err
			}
		}
		return c, nil
	}
	return s, nil
}

// applyKVStoreWrapper clones the contract's storage trie with a wrapped KVStore.
// The wrapper is provided by the sharedRecordingKVStore and records all trie node
// reads for proof node extraction.
func applyKVStoreWrapper(c Contract, addr hash.Hash160, wrapper func(trie.KVStore) trie.KVStore) error {
	cc, ok := c.(*contract)
	if !ok {
		return nil
	}
	smKV := protocol.NewKVStoreForTrieWithStateManager(ContractKVNameSpace, cc.sm)
	wrappedKV := wrapper(smKV)
	clonedTrie, err := cc.trie.Clone(wrappedKV)
	if err != nil {
		return errors.Wrap(err, "failed to clone trie with kvstore wrapper")
	}
	// Prime the root node so it's recorded in the wrapper
	rootHash, err := clonedTrie.RootHash()
	if err != nil {
		return errors.Wrap(err, "failed to get root hash for kvstore wrapper priming")
	}
	if rkv, ok := wrappedKV.(*contractRecordingKVStore); ok && len(rootHash) > 0 {
		rkv.prime(rootHash)
	}
	cc.trie = clonedTrie
	return nil
}

// StorageRootOf returns the current storage root hash for the given contract address.
// If the contract is not yet in cache, it loads it. Returns ZeroHash256 if the
// account doesn't exist or has no storage root.
func (stateDB *StateDBAdapter) StorageRootOf(addr common.Address) hash.Hash256 {
	c, err := stateDB.getContract(addr)
	if err != nil {
		return hash.ZeroHash256
	}
	return c.SelfState().Root
}

// StorageOps returns the recorded storage operations trace.
func (stateDB *StateDBAdapter) StorageOps() []StorageOp {
	return stateDB.storageOps
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
		log.T(stateDB.ctx).Panic(msg, fields...)
	}
	log.T(stateDB.ctx).Error(msg, fields...)
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
	// TODO: possibly remove the enableCancun flag after hard-fork
	if _, ok := stateDB.cachedContract[evmAddr]; !ok || !stateDB.enableCancun {
		_, err = accountutil.LoadOrCreateAccount(stateDB.sm, addr, stateDB.accountCreationOpts()...)
		if stateDB.assertError(err, "Failed to create account.", zap.Error(err), zap.String("address", evmAddr.Hex())) {
			return
		}
	}
	if stateDB.enableCancun {
		stateDB.createdAccount[evmAddr] = struct{}{}
	}
	log.T(stateDB.ctx).Debug("Called CreateAccount.", log.Hex("addrHash", evmAddr[:]))
}

// SubBalance subtracts balance from account
func (stateDB *StateDBAdapter) SubBalance(evmAddr common.Address, a256 *uint256.Int) {
	amount := a256.ToBig()
	if amount.Cmp(big.NewInt(int64(0))) == 0 {
		return
	}
	// stateDB.GetBalance(evmAddr)
	log.T(stateDB.ctx).Debug(fmt.Sprintf("SubBalance %v from %s", amount, evmAddr.Hex()))
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
	log.T(stateDB.ctx).Debug(fmt.Sprintf("AddBalance %v to %s", amount, evmAddr.Hex()))

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
	log.T(stateDB.ctx).Debug(fmt.Sprintf("Balance of %s is %v", evmAddr.Hex(), state.Balance))

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
			log.T(stateDB.ctx).Panic("Failed to get nonce.", zap.Error(err))
		} else {
			log.T(stateDB.ctx).Error("Failed to get nonce.", zap.Error(err))
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
			log.T(stateDB.ctx).Panic("invalid pending nonce")
		}
		pendingNonce--
	}
	log.T(stateDB.ctx).Debug("Called GetNonce.",
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
			log.T(stateDB.ctx).Panic("invalid nonce zero")
		}
		nonce--
	}
	log.T(stateDB.ctx).Debug("Called SetNonce.",
		zap.String("address", addr.String()),
		zap.Uint64("nonce", nonce))
	if !s.IsNewbieAccount() || s.AccountType() != 0 || nonce != 0 || stateDB.zeroNonceForFreshAccount {
		if err := s.SetPendingNonce(nonce + 1); err != nil {
			log.T(stateDB.ctx).Panic("Failed to set nonce.", zap.Error(err), zap.String("addr", addr.Hex()), zap.Uint64("pendingNonce", s.PendingNonce()), zap.Uint64("nonce", nonce), zap.String("execution", hex.EncodeToString(stateDB.executionHash[:])))
			stateDB.logError(err)
		}
	}
	err = accountutil.StoreAccount(stateDB.sm, addr, s)
	stateDB.assertError(err, "Failed to store account.", zap.Error(err), zap.String("address", evmAddr.Hex()))
}

// SubRefund subtracts refund
func (stateDB *StateDBAdapter) SubRefund(gas uint64) {
	log.T(stateDB.ctx).Debug("Called SubRefund.", zap.Uint64("gas", gas))
	// stateDB.journal.append(refundChange{prev: self.refund})
	if gas > stateDB.refund {
		log.T(stateDB.ctx).Panic("Refund counter not enough!")
	}
	stateDB.refund -= gas
}

// AddRefund adds refund
func (stateDB *StateDBAdapter) AddRefund(gas uint64) {
	log.T(stateDB.ctx).Debug("Called AddRefund.", zap.Uint64("gas", gas))
	// stateDB.journal.append(refundChange{prev: self.refund})
	stateDB.refund += gas
}

// GetRefund gets refund
func (stateDB *StateDBAdapter) GetRefund() uint64 {
	log.T(stateDB.ctx).Debug("Called GetRefund.")
	return stateDB.refund
}

// SelfDestruct kills the contract
func (stateDB *StateDBAdapter) SelfDestruct(evmAddr common.Address) {
	if !stateDB.Exist(evmAddr) {
		log.T(stateDB.ctx).Debug("Account does not exist.", zap.String("address", evmAddr.Hex()))
		return
	}
	s, err := stateDB.accountState(evmAddr)
	if stateDB.assertError(err, "Failed to get account.", zap.Error(err), zap.String("address", evmAddr.Hex())) {
		return
	}
	// clears the account balance
	actBalance := new(big.Int).Set(s.Balance)
	log.T(stateDB.ctx).Info("SelfDestruct contract", zap.String("Balance", actBalance.String()))
	err = s.SubBalance(s.Balance)
	if stateDB.assertError(err, "Failed to clear balance.", zap.Error(err), zap.String("address", evmAddr.Hex())) {
		return
	}
	_, err = stateDB.sm.PutState(s, protocol.KeyOption(evmAddr[:]))
	if stateDB.assertError(err, "Failed to kill contract.", zap.Error(err), zap.String("address", evmAddr.Hex())) {
		return
	}
	// before calling SelfDestruct, EVM will transfer the contract's balance to beneficiary
	// need to create a transaction log on successful SelfDestruct
	from, _ := address.FromBytes(evmAddr[:])
	stateDB.generateSelfDestructTransferLog(from.String(), stateDB.lastAddBalanceAmount.Cmp(actBalance) == 0)
	// mark it as deleted
	stateDB.selfDestructed[evmAddr] = struct{}{}
}

// HasSelfDestructed returns whether the contract has been killed
func (stateDB *StateDBAdapter) HasSelfDestructed(evmAddr common.Address) bool {
	_, ok := stateDB.selfDestructed[evmAddr]
	return ok
}

// Selfdestruct6780 implements EIP-6780
func (stateDB *StateDBAdapter) Selfdestruct6780(evmAddr common.Address) {
	if !stateDB.Exist(evmAddr) {
		log.T(stateDB.ctx).Debug("Account does not exist.", zap.String("address", evmAddr.Hex()))
		return
	}
	// opSelfdestruct6780 has already subtracted the contract's balance
	// so create a transaction log
	from, _ := address.FromBytes(evmAddr[:])
	stateDB.generateSelfDestructTransferLog(from.String(), true)
	// per EIP-6780, delete the account only if it is created in the same transaction
	if _, ok := stateDB.createdAccount[evmAddr]; ok {
		stateDB.selfDestructed[evmAddr] = struct{}{}
	}
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

// Exist checks the existence of an address
func (stateDB *StateDBAdapter) Exist(evmAddr common.Address) bool {
	addr, err := address.FromBytes(evmAddr.Bytes())
	if stateDB.assertError(err, "Failed to convert evm address.", zap.Error(err)) {
		return false
	}
	log.T(stateDB.ctx).Debug("Check existence.", zap.String("address", addr.String()), log.Hex("addrHash", evmAddr[:]))
	if _, ok := stateDB.cachedContract[evmAddr]; ok {
		return true
	}
	recorded, err := accountutil.Recorded(stateDB.sm, addr)
	if stateDB.assertError(err, "Account does not exist.", zap.Error(err), zap.String("address", evmAddr.Hex())) {
		return false
	}
	if !recorded {
		log.T(stateDB.ctx).Debug("Account does not exist.", zap.String("address", addr.String()))
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

// Empty returns true if the contract is empty
func (stateDB *StateDBAdapter) Empty(evmAddr common.Address) bool {
	log.T(stateDB.ctx).Debug("Check whether the contract is empty.")
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
	stateDB.storageOps = append(stateDB.storageOps, StorageOp{
		OpType:     StorageOpRevertToSnapshot,
		SnapshotID: snapshot,
	})
	ds, ok := stateDB.selfDestructedSnapshot[snapshot]
	if !ok && stateDB.panicUnrecoverableError {
		log.T(stateDB.ctx).Panic("Failed to revert to snapshot.", zap.Int("snapshot", snapshot))
	}
	err := stateDB.sm.Revert(snapshot)
	if stateDB.assertError(err, "state manager's Revert() failed.", zap.Error(err), zap.Int("snapshot", snapshot)) {
		return
	}
	if !ok {
		// this should not happen, b/c we save the SelfDestruct accounts on a successful return of Snapshot(), but check anyway
		log.T(stateDB.ctx).Error("Failed to revert to snapshot.", zap.Int("snapshot", snapshot))
		return
	}
	deleteSnapshot := snapshot
	if stateDB.fixRevertSnapshot {
		deleteSnapshot++
	}
	// restore gas refund
	if !stateDB.manualCorrectGasRefund {
		stateDB.refund = stateDB.refundSnapshot[snapshot]
		for i := deleteSnapshot; ; i++ {
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
		for i := deleteSnapshot; ; i++ {
			if _, ok := stateDB.accessListSnapshot[i]; ok {
				delete(stateDB.accessListSnapshot, i)
			} else {
				break
			}
		}
	}
	if stateDB.enableCancun {
		//restore transientStorage
		stateDB.transientStorage = stateDB.transientStorageSnapshot[snapshot]
		{
			for i := deleteSnapshot; ; i++ {
				if _, ok := stateDB.transientStorageSnapshot[i]; ok {
					delete(stateDB.transientStorageSnapshot, i)
				} else {
					break
				}
			}
		}
		// restore created accounts
		stateDB.createdAccount = stateDB.createdAccountSnapshot[snapshot]
		{
			for i := deleteSnapshot; ; i++ {
				if _, ok := stateDB.createdAccountSnapshot[i]; ok {
					delete(stateDB.createdAccountSnapshot, i)
				} else {
					break
				}
			}
		}
	}
	// restore logs and txLogs
	if stateDB.revertLog {
		stateDB.logs = stateDB.logs[:stateDB.logsSnapshot[snapshot]]
		for i := deleteSnapshot; ; i++ {
			if _, ok := stateDB.logsSnapshot[i]; ok {
				delete(stateDB.logsSnapshot, i)
			} else {
				break
			}
		}
		stateDB.transactionLogs = stateDB.transactionLogs[:stateDB.txLogsSnapshot[snapshot]]
		for i := deleteSnapshot; ; i++ {
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
		for i := deleteSnapshot; ; i++ {
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
			log.T(stateDB.ctx).Error("Failed to load root for contract.", zap.Error(err), log.Hex("addrHash", addr[:]))
			return
		}
	}
	if stateDB.fixSnapshotOrder {
		for i := deleteSnapshot; ; i++ {
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
		for i := deleteSnapshot; ; i++ {
			if _, ok := stateDB.preimageSnapshot[i]; ok {
				delete(stateDB.preimageSnapshot, i)
			} else {
				break
			}
		}
	}
}

// panicWithMissingProofDiagnostics logs detailed information about a missing
// proof node before panicking. This helps diagnose whether the stateless node's
// EVM execution diverged from the producer (accessing a key not in the witness
// entries) or the proof generation was buggy (key is in entries but proof
// nodes are insufficient).
func (stateDB *StateDBAdapter) panicWithMissingProofDiagnostics(
	evmAddr common.Address,
	storageKey common.Hash,
	mpErr *ErrMissingProofNode,
) {
	var diag strings.Builder
	fmt.Fprintf(&diag, "STATELESS VALIDATION FAILURE: missing proof node\n")
	fmt.Fprintf(&diag, "  contract:        %s\n", evmAddr.Hex())
	fmt.Fprintf(&diag, "  storage key:     %s\n", storageKey.Hex())
	fmt.Fprintf(&diag, "  missing hash:    %x\n", mpErr.MissingHash)
	fmt.Fprintf(&diag, "  proof nodes:     %d\n", mpErr.ProofNodeCount)
	fmt.Fprintf(&diag, "  puts:            %d\n", mpErr.PutCount)

	// Check if the storage key is among the witness entries for this contract.
	keyInEntries := false
	if stateDB.statelessWitnesses != nil {
		if w, ok := stateDB.statelessWitnesses[evmAddr]; ok {
			fmt.Fprintf(&diag, "  witness root:    %x\n", w.StorageRoot[:])
			fmt.Fprintf(&diag, "  witness entries: %d\n", len(w.Entries))
			for i, entry := range w.Entries {
				marker := "  "
				if entry.Key == hash.BytesToHash256(storageKey[:]) {
					marker = ">>"
					keyInEntries = true
				}
				fmt.Fprintf(&diag, "    %s entry[%d]: key=%x value=%x\n", marker, i, entry.Key[:], entry.Value)
			}
		} else {
			fmt.Fprintf(&diag, "  WARNING: no witness found for this contract in action witnesses!\n")
		}
	} else {
		fmt.Fprintf(&diag, "  NOTE: statelessWitnesses is nil (not a stateless validation)\n")
	}

	if keyInEntries {
		fmt.Fprintf(&diag, "  DIAGNOSIS: key IS in witness entries → proof generation bug (insufficient proof nodes)\n")
	} else {
		fmt.Fprintf(&diag, "  DIAGNOSIS: key NOT in witness entries → execution diverged from producer\n")
	}

	// Dump all storage operations that happened before this failure.
	// This reveals the full execution trace, helping pinpoint where
	// the stateless node's execution diverged from the producer.
	if len(stateDB.storageOps) > 0 {
		fmt.Fprintf(&diag, "\n  Stateless storage ops trace (%d ops):\n", len(stateDB.storageOps))
		for i, op := range stateDB.storageOps {
			fmt.Fprintf(&diag, "    [%d] %s\n", i, op.String())
		}
	}

	// Dump the producer's (full node) storage ops from the witness data for
	// side-by-side comparison. These ops were captured when the producer
	// executed this action and are shipped inside the witness JSON.
	var producerOps []StorageOpTraceJSON
	if svCtx, ok := GetStatelessValidationCtx(stateDB.ctx); ok && svCtx.DebugStorageOps != nil {
		producerOps = svCtx.DebugStorageOps[stateDB.executionHash]
	}
	if len(producerOps) > 0 {
		fmt.Fprintf(&diag, "\n  Producer storage ops trace (%d ops):\n", len(producerOps))
		for i, op := range producerOps {
			if op.Op == "Snapshot" || op.Op == "RevertToSnapshot" {
				fmt.Fprintf(&diag, "    [%d] %s snapshotID=%d\n", i, op.Op, op.SnapshotID)
			} else if op.ErrMsg != "" {
				fmt.Fprintf(&diag, "    [%d] %s addr=%s key=%s err=%s\n", i, op.Op, op.Addr, op.Key, op.ErrMsg)
			} else {
				fmt.Fprintf(&diag, "    [%d] %s addr=%s key=%s val=%s\n", i, op.Op, op.Addr, op.Key, op.Value)
			}
		}
	} else {
		fmt.Fprintf(&diag, "\n  Producer storage ops trace: NOT AVAILABLE (no DebugStorageOps in witness)\n")
	}

	// Compare producer vs stateless ops (filtering Snapshot/RevertToSnapshot)
	// to find the first divergence point for quick identification.
	if len(producerOps) > 0 && len(stateDB.storageOps) > 0 {
		filterJSON := func(ops []StorageOpTraceJSON) []StorageOpTraceJSON {
			var out []StorageOpTraceJSON
			for _, op := range ops {
				if op.Op != "Snapshot" && op.Op != "RevertToSnapshot" {
					out = append(out, op)
				}
			}
			return out
		}
		pFiltered := filterJSON(producerOps)
		sFiltered := filterJSON(StorageOpsToJSON(stateDB.storageOps))
		minLen := len(pFiltered)
		if len(sFiltered) < minLen {
			minLen = len(sFiltered)
		}
		divergeIdx := -1
		for i := 0; i < minLen; i++ {
			if pFiltered[i].Op != sFiltered[i].Op ||
				pFiltered[i].Addr != sFiltered[i].Addr ||
				pFiltered[i].Key != sFiltered[i].Key ||
				pFiltered[i].Value != sFiltered[i].Value {
				divergeIdx = i
				break
			}
		}
		if divergeIdx == -1 && len(pFiltered) != len(sFiltered) {
			divergeIdx = minLen
		}
		if divergeIdx >= 0 {
			fmt.Fprintf(&diag, "\n  === DIVERGENCE DETECTED at filtered op index %d (excluding Snapshot/Revert) ===\n", divergeIdx)
			// Show context: 3 ops before and 3 after the divergence
			start := divergeIdx - 3
			if start < 0 {
				start = 0
			}
			end := divergeIdx + 4
			fmt.Fprintf(&diag, "  Producer ops around divergence:\n")
			for i := start; i < end && i < len(pFiltered); i++ {
				marker := "  "
				if i == divergeIdx {
					marker = ">>"
				}
				op := pFiltered[i]
				fmt.Fprintf(&diag, "    %s [%d] %s addr=%s key=%s val=%s\n", marker, i, op.Op, op.Addr, op.Key, op.Value)
			}
			fmt.Fprintf(&diag, "  Stateless ops around divergence:\n")
			for i := start; i < end && i < len(sFiltered); i++ {
				marker := "  "
				if i == divergeIdx {
					marker = ">>"
				}
				op := sFiltered[i]
				fmt.Fprintf(&diag, "    %s [%d] %s addr=%s key=%s val=%s\n", marker, i, op.Op, op.Addr, op.Key, op.Value)
			}
		} else {
			fmt.Fprintf(&diag, "\n  All filtered ops MATCH between producer and stateless (count: producer=%d, stateless=%d)\n", len(pFiltered), len(sFiltered))
		}
	}

	panic(diag.String())
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
			log.T(stateDB.ctx).Panic("Failed to snapshot.", zap.Error(err))
		} else {
			log.T(stateDB.ctx).Error("Failed to snapshot.", zap.Error(err))
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
	if stateDB.enableCancun {
		// save a copy of transient storage
		stateDB.transientStorageSnapshot[sn] = stateDB.transientStorage.Copy()
		// save a copy of created account map
		ca := make(createdAccount)
		for k, v := range stateDB.createdAccount {
			ca[k] = v
		}
		stateDB.createdAccountSnapshot[sn] = ca
	}
	stateDB.storageOps = append(stateDB.storageOps, StorageOp{
		OpType:     StorageOpSnapshot,
		SnapshotID: sn,
	})
	return sn
}

// AddLog adds log whose transaction amount is larger than 0
func (stateDB *StateDBAdapter) AddLog(evmLog *types.Log) {
	log.T(stateDB.ctx).Debug("Called AddLog.", zap.Any("log", evmLog))
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
			log.T(stateDB.ctx).Panic("Invalid in contract transfer topics")
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

func (stateDB *StateDBAdapter) generateSelfDestructTransferLog(sender string, amountMatch bool) {
	// To ensure data consistency, generate this log after the hard-fork
	// a separate patch file will be created later to provide missing logs before the hard-fork
	// TODO: remove this gating once the hard-fork has passed
	if stateDB.suicideTxLogMismatchPanic {
		if amountMatch {
			if stateDB.lastAddBalanceAmount.Cmp(big.NewInt(0)) > 0 {
				stateDB.addTransactionLogs(&action.TransactionLog{
					Type:      iotextypes.TransactionLogType_IN_CONTRACT_TRANSFER,
					Sender:    sender,
					Recipient: stateDB.lastAddBalanceAddr,
					Amount:    stateDB.lastAddBalanceAmount,
				})
			}
		} else {
			log.T(stateDB.ctx).Panic("SelfDestruct contract's balance does not match",
				zap.String("beneficiary", stateDB.lastAddBalanceAmount.String()))
		}
	}
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
			log.T(stateDB.ctx).Error("Failed to get code hash.", zap.Error(err))
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
	log.T(stateDB.ctx).Debug("Called GetCodeSize.", log.Hex("addrHash", evmAddr[:]))
	return len(code)
}

// SetCode sets contract's code
func (stateDB *StateDBAdapter) SetCode(evmAddr common.Address, code []byte) {
	contract, err := stateDB.getContract(evmAddr)
	if err != nil {
		log.T(stateDB.ctx).Error("Failed to get contract.", zap.Error(err), zap.String("address", evmAddr.Hex()))
		stateDB.logError(err)
		return
	}
	contract.SetCode(hash.Hash256b(code), code)
}

// GetCommittedState gets committed state
func (stateDB *StateDBAdapter) GetCommittedState(evmAddr common.Address, k common.Hash) common.Hash {
	contract, err := stateDB.getContract(evmAddr)
	if err != nil {
		log.T(stateDB.ctx).Error("Failed to get contract.", zap.Error(err), zap.String("address", evmAddr.Hex()))
		stateDB.logError(err)
		stateDB.storageOps = append(stateDB.storageOps, StorageOp{
			OpType: StorageOpGetCommittedStateFailed,
			Addr:   evmAddr,
			Key:    k,
			ErrMsg: "getContract: " + err.Error(),
		})
		return common.Hash{}
	}
	v, err := contract.GetCommittedState(hash.BytesToHash256(k[:]))
	if err != nil {
		var mpErr *ErrMissingProofNode
		if stderrors.As(err, &mpErr) {
			stateDB.panicWithMissingProofDiagnostics(evmAddr, k, mpErr)
		}
		log.T(stateDB.ctx).Error("Failed to get committed state.", zap.Error(err), zap.String("address", evmAddr.Hex()), zap.String("key", k.Hex()))
		stateDB.logError(err)
		stateDB.storageOps = append(stateDB.storageOps, StorageOp{
			OpType: StorageOpGetCommittedStateFailed,
			Addr:   evmAddr,
			Key:    k,
			ErrMsg: err.Error(),
		})
		return common.Hash{}
	}
	result := common.BytesToHash(v)
	stateDB.storageOps = append(stateDB.storageOps, StorageOp{
		OpType: StorageOpGetCommittedState,
		Addr:   evmAddr,
		Key:    k,
		Value:  result,
	})
	return result
}

// GetState gets state
func (stateDB *StateDBAdapter) GetState(evmAddr common.Address, k common.Hash) common.Hash {
	contract, err := stateDB.getContract(evmAddr)
	if err != nil {
		log.T(stateDB.ctx).Error("Failed to get contract.", zap.Error(err), zap.String("address", evmAddr.Hex()))
		stateDB.logError(err)
		stateDB.storageOps = append(stateDB.storageOps, StorageOp{
			OpType: StorageOpGetStateFailed,
			Addr:   evmAddr,
			Key:    k,
			ErrMsg: "getContract: " + err.Error(),
		})
		return common.Hash{}
	}
	v, err := contract.GetState(hash.BytesToHash256(k[:]))
	if err != nil {
		// Check if this is a missing proof node error from the stateless trie.
		// If so, log enhanced diagnostics and re-panic so the block validation
		// fails with actionable debug information.
		var mpErr *ErrMissingProofNode
		if stderrors.As(err, &mpErr) {
			stateDB.panicWithMissingProofDiagnostics(evmAddr, k, mpErr)
		}
		log.T(stateDB.ctx).Error("Failed to get state.", zap.Error(err), zap.String("address", evmAddr.Hex()), zap.String("key", k.Hex()))
		stateDB.logError(err)
		stateDB.storageOps = append(stateDB.storageOps, StorageOp{
			OpType: StorageOpGetStateFailed,
			Addr:   evmAddr,
			Key:    k,
			ErrMsg: err.Error(),
		})
		return common.Hash{}
	}
	result := common.BytesToHash(v)
	stateDB.storageOps = append(stateDB.storageOps, StorageOp{
		OpType: StorageOpGetState,
		Addr:   evmAddr,
		Key:    k,
		Value:  result,
	})
	return result
}

// SetState sets state
func (stateDB *StateDBAdapter) SetState(evmAddr common.Address, k, v common.Hash) {
	contract, err := stateDB.getContract(evmAddr)
	if err != nil {
		log.T(stateDB.ctx).Error("Failed to get contract.", zap.Error(err), zap.String("address", evmAddr.Hex()))
		stateDB.logError(err)
		return
	}
	log.T(stateDB.ctx).Debug("Called SetState", log.Hex("addrHash", evmAddr[:]), log.Hex("k", k[:]))
	err = contract.SetState(hash.BytesToHash256(k[:]), v[:])
	stateDB.assertError(err, "Failed to set state.", zap.Error(err), zap.String("address", evmAddr.Hex()))
	stateDB.storageOps = append(stateDB.storageOps, StorageOp{
		OpType: StorageOpSetState,
		Addr:   evmAddr,
		Key:    k,
		Value:  v,
	})
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
		_, err := stateDB.sm.DelState(protocol.KeyOption(addr[:]), protocol.ObjectOption(&state.Account{}))
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
	contract, err := stateDB.newContract(addr, account)
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
	stateDB.logsSnapshot = make(map[int]int)
	stateDB.txLogsSnapshot = make(map[int]int)
	stateDB.logs = []*action.Log{}
	stateDB.transactionLogs = []*action.TransactionLog{}
	stateDB.storageOps = nil
	if stateDB.enableCancun {
		stateDB.transientStorage = newTransientStorage()
		stateDB.transientStorageSnapshot = make(map[int]transientStorage)
		stateDB.createdAccount = make(createdAccount)
		stateDB.createdAccountSnapshot = make(map[int]createdAccount)
	}
}
