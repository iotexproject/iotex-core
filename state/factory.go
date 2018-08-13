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
	"strings"

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
	ErrAccountNotExist = errors.New("the account does not exist")

	// ErrAccountCollision is the error that the account already exists
	ErrAccountCollision = errors.New("the account already exists")

	// ErrFailedToMarshalState is the error that the state marshaling is failed
	ErrFailedToMarshalState = errors.New("failed to marshal state")

	// ErrFailedToUnmarshalState is the error that the state un-marshaling is failed
	ErrFailedToUnmarshalState = errors.New("failed to unmarshal state")
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
		RootHash() hash.Hash32B
		CommitStateChanges(uint64, []*action.Transfer, []*action.Vote) error
		// Contracts
		CreateContract(addr string, code []byte) (string, error)
		GetCodeHash(addr string) hash.Hash32B
		GetCode(addr string) []byte
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
		cachedCandidates   map[string]*Candidate
		// accounts
		cachedAccount map[string]*State // accounts being modified in this Tx
		accountTrie   trie.Trie         // global state trie
		contractTrie  trie.Trie         // contract storage trie
		candidateTrie trie.Trie         // candidate storage trie
		codeDB        db.KVStore        // code storage DB
	}
)

// FactoryOption sets Factory construction parameter
type FactoryOption func(*factory, *config.Config) error

// PrecreatedTrieOption uses pre-created tries for state factory
func PrecreatedTrieOption(accountTrie, candidateTrie trie.Trie) FactoryOption {
	return func(sf *factory, cfg *config.Config) error {
		sf.accountTrie = accountTrie
		sf.candidateTrie = candidateTrie
		sf.codeDB = accountTrie.TrieDB()
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
		tr, err := trie.NewTrie(db.NewBoltDB(dbPath, nil), trie.AccountKVNameSpace, trie.EmptyRoot)
		if err != nil {
			return errors.Wrap(err, "failed to generate accountTrie from config")
		}
		sf.accountTrie = tr
		candidateTrie, err := trie.NewTrie(tr.TrieDB(), trie.CandidateKVNameSpace, trie.EmptyRoot)
		if err != nil {
			return errors.Wrap(err, "failed to generate candidateTrie")
		}
		sf.candidateTrie = candidateTrie
		sf.codeDB = tr.TrieDB()
		return nil
	}
}

// InMemTrieOption creates in memory trie for state factory
func InMemTrieOption() FactoryOption {
	return func(sf *factory, cfg *config.Config) error {
		tr, err := trie.NewTrie(db.NewMemKVStore(), trie.AccountKVNameSpace, trie.EmptyRoot)
		if err != nil {
			return errors.Wrapf(err, "failed to initialize in-memory accountTrie")
		}
		sf.accountTrie = tr
		candidateTrie, err := trie.NewTrie(tr.TrieDB(), trie.CandidateKVNameSpace, trie.EmptyRoot)
		if err != nil {
			return errors.Wrap(err, "failed to initialize in-memory candidateTrie")
		}
		sf.candidateTrie = candidateTrie
		sf.codeDB = tr.TrieDB()
		return nil
	}
}

// NewFactory creates a new state factory
func NewFactory(cfg *config.Config, opts ...FactoryOption) (Factory, error) {
	sf := &factory{
		currentChainHeight: 0,
		numCandidates:      cfg.Chain.NumCandidates,
		cachedCandidates:   make(map[string]*Candidate),
		cachedAccount:      make(map[string]*State),
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
	if sf.candidateTrie != nil {
		sf.lifecycle.Add(sf.candidateTrie)
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
	// only create if the address did not exist yet
	state, err := sf.getState(addr)
	switch {
	case errors.Cause(err) == ErrAccountNotExist:
		if state, err = sf.createState(addr, init); err != nil {
			return nil, err
		}
	case err != nil:
		return nil, err
	}
	return state, nil
}

// Balance returns balance
func (sf *factory) Balance(addr string) (*big.Int, error) {
	state, err := sf.getState(addr)
	if err != nil {
		return nil, err
	}
	return state.Balance, nil
}

// Nonce returns the nonce if the account exists
func (sf *factory) Nonce(addr string) (uint64, error) {
	state, err := sf.getState(addr)
	if err != nil {
		return 0, err
	}
	return state.Nonce, nil
}

// State returns the state if the address exists
func (sf *factory) State(addr string) (*State, error) {
	return sf.getState(addr)
}

// RootHash returns the hash of the root node of the trie
func (sf *factory) RootHash() hash.Hash32B {
	return sf.accountTrie.RootHash()
}

// CommitStateChanges updates a State from the given actions
func (sf *factory) CommitStateChanges(blockHeight uint64, tsf []*action.Transfer, vote []*action.Vote) error {
	if err := sf.handleTsf(tsf); err != nil {
		return err
	}
	if err := sf.handleVote(blockHeight, vote); err != nil {
		return err
	}

	// update pending state changes to trie
	sf.accountTrie.EnableBatch()
	for address, state := range sf.cachedAccount {
		ss, err := stateToBytes(state)
		if err != nil {
			return err
		}
		pkhash, err := iotxaddress.GetPubkeyHash(address)
		if err != nil {
			return errors.Wrap(err, "error when getting the pubkey hash")
		}
		addr := make([]byte, len(pkhash))
		copy(addr, pkhash[:])
		if err := sf.accountTrie.Upsert(addr, ss); err != nil {
			return err
		}

		// Perform vote update operation on candidate and delegate pools
		if !state.IsCandidate {
			// remove the candidate if the person is not a candidate anymore
			if _, ok := sf.cachedCandidates[address]; ok {
				delete(sf.cachedCandidates, address)
			}
			continue
		}
		totalWeight := big.NewInt(0)
		totalWeight.Add(totalWeight, state.VotingWeight)
		if state.Votee == address {
			totalWeight.Add(totalWeight, state.Balance)
		}
		sf.updateCandidate(address, totalWeight, blockHeight)
	}
	sf.currentChainHeight = blockHeight

	// Persist new list of candidates to candidateTrie
	candidates, err := MapToCandidates(sf.cachedCandidates)
	if err != nil {
		return errors.Wrap(err, "failed to convert candidate map to candidates")
	}
	sort.Sort(candidates)
	candidatesBytes, err := Serialize(candidates)
	if err != nil {
		return errors.Wrap(err, "failed to serialize candidates")
	}
	if err := sf.candidateTrie.Upsert(byteutil.Uint64ToBytes(blockHeight), candidatesBytes); err != nil {
		return errors.Wrapf(err, "failed to insert candidates on height %d into candidateTrie", blockHeight)
	}
	// commit the state changes to Trie in a batch
	return sf.accountTrie.Commit()
}

//======================================
// Contract functions
//======================================
// CreateContract creates a new contract account
func (sf *factory) CreateContract(addr string, code []byte) (string, error) {
	nonce, err := sf.Nonce(addr)
	if err != nil {
		return "", err
	}
	contractAddr, err := iotxaddress.CreateContractAddress(addr, nonce)
	if err != nil {
		return "", err
	}
	// only create when contract addr does not exist yet
	if _, err := sf.getState(contractAddr); errors.Cause(err) != ErrAccountNotExist {
		return "", ErrAccountCollision
	}
	contract, err := sf.createContract(contractAddr, code)
	if err != nil {
		return "", errors.Wrapf(err, "Failed to create contract")
	}
	// put the code into storage DB
	if err := sf.accountTrie.TrieDB().Put(trie.CodeKVNameSpace, contract.CodeHash, code); err != nil {
		return "", errors.Wrapf(err, "Failed to store contract code")
	}
	return contractAddr, nil
}

func (sf *factory) GetCodeHash(addr string) hash.Hash32B {
	codeHash := hash.ZeroHash32B
	state, err := sf.getState(addr)
	if err != nil || state.CodeHash == nil {
		return codeHash
	}
	copy(codeHash[:], state.CodeHash)
	return codeHash
}

func (sf *factory) GetCode(addr string) []byte {
	codeHash := sf.GetCodeHash(addr)
	if codeHash == hash.ZeroHash32B {
		return nil
	}
	// pull the code from storage DB
	code, err := sf.accountTrie.TrieDB().Get(trie.CodeKVNameSpace, codeHash[:])
	if err != nil {
		return nil
	}
	return code
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
	// Load candidates on the given height from candidateTrie
	candidatesBytes, err := sf.candidateTrie.Get(byteutil.Uint64ToBytes(height))
	if err != nil {
		return []*Candidate{}, errors.Wrapf(err, "failed to get candidates on height %d", height)
	}
	candidates, err := Deserialize(candidatesBytes)
	if err != nil {
		return []*Candidate{}, errors.Wrapf(err, "failed to deserialize candidates on height %d", height)
	}
	if len(candidates) > int(sf.numCandidates) {
		candidates = candidates[:sf.numCandidates]
	}
	sort.Slice(candidates, func(i, j int) bool {
		return strings.Compare(candidates[i].Address, candidates[j].Address) < 0
	})
	return candidates, nil
}

//======================================
// private state/account functions
//======================================
// getState pulls an existing State
func (sf *factory) getState(addr string) (*State, error) {
	pubKeyHash, err := iotxaddress.GetPubkeyHash(addr)
	if err != nil {
		return nil, errors.Wrap(err, "error when getting the pubkey hash")
	}
	mstate, err := sf.accountTrie.Get(pubKeyHash)
	if errors.Cause(err) == trie.ErrNotExist {
		return nil, ErrAccountNotExist
	}
	if err != nil {
		return nil, err
	}
	return bytesToState(mstate)
}

func (sf *factory) createState(addr string, init uint64) (*State, error) {
	pubKeyHash, err := iotxaddress.GetPubkeyHash(addr)
	if err != nil {
		return nil, errors.Wrap(err, "error when getting the pubkey hash")
	}
	balance := big.NewInt(0)
	weight := big.NewInt(0)
	balance.SetUint64(init)
	s := State{Balance: balance, VotingWeight: weight}
	mstate, err := stateToBytes(&s)
	if err != nil {
		return nil, err
	}
	if err := sf.accountTrie.Upsert(pubKeyHash, mstate); err != nil {
		return nil, err
	}
	return &s, nil
}

func (sf *factory) cache(addr string) (*State, error) {
	if state, exist := sf.cachedAccount[addr]; exist {
		return state, nil
	}
	state, err := sf.getState(addr)
	switch {
	case errors.Cause(err) == ErrAccountNotExist:
		state, err = sf.LoadOrCreateState(addr, 0)
		if err != nil {
			return nil, err
		}
	case err != nil:
		return nil, err
	}
	sf.cachedAccount[addr] = state
	return state, nil
}

func (sf *factory) createContract(addr string, code []byte) (*State, error) {
	addrHash, err := iotxaddress.GetPubkeyHash(addr)
	if addrHash == nil || err != nil {
		return nil, ErrAccountNotExist
	}
	balance := big.NewInt(0)
	weight := big.NewInt(0)
	s := State{
		Balance:      balance,
		VotingWeight: weight,
		Root:         trie.EmptyRoot,
		CodeHash:     hash.Hash256b(code),
	}
	mstate, err := stateToBytes(&s)
	if err != nil {
		return nil, err
	}
	if err := sf.accountTrie.Upsert(addrHash, mstate); err != nil {
		return nil, err
	}
	return &s, nil
}

//======================================
// private candidate functions
//======================================
func (sf *factory) updateCandidate(address string, totalWeight *big.Int, blockHeight uint64) {
	// Candidate was added when self-nomination, always exist in cachedCandidates
	candidate, _ := sf.cachedCandidates[address]
	candidate.Votes = totalWeight
	candidate.LastUpdateHeight = blockHeight
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
			sender, err := sf.cache(tx.Sender)
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
				voteeOfSender, err := sf.cache(sender.Votee)
				if err != nil {
					return err
				}
				voteeOfSender.VotingWeight.Sub(voteeOfSender.VotingWeight, tx.Amount)
			}
		}
		// check recipient
		recipient, err := sf.cache(tx.Recipient)
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
			voteeOfRecipient, err := sf.cache(recipient.Votee)
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
		voteFrom, err := sf.cache(voterAddress)
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
			oldVotee, err := sf.cache(voteFrom.Votee)
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

		voteTo, err := sf.cache(voteeAddress)
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
			if _, ok := sf.cachedCandidates[voterAddress]; !ok {
				sf.cachedCandidates[voterAddress] = &Candidate{
					Address:        voterAddress,
					PubKey:         pbVote.SelfPubkey[:],
					CreationHeight: blockHeight,
				}
			}
		}
	}
	return nil
}
