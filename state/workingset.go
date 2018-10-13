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

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/blockchain/action"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/trie"
)

type (
	// WorkingSet defines an interface for working set of states changes
	WorkingSet interface {
		// states and actions
		LoadOrCreateState(string, uint64) (*State, error)
		balance(string) (*big.Int, error)
		Nonce(string) (uint64, error) // Note that Nonce starts with 1.
		state(string) (*State, error)
		CachedState(string) (*State, error)
		rootHash() hash.Hash32B
		version() uint64
		height() uint64
		RunActions(uint64, []*action.Transfer, []*action.Vote, []*action.Execution, []action.Action) (hash.Hash32B, error)
		commit() error
		// Contracts
		GetCodeHash(hash.PKHash) (hash.Hash32B, error)
		GetCode(hash.PKHash) ([]byte, error)
		SetCode(hash.PKHash, []byte) error
		GetContractState(hash.PKHash, hash.Hash32B) (hash.Hash32B, error)
		SetContractState(hash.PKHash, hash.Hash32B, hash.Hash32B) error
		// Candidates
		workingCandidates() map[hash.PKHash]*Candidate
		getCandidates(height uint64) (CandidateList, error)
	}

	// workingset implements Workingset interface, tracks pending changes to account/contract in local cache
	workingset struct {
		ver              uint64
		blkHeight        uint64
		cachedCandidates map[hash.PKHash]*Candidate
		savedAccount     map[string]*State        // save account state before being modified in this block
		cachedAccount    map[hash.PKHash]*State   // accounts being modified in this block
		cachedContract   map[hash.PKHash]Contract // contracts being modified in this block
		accountTrie      trie.Trie                // global state trie
		dao              db.CachedKVStore         // the underlying DB for account/contract storage
	}
)

// NewWorkingSet creates a new working set
func NewWorkingSet(version uint64, kv db.KVStore, root hash.Hash32B) (WorkingSet, error) {
	ws := &workingset{
		ver:              version,
		cachedCandidates: make(map[hash.PKHash]*Candidate),
		savedAccount:     make(map[string]*State),
		cachedAccount:    make(map[hash.PKHash]*State),
		cachedContract:   make(map[hash.PKHash]Contract),
		dao:              db.NewCachedKVStore(kv),
	}
	tr, err := trie.NewTrieSharedDB(ws.dao, trie.AccountKVNameSpace, root)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate state trie from config")
	}
	ws.accountTrie = tr
	if err := ws.accountTrie.Start(context.Background()); err != nil {
		return nil, errors.Wrapf(err, "failed to load state trie from root = %x", root)
	}
	return ws, nil
}

func (ws *workingset) workingCandidates() map[hash.PKHash]*Candidate {
	return ws.cachedCandidates
}

//======================================
// State/Account functions
//======================================
// LoadOrCreateState loads existing or adds a new State with initial balance to the factory
// addr should be a bech32 properly-encoded string
func (ws *workingset) LoadOrCreateState(addr string, init uint64) (*State, error) {
	h, err := iotxaddress.GetPubkeyHash(addr)
	if err != nil {
		return nil, errors.Wrap(err, "error when getting the pubkey hash")
	}
	addrHash := byteutil.BytesTo20B(h)
	state, err := ws.cachedState(addrHash)
	switch {
	case errors.Cause(err) == ErrAccountNotExist:
		balance := big.NewInt(0)
		balance.SetUint64(init)
		state = &State{
			Balance:      balance,
			VotingWeight: big.NewInt(0),
		}
		ws.cachedAccount[addrHash] = state
	case err != nil:
		return nil, errors.Wrapf(err, "failed to get state of %x from cached state", addrHash)
	}
	return state, nil
}

// Balance returns balance
func (ws *workingset) balance(addr string) (*big.Int, error) {
	if saved, ok := ws.savedAccount[addr]; ok {
		return saved.Balance, nil
	}
	pkHash, err := iotxaddress.GetPubkeyHash(addr)
	if err != nil {
		return nil, errors.Wrap(err, "error when getting the pubkey hash")
	}
	state, err := ws.getState(byteutil.BytesTo20B(pkHash))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get state of %x", pkHash)
	}
	return state.Balance, nil
}

// Nonce returns the Nonce if the account exists
func (ws *workingset) Nonce(addr string) (uint64, error) {
	if saved, ok := ws.savedAccount[addr]; ok {
		return saved.Nonce, nil
	}
	pkHash, err := iotxaddress.GetPubkeyHash(addr)
	if err != nil {
		return 0, errors.Wrap(err, "error when getting the pubkey hash")
	}
	state, err := ws.getState(byteutil.BytesTo20B(pkHash))
	if err != nil {
		return 0, errors.Wrapf(err, "failed to get state of %x", pkHash)
	}
	return state.Nonce, nil
}

// State returns the confirmed state on the chain
func (ws *workingset) state(addr string) (*State, error) {
	if saved, ok := ws.savedAccount[addr]; ok {
		return saved, nil
	}
	pkHash, err := iotxaddress.GetPubkeyHash(addr)
	if err != nil {
		return nil, errors.Wrap(err, "error when getting the pubkey hash")
	}
	return ws.getState(byteutil.BytesTo20B(pkHash))
}

// CachedState returns the cached state if the address exists in local cache
func (ws *workingset) CachedState(addr string) (*State, error) {
	h, err := iotxaddress.GetPubkeyHash(addr)
	if err != nil {
		return nil, errors.Wrap(err, "error when getting the pubkey hash")
	}
	addrHash := byteutil.BytesTo20B(h)
	if contract, ok := ws.cachedContract[addrHash]; ok {
		return contract.SelfState(), nil
	}
	return ws.cachedState(addrHash)
}

// RootHash returns the hash of the root node of the accountTrie
func (ws *workingset) rootHash() hash.Hash32B {
	return ws.accountTrie.RootHash()
}

// version returns the version of this working set
func (ws *workingset) version() uint64 {
	return ws.ver
}

// Height returns the height of the block being worked on
func (ws *workingset) height() uint64 {
	return ws.blkHeight
}

// RunActions runs actions in the block and track pending changes in working set
func (ws *workingset) RunActions(
	blockHeight uint64,
	tsf []*action.Transfer,
	vote []*action.Vote,
	executions []*action.Execution,
	actions []action.Action) (hash.Hash32B, error) {
	ws.blkHeight = blockHeight
	// Recover cachedCandidates after restart factory
	if blockHeight > 0 && len(ws.cachedCandidates) == 0 {
		candidates, err := ws.getCandidates(blockHeight - 1)
		if err != nil {
			return hash.ZeroHash32B, errors.Wrapf(err, "failed to get previous candidates on height %d", blockHeight-1)
		}
		if ws.cachedCandidates, err = CandidatesToMap(candidates); err != nil {
			return hash.ZeroHash32B, errors.Wrap(err, "failed to convert candidate list to map of cached candidates")
		}
	}
	if err := ws.handleTsf(tsf); err != nil {
		return hash.ZeroHash32B, errors.Wrap(err, "failed to handle transfers")
	}
	if err := ws.handleVote(blockHeight, vote); err != nil {
		return hash.ZeroHash32B, errors.Wrap(err, "failed to handle votes")
	}

	// update pending state changes to trie
	for addr, state := range ws.cachedAccount {
		if err := ws.putState(addr[:], state); err != nil {
			return hash.ZeroHash32B, errors.Wrap(err, "failed to update pending state changes to trie")
		}
		// Perform vote update operation on candidate and delegate pools
		if !state.IsCandidate {
			// remove the candidate if the person is not a candidate anymore
			if _, ok := ws.cachedCandidates[addr]; ok {
				delete(ws.cachedCandidates, addr)
			}
			continue
		}
		totalWeight := big.NewInt(0)
		totalWeight.Add(totalWeight, state.VotingWeight)
		voteeAddr, _ := iotxaddress.GetPubkeyHash(state.Votee)
		if addr == byteutil.BytesTo20B(voteeAddr) {
			totalWeight.Add(totalWeight, state.Balance)
		}
		ws.updateCandidate(addr, totalWeight, blockHeight)
	}
	// update pending contract changes
	for addr, contract := range ws.cachedContract {
		if err := contract.Commit(); err != nil {
			return hash.ZeroHash32B, errors.Wrap(err, "failed to update pending contract changes")
		}
		state := contract.SelfState()
		// store the account (with new storage trie root) into state trie
		if err := ws.putState(addr[:], state); err != nil {
			return hash.ZeroHash32B, errors.Wrap(err, "failed to update pending contract state changes to trie")
		}
	}
	// increase Executor's Nonce for every execution in this block
	for _, e := range executions {
		addr, _ := iotxaddress.GetPubkeyHash(e.Executor())
		state, err := ws.cachedState(byteutil.BytesTo20B(addr))
		if err != nil {
			return hash.ZeroHash32B, errors.Wrap(err, "executor does not exist")
		}
		// save state before modifying
		ws.saveState(e.Executor(), state)
		if e.Nonce() > state.Nonce {
			state.Nonce = e.Nonce()
		}
		if err := ws.putState(addr, state); err != nil {
			return hash.ZeroHash32B, errors.Wrap(err, "failed to update pending state changes to trie")
		}
	}
	// Persist accountTrie's root hash
	rootHash := ws.accountTrie.RootHash()
	if err := ws.dao.Put(trie.AccountKVNameSpace, []byte(AccountTrieRootKey), rootHash[:]); err != nil {
		return hash.ZeroHash32B, errors.Wrap(err, "failed to store accountTrie's root hash")
	}
	// Persist new list of candidates
	candidates, err := MapToCandidates(ws.cachedCandidates)
	if err != nil {
		return hash.ZeroHash32B, errors.Wrap(err, "failed to convert map of cached candidates to candidate list")
	}
	sort.Sort(candidates)
	candidatesBytes, err := Serialize(candidates)
	if err != nil {
		return hash.ZeroHash32B, errors.Wrap(err, "failed to serialize candidates")
	}
	h := byteutil.Uint64ToBytes(blockHeight)
	if err := ws.dao.Put(trie.CandidateKVNameSpace, h, candidatesBytes); err != nil {
		return hash.ZeroHash32B, errors.Wrapf(err, "failed to store candidates on height %d", blockHeight)
	}
	// Persist current chain height
	if err := ws.dao.Put(trie.AccountKVNameSpace, []byte(CurrentHeightKey), h); err != nil {
		return hash.ZeroHash32B, errors.Wrap(err, "failed to store accountTrie's current height")
	}
	return ws.rootHash(), nil
}

// Commit persists all changes in RunActions() into the DB
func (ws *workingset) commit() error {
	// commit all changes in a batch
	if err := ws.accountTrie.Commit(); err != nil {
		return errors.Wrap(err, "failed to commit all changes to underlying DB in a batch")
	}
	ws.clearCache()
	return nil
}

//======================================
// Contract functions
//======================================
// GetCodeHash returns contract's code hash
func (ws *workingset) GetCodeHash(addr hash.PKHash) (hash.Hash32B, error) {
	if contract, ok := ws.cachedContract[addr]; ok {
		return byteutil.BytesTo32B(contract.SelfState().CodeHash), nil
	}
	state, err := ws.cachedState(addr)
	if err != nil {
		return hash.ZeroHash32B, errors.Wrapf(err, "failed to GetCodeHash for contract %x", addr)
	}
	return byteutil.BytesTo32B(state.CodeHash), nil
}

// GetCode returns contract's code
func (ws *workingset) GetCode(addr hash.PKHash) ([]byte, error) {
	if contract, ok := ws.cachedContract[addr]; ok {
		return contract.GetCode()
	}
	state, err := ws.cachedState(addr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to GetCode for contract %x", addr)
	}
	return ws.dao.Get(trie.CodeKVNameSpace, state.CodeHash[:])
}

// SetCode sets contract's code
func (ws *workingset) SetCode(addr hash.PKHash, code []byte) error {
	if contract, ok := ws.cachedContract[addr]; ok {
		contract.SetCode(byteutil.BytesTo32B(hash.Hash256b(code)), code)
		return nil
	}
	contract, err := ws.getContract(addr)
	if err != nil {
		return errors.Wrapf(err, "failed to SetCode for contract %x", addr)
	}
	contract.SetCode(byteutil.BytesTo32B(hash.Hash256b(code)), code)
	return nil
}

// GetContractState returns contract's storage value
func (ws *workingset) GetContractState(addr hash.PKHash, key hash.Hash32B) (hash.Hash32B, error) {
	if contract, ok := ws.cachedContract[addr]; ok {
		v, err := contract.GetState(key)
		return byteutil.BytesTo32B(v), err
	}
	contract, err := ws.getContract(addr)
	if err != nil {
		return hash.ZeroHash32B, errors.Wrapf(err, "failed to GetContractState for contract %x", addr)
	}
	v, err := contract.GetState(key)
	return byteutil.BytesTo32B(v), err
}

// SetContractState writes contract's storage value
func (ws *workingset) SetContractState(addr hash.PKHash, key, value hash.Hash32B) error {
	if contract, ok := ws.cachedContract[addr]; ok {
		return contract.SetState(key, value[:])
	}
	contract, err := ws.getContract(addr)
	if err != nil {
		return errors.Wrapf(err, "failed to SetContractState for contract %x", addr)
	}
	return contract.SetState(key, value[:])
}

//======================================
// private state/account functions
//======================================
// getState pulls a State from DB
func (ws *workingset) getState(hash hash.PKHash) (*State, error) {
	mstate, err := ws.accountTrie.Get(hash[:])
	if errors.Cause(err) == trie.ErrNotExist {
		return nil, errors.Wrapf(ErrAccountNotExist, "addrHash = %x", hash[:])
	}
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get state of %x", hash)
	}
	return bytesToState(mstate)
}

func (ws *workingset) cachedState(hash hash.PKHash) (*State, error) {
	if state, ok := ws.cachedAccount[hash]; ok {
		return state, nil
	}
	// add to local cache
	state, err := ws.getState(hash)
	if state != nil {
		ws.cachedAccount[hash] = state
	}
	return state, err
}

// getState stores a State to DB
func (ws *workingset) putState(addr []byte, state *State) error {
	ss, err := stateToBytes(state)
	if err != nil {
		return errors.Wrapf(err, "failed to convert state %v to bytes", state)
	}
	return ws.accountTrie.Upsert(addr, ss)
}

func (ws *workingset) saveState(addr string, state *State) {
	if _, ok := ws.savedAccount[addr]; !ok {
		ws.savedAccount[addr] = state.clone()
	}
}

func (ws *workingset) getContract(addr hash.PKHash) (Contract, error) {
	state, err := ws.cachedState(addr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get the cached state of %x", addr)
	}
	delete(ws.cachedAccount, addr)
	if state.Root == hash.ZeroHash32B {
		state.Root = trie.EmptyRoot
	}
	tr, err := trie.NewTrieSharedDB(ws.dao, trie.ContractKVNameSpace, state.Root)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create storage trie for new contract %x", addr)
	}
	// add to contract cache
	contract := newContract(state, tr)
	ws.cachedContract[addr] = contract
	return contract, nil
}

// clearCache removes all local changes after committing to trie
func (ws *workingset) clearCache() {
	ws.savedAccount = nil
	ws.cachedAccount = nil
	ws.cachedContract = nil
	ws.savedAccount = make(map[string]*State)
	ws.cachedAccount = make(map[hash.PKHash]*State)
	ws.cachedContract = make(map[hash.PKHash]Contract)
}

//======================================
// private candidate functions
//======================================
func (ws *workingset) updateCandidate(pkHash hash.PKHash, totalWeight *big.Int, blockHeight uint64) {
	// Candidate was added when self-nomination, always exist in cachedCandidates
	candidate := ws.cachedCandidates[pkHash]
	candidate.Votes = totalWeight
	candidate.LastUpdateHeight = blockHeight
}

func (ws *workingset) getCandidates(height uint64) (CandidateList, error) {
	candidatesBytes, err := ws.dao.Get(trie.CandidateKVNameSpace, byteutil.Uint64ToBytes(height))
	if err != nil {
		return []*Candidate{}, errors.Wrapf(err, "failed to get candidates on height %d", height)
	}
	return Deserialize(candidatesBytes)
}

//======================================
// private transfer/vote functions
//======================================
func (ws *workingset) handleTsf(tsf []*action.Transfer) error {
	for _, tx := range tsf {
		if tx.IsContract() {
			continue
		}
		if !tx.IsCoinbase() {
			// check sender
			sender, err := ws.LoadOrCreateState(tx.Sender(), 0)
			if err != nil {
				return errors.Wrapf(err, "failed to load or create the state of sender %s", tx.Sender())
			}
			// save state before modifying
			ws.saveState(tx.Sender(), sender)
			if tx.Amount().Cmp(sender.Balance) == 1 {
				return errors.Wrapf(ErrNotEnoughBalance, "failed to verify the balance of sender %s", tx.Sender())
			}
			// update sender balance
			if err := sender.SubBalance(tx.Amount()); err != nil {
				return errors.Wrapf(err, "failed to update the balance of sender %s", tx.Sender())
			}
			// update sender Nonce
			if tx.Nonce() > sender.Nonce {
				sender.Nonce = tx.Nonce()
			}
			// Update sender votes
			if len(sender.Votee) > 0 && sender.Votee != tx.Sender() {
				// sender already voted to a different person
				voteeOfSender, err := ws.LoadOrCreateState(sender.Votee, 0)
				if err != nil {
					return errors.Wrapf(err, "failed to load or create the state of sender's votee %s", sender.Votee)
				}
				// save state before modifying
				ws.saveState(sender.Votee, voteeOfSender)
				voteeOfSender.VotingWeight.Sub(voteeOfSender.VotingWeight, tx.Amount())
			}
		}
		// check recipient
		recipient, err := ws.LoadOrCreateState(tx.Recipient(), 0)
		if err != nil {
			return errors.Wrapf(err, "failed to laod or create the state of recipient %s", tx.Recipient())
		}
		// save state before modifying
		ws.saveState(tx.Recipient(), recipient)
		// update recipient balance
		if err := recipient.AddBalance(tx.Amount()); err != nil {
			return errors.Wrapf(err, "failed to update the balance of recipient %s", tx.Recipient())
		}
		// Update recipient votes
		if len(recipient.Votee) > 0 && recipient.Votee != tx.Recipient() {
			// recipient already voted to a different person
			voteeOfRecipient, err := ws.LoadOrCreateState(recipient.Votee, 0)
			if err != nil {
				return errors.Wrapf(err, "failed to load or create the state of recipient's votee %s", recipient.Votee)
			}
			// save state before modifying
			ws.saveState(recipient.Votee, voteeOfRecipient)
			voteeOfRecipient.VotingWeight.Add(voteeOfRecipient.VotingWeight, tx.Amount())
		}
	}
	return nil
}

func (ws *workingset) handleVote(blockHeight uint64, vote []*action.Vote) error {
	for _, v := range vote {
		voteFrom, err := ws.LoadOrCreateState(v.Voter(), 0)
		if err != nil {
			return errors.Wrapf(err, "failed to load or create the state of voter %s", v.Voter())
		}
		// save state before modifying
		ws.saveState(v.Voter(), voteFrom)
		// update voteFrom Nonce
		if v.Nonce() > voteFrom.Nonce {
			voteFrom.Nonce = v.Nonce()
		}
		// Update old votee's weight
		if len(voteFrom.Votee) > 0 && voteFrom.Votee != v.Voter() {
			// voter already voted
			oldVotee, err := ws.LoadOrCreateState(voteFrom.Votee, 0)
			if err != nil {
				return errors.Wrapf(err, "failed to load or create the state of voter's old votee %s", voteFrom.Votee)
			}
			// save state before modifying
			ws.saveState(voteFrom.Votee, oldVotee)
			oldVotee.VotingWeight.Sub(oldVotee.VotingWeight, voteFrom.Balance)
			voteFrom.Votee = ""
		}

		if v.Votee() == "" {
			// unvote operation
			voteFrom.IsCandidate = false
			continue
		}

		voteTo, err := ws.LoadOrCreateState(v.Votee(), 0)
		if err != nil {
			return errors.Wrapf(err, "failed to load or create the state of votee %s", v.Votee())
		}
		// save state before modifying
		ws.saveState(v.Votee(), voteTo)
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
			if _, ok := ws.cachedCandidates[pkHashAddress]; !ok {
				ws.cachedCandidates[pkHashAddress] = &Candidate{
					Address:        v.Voter(),
					PublicKey:      votePubkey,
					CreationHeight: blockHeight,
				}
			}
		}
	}
	return nil
}
