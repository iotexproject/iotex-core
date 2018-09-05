// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package state

import (
	"context"
	"github.com/boltdb/bolt"
	"github.com/pkg/errors"
	"math/big"
	"sort"

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
	// CurrentHeightKey indicates the key of current factory height in underlying database
	CurrentHeightKey = "currentHeight"
	// AccountTrieRootKey indicates the key of accountTrie root hash in underlying database
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
		CommitStateChanges(uint64, []*action.Transfer, []*action.Vote, []*action.Execution) error
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

	// factory implements StateFactory interface, tracks changes in a map and batch-commits to trie/db
	factory struct {
		lifecycle lifecycle.Lifecycle
		// candidate pool
		currentChainHeight uint64
		numCandidates      uint
		cachedCandidates   map[hash.AddrHash]*Candidate
		// accounts
		cachedAccount  map[hash.AddrHash]*State   // accounts being modified in this Tx
		cachedContract map[hash.AddrHash]Contract // contracts being modified in this Tx
		accountTrie    trie.Trie                  // global state trie
		contractTrie   trie.Trie                  // contract storage trie
	}
)

// FactoryOption sets Factory construction parameter
type FactoryOption func(*factory, *config.Config) error

// PrecreatedTrieOption uses pre-created tries for state factory
func PrecreatedTrieOption(accountTrie trie.Trie) FactoryOption {
	return func(sf *factory, cfg *config.Config) error {
		sf.accountTrie = accountTrie
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
		// create account trie
		accountTrieRoot, err := sf.getRoot(trieDB, trie.AccountKVNameSpace, AccountTrieRootKey)
		if err != nil {
			return errors.Wrap(err, "failed to get accountTrie's root hash from underlying db")
		}
		tr, err := trie.NewTrie(trieDB, trie.AccountKVNameSpace, accountTrieRoot)
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
		// create account trie
		accountTrieRoot, err := sf.getRoot(trieDB, trie.AccountKVNameSpace, AccountTrieRootKey)
		if err != nil {
			return errors.Wrap(err, "failed to get accountTrie's root hash from underlying db")
		}
		tr, err := trie.NewTrie(trieDB, trie.AccountKVNameSpace, accountTrieRoot)
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
		return nil, err
	}
	return state, nil
}

// Balance returns balance
func (sf *factory) Balance(addr string) (*big.Int, error) {
	pkHash, err := iotxaddress.GetPubkeyHash(addr)
	if err != nil {
		return nil, errors.Wrap(err, "error when getting the pubkey hash")
	}
	state, err := sf.getState(byteutil.BytesTo20B(pkHash))
	if err != nil {
		return nil, err
	}
	return state.Balance, nil
}

// Nonce returns the nonce if the account exists
func (sf *factory) Nonce(addr string) (uint64, error) {
	pkHash, err := iotxaddress.GetPubkeyHash(addr)
	if err != nil {
		return 0, errors.Wrap(err, "error when getting the pubkey hash")
	}
	state, err := sf.getState(byteutil.BytesTo20B(pkHash))
	if err != nil {
		return 0, err
	}
	return state.Nonce, nil
}

// State returns the confirmed state on the chain
func (sf *factory) State(addr string) (*State, error) {
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
	height, err := sf.accountTrie.TrieDB().Get(trie.AccountKVNameSpace, []byte(CurrentHeightKey))
	if err != nil {
		return 0, errors.Wrap(err, "failed to get factory's height from underlying db")
	}
	return byteutil.BytesToUint64(height), nil
}

// CommitStateChanges updates a State from the given actions
func (sf *factory) CommitStateChanges(blockHeight uint64, tsf []*action.Transfer, vote []*action.Vote, executions []*action.Execution) error {
	// Recover cachedCandidates after restart factory
	if blockHeight > 0 && len(sf.cachedCandidates) == 0 {
		candidates, err := sf.getCandidates(blockHeight - 1)
		if err != nil {
			return errors.Wrapf(err, "failed to get previous candidates on height %d", blockHeight-1)
		}
		if sf.cachedCandidates, err = CandidatesToMap(candidates); err != nil {
			return errors.Wrap(err, "failed to convert candidate list to map of cached candidates")
		}
	}

	defer sf.clearCache()
	if err := sf.handleTsf(tsf); err != nil {
		return err
	}
	if err := sf.handleVote(blockHeight, vote); err != nil {
		return err
	}

	// update pending state changes to trie
	for addr, state := range sf.cachedAccount {
		if err := sf.putState(state, addr[:]); err != nil {
			return err
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
			return err
		}
		state := contract.SelfState()
		// store the account (with new storage trie root) into state trie
		if err := sf.putState(state, addr[:]); err != nil {
			return err
		}
	}
	// increase Executor's Nonce for every execution in this block
	for _, e := range executions {
		addr, _ := iotxaddress.GetPubkeyHash(e.Executor)
		state, err := sf.cachedState(byteutil.BytesTo20B(addr))
		if err != nil {
			return errors.Wrap(err, "Executor does not exist")
		}
		if e.Nonce > state.Nonce {
			state.Nonce = e.Nonce
		}
		if err := sf.putState(state, addr); err != nil {
			return err
		}
	}
	// commit to account Trie in a batch
	if err := sf.accountTrie.Commit(); err != nil {
		return errors.Wrap(err, "failed to commit changes to account Trie in a batch")
	}

	trieDB := sf.accountTrie.TrieDB()
	// Persist accountTrie's root hash
	accountRootHash := sf.RootHash()
	if err := trieDB.Put(trie.AccountKVNameSpace, []byte(AccountTrieRootKey), accountRootHash[:]); err != nil {
		return errors.Wrap(err, "failed to update accountTrie's root hash in underlying db")
	}

	// Persist new list of candidates to underlying db
	candidates, err := MapToCandidates(sf.cachedCandidates)
	if err != nil {
		return errors.Wrap(err, "failed to convert map of cached candidates to candidate list")
	}
	sort.Sort(candidates)
	candidatesBytes, err := Serialize(candidates)
	if err != nil {
		return errors.Wrap(err, "failed to serialize candidates")
	}
	if err := trieDB.Put(trie.CandidateKVNameSpace, byteutil.Uint64ToBytes(blockHeight), candidatesBytes); err != nil {
		return errors.Wrapf(err, "failed to store candidates on height %d into underlying db", blockHeight)
	}

	// Set current chain height and persist it to db
	sf.currentChainHeight = blockHeight
	return trieDB.Put(trie.AccountKVNameSpace, []byte(CurrentHeightKey), byteutil.Uint64ToBytes(blockHeight))
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
		return hash.ZeroHash32B, errors.Wrapf(err, "Failed to GetCodeHash for contract %x", addr)
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
		return nil, errors.Wrapf(err, "Failed to GetCode for contract %x", addr)
	}
	return sf.accountTrie.TrieDB().Get(trie.CodeKVNameSpace, state.CodeHash[:])
}

// SetCode sets contract's code
func (sf *factory) SetCode(addr hash.AddrHash, code []byte) error {
	if contract, ok := sf.cachedContract[addr]; ok {
		contract.SetCode(byteutil.BytesTo32B(hash.Hash256b(code)), code)
		return nil
	}
	contract, err := sf.getContract(addr)
	if err != nil {
		return errors.Wrapf(err, "Failed to SetCode for contract %x", addr)
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
		return hash.ZeroHash32B, errors.Wrapf(err, "Failed to GetContractState for contract %x", addr)
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
		return errors.Wrapf(err, "Failed to SetContractState for contract %x", addr)
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
		return nil, err
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
func (sf *factory) putState(state *State, addr []byte) error {
	ss, err := stateToBytes(state)
	if err != nil {
		return err
	}
	if err := sf.accountTrie.Upsert(addr, ss); err != nil {
		return err
	}
	return nil
}

func (sf *factory) getContract(addr hash.AddrHash) (Contract, error) {
	state, err := sf.cachedState(addr)
	if err != nil {
		return nil, err
	}
	logger.Warn().Msgf("promote contract %x", addr)
	delete(sf.cachedAccount, addr)
	if state.Root == hash.ZeroHash32B {
		state.Root = trie.EmptyRoot
	}
	tr, err := trie.NewTrie(sf.accountTrie.TrieDB(), trie.ContractKVNameSpace, state.Root)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to create storage trie for new contract %x", addr)
	}
	// add to contract cache
	contract := newContract(state, tr)
	sf.cachedContract[addr] = contract
	return contract, nil
}

// clearCache removes all local changes after committing to trie
func (sf *factory) clearCache() {
	sf.cachedAccount = nil
	sf.cachedContract = nil
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
	candidatesBytes, err := sf.accountTrie.TrieDB().Get(trie.CandidateKVNameSpace, byteutil.Uint64ToBytes(height))
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
		if !tx.IsCoinbase {
			// check sender
			sender, err := sf.LoadOrCreateState(tx.Sender, 0)
			if err != nil {
				return err
			}
			if tx.Amount.Cmp(sender.Balance) == 1 {
				return ErrNotEnoughBalance
			}
			// update sender balance
			if err := sender.SubBalance(tx.Amount); err != nil {
				return err
			}
			// update sender nonce
			if tx.Nonce > sender.Nonce {
				sender.Nonce = tx.Nonce
			}
			// Update sender votes
			if len(sender.Votee) > 0 && sender.Votee != tx.Sender {
				// sender already voted to a different person
				voteeOfSender, err := sf.LoadOrCreateState(sender.Votee, 0)
				if err != nil {
					return err
				}
				voteeOfSender.VotingWeight.Sub(voteeOfSender.VotingWeight, tx.Amount)
			}
		}
		// check recipient
		recipient, err := sf.LoadOrCreateState(tx.Recipient, 0)
		if err != nil {
			return err
		}
		// update recipient balance
		if err := recipient.AddBalance(tx.Amount); err != nil {
			return err
		}
		// Update recipient votes
		if len(recipient.Votee) > 0 && recipient.Votee != tx.Recipient {
			// recipient already voted to a different person
			voteeOfRecipient, err := sf.LoadOrCreateState(recipient.Votee, 0)
			if err != nil {
				return err
			}
			voteeOfRecipient.VotingWeight.Add(voteeOfRecipient.VotingWeight, tx.Amount)
		}
	}
	return nil
}

func (sf *factory) handleVote(blockHeight uint64, vote []*action.Vote) error {
	for _, v := range vote {
		pbVote := v.GetVote()
		voterAddress := pbVote.VoterAddress
		voteFrom, err := sf.LoadOrCreateState(voterAddress, 0)
		if err != nil {
			return err
		}

		// update voteFrom nonce
		if v.Nonce > voteFrom.Nonce {
			voteFrom.Nonce = v.Nonce
		}
		// Update old votee's weight
		if len(voteFrom.Votee) > 0 && voteFrom.Votee != voterAddress {
			// voter already voted
			oldVotee, err := sf.LoadOrCreateState(voteFrom.Votee, 0)
			if err != nil {
				return err
			}
			oldVotee.VotingWeight.Sub(oldVotee.VotingWeight, voteFrom.Balance)
			voteFrom.Votee = ""
		}

		voteeAddress := pbVote.VoteeAddress
		if voteeAddress == "" {
			// unvote operation
			voteFrom.IsCandidate = false
			continue
		}

		voteTo, err := sf.LoadOrCreateState(voteeAddress, 0)
		if err != nil {
			return err
		}

		if voterAddress != voteeAddress {
			// Voter votes to a different person
			voteTo.VotingWeight.Add(voteTo.VotingWeight, voteFrom.Balance)
			voteFrom.Votee = voteeAddress
		} else {
			// Vote to self: self-nomination or cancel the previous vote case
			voteFrom.Votee = voterAddress
			voteFrom.IsCandidate = true
			pkHash, err := iotxaddress.GetPubkeyHash(voterAddress)
			if err != nil {
				return errors.Wrap(err, "cannot get the hash of the address")
			}
			pkHashAddress := byteutil.BytesTo20B(pkHash)
			if _, ok := sf.cachedCandidates[pkHashAddress]; !ok {
				sf.cachedCandidates[pkHashAddress] = &Candidate{
					Address:        voterAddress,
					PubKey:         pbVote.SelfPubkey[:],
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
func (sf *factory) getRoot(trieDB db.KVStore, nameSpace string, key string) (hash.Hash32B, error) {
	var trieRoot hash.Hash32B
	switch root, err := trieDB.Get(nameSpace, []byte(key)); errors.Cause(err) {
	case nil:
		trieRoot = byteutil.BytesTo32B(root)
	case bolt.ErrBucketNotFound:
		trieRoot = trie.EmptyRoot
	default:
		return hash.ZeroHash32B, errors.Wrap(err, "failed to get trie's root hash from underlying db")
	}
	return trieRoot, nil
}
