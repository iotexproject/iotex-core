// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package state

import (
	"context"
	"math/big"
	"sort"

	"github.com/boltdb/bolt"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/blockchain/action"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/trie"
)

var (
	// ErrNotEnoughBalance is the error that the balance is not enough
	ErrNotEnoughBalance = errors.New("not enough balance")

	// ErrAccountNotExist is the error that the account does not exist
	ErrAccountNotExist = errors.New("account does not exist")

	// ErrAccountCollision is the error that the account already exists
	ErrAccountCollision = errors.New("account already exists")

	// ErrFailedToMarshalState is the error that the state marshaling is failed
	ErrFailedToMarshalState = errors.New("failed to marshal state")

	// ErrFailedToUnmarshalState is the error that the state un-marshaling is failed
	ErrFailedToUnmarshalState = errors.New("failed to unmarshal state")
)

const (
	// CurrentHeightKey indicates the key of current factory height in underlying DB
	CurrentHeightKey = "currentHeight"
	// AccountTrieRootKey indicates the key of accountTrie root hash in underlying DB
	AccountTrieRootKey = "accountTrieRoot"
)

type (
	// Factory defines an interface for managing states
	Factory interface {
		lifecycle.StartStopper
		// Accounts
		LoadOrCreateState(string, uint64) (*State, error)
		Balance(string) (*big.Int, error)
		Nonce(string) (uint64, error) // Note that nonce starts with 1.
		State(string) (*State, error)
		CachedState(string) (*State, error)
		RootHash() hash.Hash32B
		Height() (uint64, error)
		RunActions(uint64, []*action.Transfer, []*action.Vote, []*action.Execution) (hash.Hash32B, error)
		HasRun() bool
		Commit() error
		// Contracts
		GetCodeHash(hash.AddrHash) (hash.Hash32B, error)
		GetCode(hash.AddrHash) ([]byte, error)
		SetCode(hash.AddrHash, []byte) error
		GetContractState(hash.AddrHash, hash.Hash32B) (hash.Hash32B, error)
		SetContractState(hash.AddrHash, hash.Hash32B, hash.Hash32B) error
		// Candidate pool
		Candidates() (uint64, []*Candidate)
		CandidatesByHeight(uint64) ([]*Candidate, error)
	}

	// factory implements StateFactory interface, tracks changes to account/contract and batch-commits to DB
	factory struct {
		lifecycle lifecycle.Lifecycle
		// candidate pool
		currentChainHeight uint64
		numCandidates      uint
		cachedCandidates   map[hash.AddrHash]*Candidate
		// accounts
		savedAccount   map[string]*State          // save account state before being modified in this block
		cachedAccount  map[hash.AddrHash]*State   // accounts being modified in this block
		cachedContract map[hash.AddrHash]Contract // contracts being modified in this block
		run            bool                       // indicates that RunActions() has been called
		rootHash       hash.Hash32B               // new root hash after running executions in this block
		accountTrie    trie.Trie                  // global state trie
		dao            db.CachedKVStore           // the underlying DB for account/contract storage
	}
)

// FactoryOption sets Factory construction parameter
type FactoryOption func(*factory, *config.Config) error

// PrecreatedTrieOption uses pre-created tries for state factory
func PrecreatedTrieOption(accountTrie trie.Trie) FactoryOption {
	return func(sf *factory, cfg *config.Config) error {
		sf.accountTrie = accountTrie
		sf.dao = db.NewCachedKVStore(sf.accountTrie.TrieDB())
		return nil
	}
}

// DefaultTrieOption creates trie from config for state factory
func DefaultTrieOption() FactoryOption {
	return func(sf *factory, cfg *config.Config) error {
		dbPath := cfg.Chain.TrieDBPath
		if len(dbPath) == 0 {
			return errors.New("Invalid empty trie db path")
		}
		trieDB := db.NewBoltDB(dbPath, &cfg.DB)
		if err := trieDB.Start(context.Background()); err != nil {
			return errors.Wrap(err, "failed to start trie db")
		}
		// create a common cached KV to be shared by account trie and all contract trie
		sf.dao = db.NewCachedKVStore(trieDB)
		// create account trie
		accountTrieRoot, err := sf.getRoot(trie.AccountKVNameSpace, AccountTrieRootKey)
		if err != nil {
			return errors.Wrap(err, "failed to get accountTrie's root hash from underlying DB")
		}
		tr, err := trie.NewTrieSharedDB(sf.dao, trie.AccountKVNameSpace, accountTrieRoot)
		if err != nil {
			return errors.Wrap(err, "failed to generate accountTrie from config")
		}
		sf.accountTrie = tr
		return nil
	}
}

// InMemTrieOption creates in memory trie for state factory
func InMemTrieOption() FactoryOption {
	return func(sf *factory, cfg *config.Config) error {
		trieDB := db.NewMemKVStore()
		if err := trieDB.Start(context.Background()); err != nil {
			return errors.Wrap(err, "failed to start trie db")
		}
		// create a common cached KV to be shared by account trie and all contract trie
		sf.dao = db.NewCachedKVStore(trieDB)
		// create account trie
		accountTrieRoot, err := sf.getRoot(trie.AccountKVNameSpace, AccountTrieRootKey)
		if err != nil {
			return errors.Wrap(err, "failed to get accountTrie's root hash from underlying DB")
		}
		tr, err := trie.NewTrieSharedDB(sf.dao, trie.AccountKVNameSpace, accountTrieRoot)
		if err != nil {
			return errors.Wrap(err, "failed to generate accountTrie from config")
		}
		sf.accountTrie = tr
		return nil
	}
}

// NewFactory creates a new state factory
func NewFactory(cfg *config.Config, opts ...FactoryOption) (Factory, error) {
	sf := &factory{
		currentChainHeight: 0,
		numCandidates:      cfg.Chain.NumCandidates,
		cachedCandidates:   make(map[hash.AddrHash]*Candidate),
		savedAccount:       make(map[string]*State),
		cachedAccount:      make(map[hash.AddrHash]*State),
		cachedContract:     make(map[hash.AddrHash]Contract),
	}

	for _, opt := range opts {
		if err := opt(sf, cfg); err != nil {
			logger.Error().Err(err).Msgf("Failed to create state factory option %s", opt)
			return nil, err
		}
	}
	if sf.accountTrie != nil {
		sf.lifecycle.Add(sf.accountTrie)
	}
	return sf, nil
}

func (sf *factory) Start(ctx context.Context) error { return sf.lifecycle.OnStart(ctx) }

func (sf *factory) Stop(ctx context.Context) error { return sf.lifecycle.OnStop(ctx) }

//======================================
// State/Account functions
//======================================
// LoadOrCreateState loads existing or adds a new State with initial balance to the factory
// addr should be a bech32 properly-encoded string
func (sf *factory) LoadOrCreateState(addr string, init uint64) (*State, error) {
	h, err := iotxaddress.GetPubkeyHash(addr)
	if err != nil {
		return nil, errors.Wrap(err, "error when getting the pubkey hash")
	}
	addrHash := byteutil.BytesTo20B(h)
	state, err := sf.cachedState(addrHash)
	switch {
	case errors.Cause(err) == ErrAccountNotExist:
		balance := big.NewInt(0)
		balance.SetUint64(init)
		state = &State{
			Balance:      balance,
			VotingWeight: big.NewInt(0),
		}
		sf.cachedAccount[addrHash] = state
	case err != nil:
		return nil, errors.Wrapf(err, "failed to get state of %x from cached state", addrHash)
	}
	return state, nil
}

// Balance returns balance
func (sf *factory) Balance(addr string) (*big.Int, error) {
	if saved, ok := sf.savedAccount[addr]; ok {
		return saved.Balance, nil
	}
	pkHash, err := iotxaddress.GetPubkeyHash(addr)
	if err != nil {
		return nil, errors.Wrap(err, "error when getting the pubkey hash")
	}
	state, err := sf.getState(byteutil.BytesTo20B(pkHash))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get state of %x", pkHash)
	}
	return state.Balance, nil
}

// Nonce returns the nonce if the account exists
func (sf *factory) Nonce(addr string) (uint64, error) {
	if saved, ok := sf.savedAccount[addr]; ok {
		return saved.Nonce, nil
	}
	pkHash, err := iotxaddress.GetPubkeyHash(addr)
	if err != nil {
		return 0, errors.Wrap(err, "error when getting the pubkey hash")
	}
	state, err := sf.getState(byteutil.BytesTo20B(pkHash))
	if err != nil {
		return 0, errors.Wrapf(err, "failed to get state of %x", pkHash)
	}
	return state.Nonce, nil
}

// State returns the confirmed state on the chain
func (sf *factory) State(addr string) (*State, error) {
	if saved, ok := sf.savedAccount[addr]; ok {
		return saved, nil
	}
	pkHash, err := iotxaddress.GetPubkeyHash(addr)
	if err != nil {
		return nil, errors.Wrap(err, "error when getting the pubkey hash")
	}
	return sf.getState(byteutil.BytesTo20B(pkHash))
}

// CachedState returns the cached state if the address exists in local cache
func (sf *factory) CachedState(addr string) (*State, error) {
	h, err := iotxaddress.GetPubkeyHash(addr)
	if err != nil {
		return nil, errors.Wrap(err, "error when getting the pubkey hash")
	}
	addrHash := byteutil.BytesTo20B(h)
	if contract, ok := sf.cachedContract[addrHash]; ok {
		return contract.SelfState(), nil
	}
	return sf.cachedState(addrHash)
}

// RootHash returns the hash of the root node of the accountTrie
func (sf *factory) RootHash() hash.Hash32B {
	return sf.accountTrie.RootHash()
}

// Height returns factory's height
func (sf *factory) Height() (uint64, error) {
	height, err := sf.dao.Get(trie.AccountKVNameSpace, []byte(CurrentHeightKey))
	if err != nil {
		return 0, errors.Wrap(err, "failed to get factory's height from underlying DB")
	}
	return byteutil.BytesToUint64(height), nil
}

// RunActions will be called 2 times in
// 1. In MintNewBlock(), the block producer runs all executions in new block and get the new trie root hash (which
// is written in block header), but all changes are not committed to blockchain yet
// 2. In CommitBlock(), all nodes except block producer will run all execution and verify the trie root hash match
// what's written in the block header
func (sf *factory) RunActions(
	blockHeight uint64,
	tsf []*action.Transfer,
	vote []*action.Vote,
	executions []*action.Execution) (hash.Hash32B, error) {
	if sf.run {
		// RunActions() already called in MintNewBlock()
		return sf.rootHash, nil
	}

	defer func() {
		sf.run = true
	}()
	// Recover cachedCandidates after restart factory
	if blockHeight > 0 && len(sf.cachedCandidates) == 0 {
		candidates, err := sf.getCandidates(blockHeight - 1)
		if err != nil {
			return sf.rootHash, errors.Wrapf(err, "failed to get previous candidates on height %d", blockHeight-1)
		}
		if sf.cachedCandidates, err = CandidatesToMap(candidates); err != nil {
			return sf.rootHash, errors.Wrap(err, "failed to convert candidate list to map of cached candidates")
		}
	}

	if err := sf.handleTsf(tsf); err != nil {
		return sf.rootHash, errors.Wrap(err, "failed to handle transfers")
	}
	if err := sf.handleVote(blockHeight, vote); err != nil {
		return sf.rootHash, errors.Wrap(err, "failed to handle votes")
	}

	// update pending state changes to trie
	for addr, state := range sf.cachedAccount {
		if err := sf.putState(addr[:], state); err != nil {
			return sf.rootHash, errors.Wrap(err, "failed to update pending state changes to trie")
		}
		// Perform vote update operation on candidate and delegate pools
		if !state.IsCandidate {
			// remove the candidate if the person is not a candidate anymore
			if _, ok := sf.cachedCandidates[addr]; ok {
				delete(sf.cachedCandidates, addr)
			}
			continue
		}
		totalWeight := big.NewInt(0)
		totalWeight.Add(totalWeight, state.VotingWeight)
		voteeAddr, _ := iotxaddress.GetPubkeyHash(state.Votee)
		if addr == byteutil.BytesTo20B(voteeAddr) {
			totalWeight.Add(totalWeight, state.Balance)
		}
		sf.updateCandidate(addr, totalWeight, blockHeight)
	}
	// update pending contract changes
	for addr, contract := range sf.cachedContract {
		if err := contract.Commit(); err != nil {
			return sf.rootHash, errors.Wrap(err, "failed to update pending contract changes")
		}
		state := contract.SelfState()
		// store the account (with new storage trie root) into state trie
		if err := sf.putState(addr[:], state); err != nil {
			return sf.rootHash, errors.Wrap(err, "failed to update pending contract state changes to trie")
		}
	}
	// increase Executor's Nonce for every execution in this block
	for _, e := range executions {
		addr, _ := iotxaddress.GetPubkeyHash(e.Executor())
		state, err := sf.cachedState(byteutil.BytesTo20B(addr))
		if err != nil {
			return sf.rootHash, errors.Wrap(err, "executor does not exist")
		}
		// save state before modifying
		sf.saveState(e.Executor(), state)
		if e.Nonce() > state.Nonce {
			state.Nonce = e.Nonce()
		}
		if err := sf.putState(addr, state); err != nil {
			return sf.rootHash, errors.Wrap(err, "failed to update pending state changes to trie")
		}
	}
	// Persist accountTrie's root hash
	sf.rootHash = sf.RootHash()
	if err := sf.dao.Put(trie.AccountKVNameSpace, []byte(AccountTrieRootKey), sf.rootHash[:]); err != nil {
		return sf.rootHash, errors.Wrap(err, "failed to store accountTrie's root hash")
	}
	// Persist new list of candidates
	candidates, err := MapToCandidates(sf.cachedCandidates)
	if err != nil {
		return sf.rootHash, errors.Wrap(err, "failed to convert map of cached candidates to candidate list")
	}
	sort.Sort(candidates)
	candidatesBytes, err := Serialize(candidates)
	if err != nil {
		return sf.rootHash, errors.Wrap(err, "failed to serialize candidates")
	}
	if err := sf.dao.Put(trie.CandidateKVNameSpace, byteutil.Uint64ToBytes(blockHeight), candidatesBytes); err != nil {
		return sf.rootHash, errors.Wrapf(err, "failed to store candidates on height %d", blockHeight)
	}
	// Persist current chain height
	sf.currentChainHeight = blockHeight
	if err := sf.dao.Put(trie.AccountKVNameSpace, []byte(CurrentHeightKey), byteutil.Uint64ToBytes(blockHeight)); err != nil {
		return sf.rootHash, errors.Wrap(err, "failed to store accountTrie's current height")
	}
	return sf.rootHash, nil
}

// HasRun return the run status
func (sf *factory) HasRun() bool {
	return sf.run
}

// Commit persists all changes in RunActions() into the DB
func (sf *factory) Commit() error {
	// commit all changes in a batch
	if err := sf.accountTrie.Commit(); err != nil {
		return errors.Wrap(err, "failed to commit all changes to underlying DB in a batch")
	}
	sf.clearCache()
	sf.run = false
	return nil
}

//======================================
// Contract functions
//======================================
// GetCodeHash returns contract's code hash
func (sf *factory) GetCodeHash(addr hash.AddrHash) (hash.Hash32B, error) {
	if contract, ok := sf.cachedContract[addr]; ok {
		return byteutil.BytesTo32B(contract.SelfState().CodeHash), nil
	}
	state, err := sf.cachedState(addr)
	if err != nil {
		return hash.ZeroHash32B, errors.Wrapf(err, "failed to GetCodeHash for contract %x", addr)
	}
	return byteutil.BytesTo32B(state.CodeHash), nil
}

// GetCode returns contract's code
func (sf *factory) GetCode(addr hash.AddrHash) ([]byte, error) {
	if contract, ok := sf.cachedContract[addr]; ok {
		return contract.GetCode()
	}
	state, err := sf.cachedState(addr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to GetCode for contract %x", addr)
	}
	return sf.dao.Get(trie.CodeKVNameSpace, state.CodeHash[:])
}

// SetCode sets contract's code
func (sf *factory) SetCode(addr hash.AddrHash, code []byte) error {
	if contract, ok := sf.cachedContract[addr]; ok {
		contract.SetCode(byteutil.BytesTo32B(hash.Hash256b(code)), code)
		return nil
	}
	contract, err := sf.getContract(addr)
	if err != nil {
		return errors.Wrapf(err, "failed to SetCode for contract %x", addr)
	}
	contract.SetCode(byteutil.BytesTo32B(hash.Hash256b(code)), code)
	return nil
}

// GetContractState returns contract's storage value
func (sf *factory) GetContractState(addr hash.AddrHash, key hash.Hash32B) (hash.Hash32B, error) {
	if contract, ok := sf.cachedContract[addr]; ok {
		v, err := contract.GetState(key)
		return byteutil.BytesTo32B(v), err
	}
	contract, err := sf.getContract(addr)
	if err != nil {
		return hash.ZeroHash32B, errors.Wrapf(err, "failed to GetContractState for contract %x", addr)
	}
	v, err := contract.GetState(key)
	return byteutil.BytesTo32B(v), err
}

// SetContractState writes contract's storage value
func (sf *factory) SetContractState(addr hash.AddrHash, key, value hash.Hash32B) error {
	if contract, ok := sf.cachedContract[addr]; ok {
		return contract.SetState(key, value[:])
	}
	contract, err := sf.getContract(addr)
	if err != nil {
		return errors.Wrapf(err, "failed to SetContractState for contract %x", addr)
	}
	return contract.SetState(key, value[:])
}

//======================================
// Candidate functions
//======================================
// Candidates returns array of candidates in candidate pool
func (sf *factory) Candidates() (uint64, []*Candidate) {
	candidates, _ := MapToCandidates(sf.cachedCandidates)
	if len(candidates) <= int(sf.numCandidates) {
		return sf.currentChainHeight, candidates
	}
	sort.Sort(candidates)
	return sf.currentChainHeight, candidates[:sf.numCandidates]
}

// CandidatesByHeight returns array of candidates in candidate pool of a given height
func (sf *factory) CandidatesByHeight(height uint64) ([]*Candidate, error) {
	// Load candidates on the given height from underlying db
	candidates, err := sf.getCandidates(height)
	if err != nil {
		return []*Candidate{}, errors.Wrapf(err, "failed to get candidates on height %d", height)
	}
	if len(candidates) > int(sf.numCandidates) {
		candidates = candidates[:sf.numCandidates]
	}
	return candidates, nil
}

//======================================
// private state/account functions
//======================================
// getState pulls a State from DB
func (sf *factory) getState(hash hash.AddrHash) (*State, error) {
	mstate, err := sf.accountTrie.Get(hash[:])
	if errors.Cause(err) == trie.ErrNotExist {
		return nil, errors.Wrapf(ErrAccountNotExist, "addrHash = %x", hash[:])
	}
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get state of %x", hash)
	}
	return bytesToState(mstate)
}

func (sf *factory) cachedState(hash hash.AddrHash) (*State, error) {
	if state, ok := sf.cachedAccount[hash]; ok {
		return state, nil
	}
	// add to local cache
	state, err := sf.getState(hash)
	if state != nil {
		sf.cachedAccount[hash] = state
	}
	return state, err
}

// getState stores a State to DB
func (sf *factory) putState(addr []byte, state *State) error {
	ss, err := stateToBytes(state)
	if err != nil {
		return errors.Wrapf(err, "failed to convert state %v to bytes", state)
	}
	return sf.accountTrie.Upsert(addr, ss)
}

func (sf *factory) saveState(addr string, state *State) {
	if _, ok := sf.savedAccount[addr]; !ok {
		sf.savedAccount[addr] = state.clone()
	}
}

func (sf *factory) getContract(addr hash.AddrHash) (Contract, error) {
	state, err := sf.cachedState(addr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get the cached state of %x", addr)
	}
	delete(sf.cachedAccount, addr)
	if state.Root == hash.ZeroHash32B {
		state.Root = trie.EmptyRoot
	}
	tr, err := trie.NewTrieSharedDB(sf.dao, trie.ContractKVNameSpace, state.Root)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create storage trie for new contract %x", addr)
	}
	// add to contract cache
	contract := newContract(state, tr)
	sf.cachedContract[addr] = contract
	return contract, nil
}

// clearCache removes all local changes after committing to trie
func (sf *factory) clearCache() {
	sf.savedAccount = nil
	sf.cachedAccount = nil
	sf.cachedContract = nil
	sf.savedAccount = make(map[string]*State)
	sf.cachedAccount = make(map[hash.AddrHash]*State)
	sf.cachedContract = make(map[hash.AddrHash]Contract)
}

//======================================
// private candidate functions
//======================================
func (sf *factory) updateCandidate(pkHash hash.AddrHash, totalWeight *big.Int, blockHeight uint64) {
	// Candidate was added when self-nomination, always exist in cachedCandidates
	candidate, _ := sf.cachedCandidates[pkHash]
	candidate.Votes = totalWeight
	candidate.LastUpdateHeight = blockHeight
}

func (sf *factory) getCandidates(height uint64) (CandidateList, error) {
	candidatesBytes, err := sf.dao.Get(trie.CandidateKVNameSpace, byteutil.Uint64ToBytes(height))
	if err != nil {
		return []*Candidate{}, errors.Wrapf(err, "failed to get candidates on height %d", height)
	}
	return Deserialize(candidatesBytes)
}

//======================================
// private transfer/vote functions
//======================================
func (sf *factory) handleTsf(tsf []*action.Transfer) error {
	for _, tx := range tsf {
		if tx.IsContract() {
			continue
		}
		if !tx.IsCoinbase() {
			// check sender
			sender, err := sf.LoadOrCreateState(tx.Sender(), 0)
			if err != nil {
				return errors.Wrapf(err, "failed to load or create the state of sender %s", tx.Sender)
			}
			// save state before modifying
			sf.saveState(tx.Sender(), sender)
			if tx.Amount().Cmp(sender.Balance) == 1 {
				return errors.Wrapf(ErrNotEnoughBalance, "failed to verify the balance of sender %s", tx.Sender)
			}
			// update sender balance
			if err := sender.SubBalance(tx.Amount()); err != nil {
				return errors.Wrapf(err, "failed to update the balance of sender %s", tx.Sender)
			}
			// update sender nonce
			if tx.Nonce() > sender.Nonce {
				sender.Nonce = tx.Nonce()
			}
			// Update sender votes
			if len(sender.Votee) > 0 && sender.Votee != tx.Sender() {
				// sender already voted to a different person
				voteeOfSender, err := sf.LoadOrCreateState(sender.Votee, 0)
				if err != nil {
					return errors.Wrapf(err, "failed to load or create the state of sender's votee %s", sender.Votee)
				}
				// save state before modifying
				sf.saveState(sender.Votee, voteeOfSender)
				voteeOfSender.VotingWeight.Sub(voteeOfSender.VotingWeight, tx.Amount())
			}
		}
		// check recipient
		recipient, err := sf.LoadOrCreateState(tx.Recipient(), 0)
		if err != nil {
			return errors.Wrapf(err, "failed to laod or create the state of recipient %s", tx.Recipient())
		}
		// save state before modifying
		sf.saveState(tx.Recipient(), recipient)
		// update recipient balance
		if err := recipient.AddBalance(tx.Amount()); err != nil {
			return errors.Wrapf(err, "failed to update the balance of recipient %s", tx.Recipient)
		}
		// Update recipient votes
		if len(recipient.Votee) > 0 && recipient.Votee != tx.Recipient() {
			// recipient already voted to a different person
			voteeOfRecipient, err := sf.LoadOrCreateState(recipient.Votee, 0)
			if err != nil {
				return errors.Wrapf(err, "failed to load or create the state of recipient's votee %s", recipient.Votee)
			}
			// save state before modifying
			sf.saveState(recipient.Votee, voteeOfRecipient)
			voteeOfRecipient.VotingWeight.Add(voteeOfRecipient.VotingWeight, tx.Amount())
		}
	}
	return nil
}

func (sf *factory) handleVote(blockHeight uint64, vote []*action.Vote) error {
	for _, v := range vote {
		voteFrom, err := sf.LoadOrCreateState(v.Voter(), 0)
		if err != nil {
			return errors.Wrapf(err, "failed to load or create the state of voter %s", v.Voter())
		}
		// save state before modifying
		sf.saveState(v.Voter(), voteFrom)
		// update voteFrom nonce
		if v.Nonce() > voteFrom.Nonce {
			voteFrom.Nonce = v.Nonce()
		}
		// Update old votee's weight
		if len(voteFrom.Votee) > 0 && voteFrom.Votee != v.Voter() {
			// voter already voted
			oldVotee, err := sf.LoadOrCreateState(voteFrom.Votee, 0)
			if err != nil {
				return errors.Wrapf(err, "failed to load or create the state of voter's old votee %s", voteFrom.Votee)
			}
			// save state before modifying
			sf.saveState(voteFrom.Votee, oldVotee)
			oldVotee.VotingWeight.Sub(oldVotee.VotingWeight, voteFrom.Balance)
			voteFrom.Votee = ""
		}

		if v.Votee() == "" {
			// unvote operation
			voteFrom.IsCandidate = false
			continue
		}

		voteTo, err := sf.LoadOrCreateState(v.Votee(), 0)
		if err != nil {
			return errors.Wrapf(err, "failed to load or create the state of votee %s", v.Votee())
		}
		// save state before modifying
		sf.saveState(v.Votee(), voteTo)
		if v.Voter() != v.Votee() {
			// Voter votes to a different person
			voteTo.VotingWeight.Add(voteTo.VotingWeight, voteFrom.Balance)
			voteFrom.Votee = v.Votee()
		} else {
			// Vote to self: self-nomination or cancel the previous vote case
			voteFrom.Votee = v.Voter()
			voteFrom.IsCandidate = true
			pkHash, err := iotxaddress.GetPubkeyHash(v.Voter())
			if err != nil {
				return errors.Wrap(err, "cannot get the hash of the address")
			}
			pkHashAddress := byteutil.BytesTo20B(pkHash)
			votePubkey := v.VoterPublicKey()
			if _, ok := sf.cachedCandidates[pkHashAddress]; !ok {
				sf.cachedCandidates[pkHashAddress] = &Candidate{
					Address:        v.Voter(),
					PubKey:         votePubkey[:],
					CreationHeight: blockHeight,
				}
			}
		}
	}
	return nil
}

//======================================
// private trie constructor functions
//======================================
func (sf *factory) getRoot(nameSpace string, key string) (hash.Hash32B, error) {
	var trieRoot hash.Hash32B
	switch root, err := sf.dao.Get(nameSpace, []byte(key)); errors.Cause(err) {
	case nil:
		trieRoot = byteutil.BytesTo32B(root)
	case bolt.ErrBucketNotFound:
		trieRoot = trie.EmptyRoot
	default:
		return hash.ZeroHash32B, errors.Wrap(err, "failed to get trie's root hash from underlying db")
	}
	return trieRoot, nil
}
