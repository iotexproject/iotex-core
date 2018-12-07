// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"
	"fmt"
	"math/big"
	"sort"

	"github.com/CoderZhi/go-ethereum/core/vm"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/db/trie"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/state"
)

type (
	// WorkingSet defines an interface for working set of states changes
	WorkingSet interface {
		// states and actions
		LoadOrCreateAccountState(string, *big.Int) (*state.Account, error)
		Nonce(string) (uint64, error) // Note that Nonce starts with 1.
		CachedAccountState(string) (*state.Account, error)
		RunActions(context.Context, uint64, []action.Action) (hash.Hash32B, map[hash.Hash32B]*action.Receipt, error)
		Snapshot() int
		Revert(int) error
		Commit() error
		// Accounts
		RootHash() hash.Hash32B
		Version() uint64
		Height() uint64
		// General state
		State(hash.PKHash, interface{}) error
		PutState(hash.PKHash, interface{}) error
		CachedState(hash.PKHash, state.State) (state.State, error)
		GetDB() db.KVStore
		GetCachedBatch() db.CachedBatch
	}

	// workingSet implements WorkingSet interface, tracks pending changes to account/contract in local cache
	workingSet struct {
		ver              uint64
		blkHeight        uint64
		cachedCandidates map[hash.PKHash]*state.Candidate
		cachedStates     map[hash.PKHash]state.State // states being modified in this block
		accountTrie      trie.Trie                   // global account state trie
		trieRoots        map[int]hash.Hash32B        // root of trie at time of snapshot
		cb               db.CachedBatch              // cached batch for pending writes
		dao              db.KVStore                  // the underlying DB for account/contract storage
		actionHandlers   []protocol.ActionHandler
	}
)

// NewWorkingSet creates a new working set
func NewWorkingSet(
	version uint64,
	kv db.KVStore,
	root hash.Hash32B,
	actionHandlers []protocol.ActionHandler,
) (WorkingSet, error) {
	ws := &workingSet{
		ver:              version,
		cachedCandidates: make(map[hash.PKHash]*state.Candidate),
		cachedStates:     make(map[hash.PKHash]state.State),
		trieRoots:        make(map[int]hash.Hash32B),
		cb:               db.NewCachedBatch(),
		dao:              kv,
		actionHandlers:   actionHandlers,
	}
	dbForTrie, err := db.NewKVStoreForTrie(AccountKVNameSpace, ws.dao, db.CachedBatchOption(ws.cb))
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate state tire db")
	}
	tr, err := trie.NewTrie(trie.KVStoreOption(dbForTrie), trie.RootHashOption(root[:]))
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate state trie from config")
	}
	ws.accountTrie = tr
	if err := ws.accountTrie.Start(context.Background()); err != nil {
		return nil, errors.Wrapf(err, "failed to load state trie from root = %x", root)
	}
	return ws, nil
}

//======================================
// account functions
//======================================
// LoadOrCreateAccountState loads existing or adds a new account state with initial balance to the factory
// addr should be a bech32 properly-encoded string
func (ws *workingSet) LoadOrCreateAccountState(addr string, init *big.Int) (*state.Account, error) {
	account, err := ws.CachedAccountState(addr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get account of %s from cached account", addr)
	}
	if account == state.EmptyAccount {
		account = &state.Account{
			Balance:      init,
			VotingWeight: big.NewInt(0),
		}
		addrHash, _ := iotxaddress.AddressToPKHash(addr)
		ws.cachedStates[addrHash] = account
		return account, nil
	}
	return account, nil
}

// Nonce returns the Nonce if the account exists
func (ws *workingSet) Nonce(addr string) (uint64, error) {
	s, err := ws.accountState(addr)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to get account state of %s", addr)
	}
	return s.Nonce, nil
}

// CachedAccountState returns the cached account state if the address exists in local cache
func (ws *workingSet) CachedAccountState(addr string) (*state.Account, error) {
	addrHash, err := iotxaddress.AddressToPKHash(addr)
	if err != nil {
		return nil, err
	}
	s, err := ws.CachedState(addrHash, &state.Account{})
	if err != nil {
		if errors.Cause(err) == state.ErrStateNotExist {
			return state.EmptyAccount, nil
		}
		return nil, err
	}
	account, err := stateToAccountState(s)
	if err != nil {
		return nil, err
	}
	return account, nil
}

// RootHash returns the hash of the root node of the accountTrie
func (ws *workingSet) RootHash() hash.Hash32B {
	return byteutil.BytesTo32B(ws.accountTrie.RootHash())
}

// Version returns the Version of this working set
func (ws *workingSet) Version() uint64 {
	return ws.ver
}

// Height returns the Height of the block being worked on
func (ws *workingSet) Height() uint64 {
	return ws.blkHeight
}

// RunActions runs actions in the block and track pending changes in working set
func (ws *workingSet) RunActions(
	ctx context.Context,
	blockHeight uint64,
	actions []action.Action,
) (hash.Hash32B, map[hash.Hash32B]*action.Receipt, error) {
	ws.blkHeight = blockHeight
	// Recover cachedCandidates after restart factory
	if blockHeight > 0 && len(ws.cachedCandidates) == 0 {
		candidates, err := ws.getCandidates(blockHeight - 1)
		if err != nil {
			logger.Info().Err(err).Msgf("No previous Candidates on Height %d", blockHeight-1)
			candidates = state.CandidateList{}
		}
		if ws.cachedCandidates, err = state.CandidatesToMap(candidates); err != nil {
			return hash.ZeroHash32B, nil,
				errors.Wrap(err, "failed to convert candidate list to map of cached Candidates")
		}
	}

	tsfs, votes, _ := action.ClassifyActions(actions)
	raCtx, ok := state.GetRunActionsCtx(ctx)
	if !ok {
		return hash.ZeroHash32B, nil,
			errors.New("failed to get RunActionsCtx")
	}
	// check producer
	producer, err := ws.LoadOrCreateAccountState(raCtx.ProducerAddr, big.NewInt(0))
	if err != nil {
		return hash.ZeroHash32B, nil, errors.Wrapf(err, "failed to load or create the account of block producer %s", raCtx.ProducerAddr)
	}
	if err := ws.handleTsf(producer, tsfs, raCtx.GasLimit, raCtx.EnableGasCharge); err != nil {
		return hash.ZeroHash32B, nil, errors.Wrap(err, "failed to handle transfers")
	}
	if err := ws.handleVote(producer, blockHeight, votes, raCtx.GasLimit, raCtx.EnableGasCharge); err != nil {
		return hash.ZeroHash32B, nil, errors.Wrap(err, "failed to handle votes")
	}

	// update pending account changes to trie
	for addr, state := range ws.cachedStates {
		if err := ws.PutState(addr, state); err != nil {
			return hash.ZeroHash32B, nil, errors.Wrap(err, "failed to update pending account changes to trie")
		}
		account, err := stateToAccountState(state)
		if err != nil {
			return hash.ZeroHash32B, nil, err
		}
		// Perform vote update operation on candidate and delegate pools
		if !account.IsCandidate {
			// remove the candidate if the person is not a candidate anymore
			if _, ok := ws.cachedCandidates[addr]; ok {
				delete(ws.cachedCandidates, addr)
			}
			continue
		}
		totalWeight := big.NewInt(0)
		totalWeight.Add(totalWeight, account.VotingWeight)
		voteePKHash, err := iotxaddress.AddressToPKHash(account.Votee)
		if err != nil {
			return hash.ZeroHash32B, nil, err
		}
		if addr == voteePKHash {
			totalWeight.Add(totalWeight, account.Balance)
		}
		ws.updateCandidate(addr, totalWeight, blockHeight)
	}

	// Handle actions
	receipts := make(map[hash.Hash32B]*action.Receipt)
	for _, act := range actions {
		for _, actionHandler := range ws.actionHandlers {
			receipt, err := actionHandler.Handle(ctx, act, ws)
			if err != nil {
				return hash.ZeroHash32B, nil, errors.Wrapf(
					err,
					"error when action %x (nonce: %d) from %s mutates states",
					act.Hash(),
					act.Nonce(),
					act.SrcAddr(),
				)
			}
			if receipt != nil {
				receipts[act.Hash()] = receipt
			}
		}
	}

	// Persist accountTrie's root hash
	rootHash := ws.accountTrie.RootHash()
	ws.cb.Put(AccountKVNameSpace, []byte(AccountTrieRootKey), rootHash[:], "failed to store accountTrie's root hash")
	// Persist new list of Candidates
	candidates, err := state.MapToCandidates(ws.cachedCandidates)
	if err != nil {
		return hash.ZeroHash32B, nil, errors.Wrap(err, "failed to convert map of cached Candidates to candidate list")
	}
	sort.Sort(candidates)
	candidatesBytes, err := candidates.Serialize()
	if err != nil {
		return hash.ZeroHash32B, nil, errors.Wrap(err, "failed to serialize Candidates")
	}
	h := byteutil.Uint64ToBytes(blockHeight)
	ws.cb.Put(CandidateKVNameSpace, h, candidatesBytes, "failed to store Candidates on Height %d", blockHeight)
	// Persist current chain Height
	ws.cb.Put(AccountKVNameSpace, []byte(CurrentHeightKey), h, "failed to store accountTrie's current Height")

	return ws.RootHash(), receipts, nil
}

func (ws *workingSet) Snapshot() int {
	s := ws.cb.Snapshot()
	ws.trieRoots[s] = byteutil.BytesTo32B(ws.accountTrie.RootHash())
	return s
}

func (ws *workingSet) Revert(snapshot int) error {
	if err := ws.cb.Revert(snapshot); err != nil {
		return err
	}
	root, ok := ws.trieRoots[snapshot]
	if !ok {
		// this should not happen, b/c we save the trie root on a successful return of Snapshot(), but check anyway
		return errors.Wrapf(trie.ErrInvalidTrie, "failed to get trie root for snapshot = %d", snapshot)
	}
	return ws.accountTrie.SetRootHash(root[:])
}

// Commit persists all changes in RunActions() into the DB
func (ws *workingSet) Commit() error {
	// Commit all changes in a batch
	if err := ws.dao.Commit(ws.cb); err != nil {
		return errors.Wrap(err, "failed to Commit all changes to underlying DB in a batch")
	}
	ws.clearCache()
	return nil
}

// GetDB returns the underlying DB for account/contract storage
func (ws *workingSet) GetDB() db.KVStore {
	return ws.dao
}

// GetCachedBatch returns the cached batch for pending writes
func (ws *workingSet) GetCachedBatch() db.CachedBatch {
	return ws.cb
}

//======================================
// private account/contract functions
//======================================
// state pulls a state from DB
func (ws *workingSet) State(hash hash.PKHash, s interface{}) error {
	mstate, err := ws.accountTrie.Get(hash[:])
	if errors.Cause(err) == trie.ErrNotExist {
		return errors.Wrapf(state.ErrStateNotExist, "addrHash = %x", hash[:])
	}
	if err != nil {
		return errors.Wrapf(err, "failed to get account of %x", hash)
	}
	if err := state.Deserialize(s, mstate); err != nil {
		return err
	}
	return nil
}

// accountState returns the confirmed account state on the chain
func (ws *workingSet) accountState(addr string) (*state.Account, error) {
	addrHash, err := iotxaddress.AddressToPKHash(addr)
	if err != nil {
		return nil, err
	}
	var ac state.Account
	if err := ws.State(addrHash, &ac); err != nil {
		if errors.Cause(err) == state.ErrStateNotExist {
			return state.EmptyAccount, nil
		}
		return nil, err
	}
	return &ac, nil
}

// cachedState pulls a state from cache first. If missing, it will hit DB
func (ws *workingSet) CachedState(hash hash.PKHash, s state.State) (state.State, error) {
	if state, ok := ws.cachedStates[hash]; ok {
		return state, nil
	}
	// add to local cache
	if err := ws.State(hash, s); err != nil {
		return s, err
	}
	ws.cachedStates[hash] = s
	return s, nil
}

// putState put a state into DB
func (ws *workingSet) PutState(pkHash hash.PKHash, s interface{}) error {
	ss, err := state.Serialize(s)
	if err != nil {
		return errors.Wrapf(err, "failed to convert account %v to bytes", s)
	}
	return ws.accountTrie.Upsert(pkHash[:], ss)
}

// clearCache removes all local changes after committing to trie
func (ws *workingSet) clearCache() {
	ws.cachedStates = nil
	ws.cachedCandidates = nil
	ws.trieRoots = nil
	ws.cachedStates = make(map[hash.PKHash]state.State)
	ws.cachedCandidates = make(map[hash.PKHash]*state.Candidate)
	ws.trieRoots = make(map[int]hash.Hash32B)
}

//======================================
// private candidate functions
//======================================
func (ws *workingSet) updateCandidate(pkHash hash.PKHash, totalWeight *big.Int, blockHeight uint64) {
	// Candidate was added when self-nomination, always exist in cachedCandidates
	candidate := ws.cachedCandidates[pkHash]
	candidate.Votes = totalWeight
	candidate.LastUpdateHeight = blockHeight
}

func (ws *workingSet) getCandidates(height uint64) (state.CandidateList, error) {
	candidatesBytes, err := ws.dao.Get(CandidateKVNameSpace, byteutil.Uint64ToBytes(height))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get Candidates on Height %d", height)
	}
	var candidates state.CandidateList
	if err := candidates.Deserialize(candidatesBytes); err != nil {
		return nil, err
	}
	return candidates, nil
}

//======================================
// private transfer/vote functions
//======================================
func (ws *workingSet) handleTsf(producer *state.Account, tsfs []*action.Transfer, gasLimit *uint64, enableGasCharge bool) error {
	for _, tx := range tsfs {
		if tx.IsContract() {
			continue
		}
		if !tx.IsCoinbase() {
			// check sender
			sender, err := ws.LoadOrCreateAccountState(tx.Sender(), big.NewInt(0))
			if err != nil {
				return errors.Wrapf(err, "failed to load or create the account of sender %s", tx.Sender())
			}

			if enableGasCharge {
				gas, err := tx.IntrinsicGas()
				if err != nil {
					return errors.Wrapf(err, "failed to get intrinsic gas for transfer hash %s", tx.Hash())
				}
				if *gasLimit < gas {
					return vm.ErrOutOfGas
				}

				gasFee := big.NewInt(0).Mul(tx.GasPrice(), big.NewInt(0).SetUint64(gas))
				if big.NewInt(0).Add(tx.Amount(), gasFee).Cmp(sender.Balance) == 1 {
					return errors.Wrapf(state.ErrNotEnoughBalance, "failed to verify the Balance of sender %s", tx.Sender())
				}

				// charge sender gas
				if err := sender.SubBalance(gasFee); err != nil {
					return errors.Wrapf(err, "failed to charge the gas for sender %s", tx.Sender())
				}
				// compensate block producer gas
				if err := producer.AddBalance(gasFee); err != nil {
					return errors.Wrapf(err, "failed to compensate gas to producer")
				}
				*gasLimit -= gas
			}
			// update sender Balance
			if err := sender.SubBalance(tx.Amount()); err != nil {
				return errors.Wrapf(err, "failed to update the Balance of sender %s", tx.Sender())
			}
			// update sender Nonce
			if tx.Nonce() > sender.Nonce {
				sender.Nonce = tx.Nonce()
			}
			// Update sender votes
			if len(sender.Votee) > 0 && sender.Votee != tx.Sender() {
				// sender already voted to a different person
				voteeOfSender, err := ws.LoadOrCreateAccountState(sender.Votee, big.NewInt(0))
				if err != nil {
					return errors.Wrapf(err, "failed to load or create the account of sender's votee %s", sender.Votee)
				}
				voteeOfSender.VotingWeight.Sub(voteeOfSender.VotingWeight, tx.Amount())
			}
		}
		// check recipient
		recipient, err := ws.LoadOrCreateAccountState(tx.Recipient(), big.NewInt(0))
		if err != nil {
			return errors.Wrapf(err, "failed to laod or create the account of recipient %s", tx.Recipient())
		}
		if err := recipient.AddBalance(tx.Amount()); err != nil {
			return errors.Wrapf(err, "failed to update the Balance of recipient %s", tx.Recipient())
		}
		// Update recipient votes
		if len(recipient.Votee) > 0 && recipient.Votee != tx.Recipient() {
			// recipient already voted to a different person
			voteeOfRecipient, err := ws.LoadOrCreateAccountState(recipient.Votee, big.NewInt(0))
			if err != nil {
				return errors.Wrapf(err, "failed to load or create the account of recipient's votee %s", recipient.Votee)
			}
			voteeOfRecipient.VotingWeight.Add(voteeOfRecipient.VotingWeight, tx.Amount())
		}
	}
	return nil
}

func (ws *workingSet) handleVote(producer *state.Account, blockHeight uint64, votes []*action.Vote, gasLimit *uint64, enableGasCharge bool) error {
	for _, v := range votes {
		voteFrom, err := ws.LoadOrCreateAccountState(v.Voter(), big.NewInt(0))
		if err != nil {
			return errors.Wrapf(err, "failed to load or create the account of voter %s", v.Voter())
		}
		voterPKHash, err := iotxaddress.AddressToPKHash(v.Voter())
		if err != nil {
			return err
		}

		if enableGasCharge {
			gas, err := v.IntrinsicGas()
			if err != nil {
				return errors.Wrapf(err, "failed to get intrinsic gas for vote hash %s", v.Hash())
			}
			if *gasLimit < gas {
				return vm.ErrOutOfGas
			}
			gasFee := big.NewInt(0).Mul(v.GasPrice(), big.NewInt(0).SetUint64(gas))

			if gasFee.Cmp(voteFrom.Balance) == 1 {
				return errors.Wrapf(state.ErrNotEnoughBalance, "failed to verify the Balance for gas of voter %s, %d, %d", v.Voter(), gas, voteFrom.Balance)
			}

			// charge voter Gas
			if err := voteFrom.SubBalance(gasFee); err != nil {
				return errors.Wrapf(err, "failed to charge the gas for voter %s", v.Voter())
			}
			// compensate block producer gas
			if err := producer.AddBalance(gasFee); err != nil {
				return errors.Wrapf(err, "failed to compensate gas to producer")
			}
			*gasLimit -= gas
		}
		// update voteFrom Nonce
		if v.Nonce() > voteFrom.Nonce {
			voteFrom.Nonce = v.Nonce()
		}
		// Update old votee's weight
		if len(voteFrom.Votee) > 0 && voteFrom.Votee != v.Voter() {
			// voter already voted
			oldVotee, err := ws.LoadOrCreateAccountState(voteFrom.Votee, big.NewInt(0))
			if err != nil {
				return errors.Wrapf(err, "failed to load or create the account of voter's old votee %s", voteFrom.Votee)
			}
			oldVotee.VotingWeight.Sub(oldVotee.VotingWeight, voteFrom.Balance)
			voteFrom.Votee = ""
		}

		if v.Votee() == "" {
			// unvote operation
			voteFrom.IsCandidate = false
			continue
		}

		voteTo, err := ws.LoadOrCreateAccountState(v.Votee(), big.NewInt(0))
		if err != nil {
			return errors.Wrapf(err, "failed to load or create the account of votee %s", v.Votee())
		}
		if v.Voter() != v.Votee() {
			// Voter votes to a different person
			voteTo.VotingWeight.Add(voteTo.VotingWeight, voteFrom.Balance)
			voteFrom.Votee = v.Votee()
		} else {
			// Vote to self: self-nomination or cancel the previous vote case
			voteFrom.Votee = v.Voter()
			voteFrom.IsCandidate = true
			votePubkey := v.VoterPublicKey()
			if _, ok := ws.cachedCandidates[voterPKHash]; !ok {
				ws.cachedCandidates[voterPKHash] = &state.Candidate{
					Address:        v.Voter(),
					PublicKey:      votePubkey,
					CreationHeight: blockHeight,
				}
			}
		}
	}
	return nil
}

func stateToAccountState(s state.State) (*state.Account, error) {
	account, ok := s.(*state.Account)
	if !ok {
		return nil, fmt.Errorf("error when casting %T state into account state", s)
	}
	return account, nil
}
