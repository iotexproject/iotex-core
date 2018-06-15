// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package state

import (
	"container/heap"
	"fmt"
	"math/big"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/blockchain/action"
	"github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/trie"
)

const (
	// Level 1 is for candidate pool
	candidatePool = 1
	// Level 2 is for candidate buffer pool
	candidateBufferPool = candidatePool + 1

	// TODO: make this configurable
	//	delegateSize  = 101
	//	candidateSize = 400
	//	bufferSize    = 10000
)

// TODO: this is only for test config, use 101, 400, 10000 when in production
const (
	candidateSize       = 2
	candidateBufferSize = 10
)

var (
	// ErrInvalidAddr is the error that the address format is invalid, cannot be decoded
	ErrInvalidAddr = errors.New("address format is invalid")

	// ErrNotEnoughBalance is the error that the balance is not enough
	ErrNotEnoughBalance = errors.New("not enough balance")

	// ErrAccountNotExist is the error that the account does not exist
	ErrAccountNotExist = errors.New("the account does not exist")

	// ErrFailedToMarshalState is the error that the state marshaling is failed
	ErrFailedToMarshalState = errors.New("failed to marshal state")

	// ErrFailedToUnmarshalState is the error that the state un-marshaling is failed
	ErrFailedToUnmarshalState = errors.New("failed to unmarshal state")
)

type (
	// Factory defines an interface for managing states
	Factory interface {
		CreateState(string, uint64) (*State, error)
		Balance(string) (*big.Int, error)
		CommitStateChanges(uint64, []*action.Transfer, []*action.Vote) error
		Nonce(string) (uint64, error)
		RootHash() common.Hash32B
		Candidates() (uint64, []*Candidate)
	}

	// factory implements StateFactory interface, tracks changes in a map and batch-commits to trie/db
	factory struct {
		currentChainHeight     uint64
		trie                   trie.Trie
		candidateHeap          CandidateMinPQ
		candidateBufferMinHeap CandidateMinPQ
		candidateBufferMaxHeap CandidateMaxPQ
	}
)

// NewFactory creates a new state factory
func NewFactory(tr trie.Trie) Factory {
	return &factory{
		currentChainHeight:     0,
		trie:                   tr,
		candidateHeap:          CandidateMinPQ{candidateSize, make([]*Candidate, 0)},
		candidateBufferMinHeap: CandidateMinPQ{candidateBufferSize, make([]*Candidate, 0)},
		candidateBufferMaxHeap: CandidateMaxPQ{candidateBufferSize, make([]*Candidate, 0)},
	}
}

// NewFactoryFromTrieDBPath creates a new stateFactory from give trie db path.
func NewFactoryFromTrieDBPath(dbPath string) (Factory, error) {
	if len(dbPath) == 0 {
		// TODO not return error here is a hack
		return nil, nil
	}
	tr, err := trie.NewTrie(dbPath, false)
	if err != nil {
		return nil, err
	}
	return NewFactory(tr), nil
}

// CreateState adds a new State with initial balance to the factory
func (sf *factory) CreateState(addr string, init uint64) (*State, error) {
	pubKeyHash := iotxaddress.GetPubkeyHash(addr)
	if pubKeyHash == nil {
		return nil, ErrInvalidAddr
	}
	balance := big.NewInt(0)
	weight := big.NewInt(0)
	balance.SetUint64(init)
	s := State{Address: addr, Balance: balance, VotingWeight: weight}
	mstate, err := stateToBytes(&s)
	if err != nil {
		return nil, err
	}
	if err := sf.trie.Upsert(pubKeyHash, mstate); err != nil {
		return nil, err
	}
	return &s, nil
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

// RootHash returns the hash of the root node of the trie
func (sf *factory) RootHash() common.Hash32B {
	return sf.trie.RootHash()
}

// CommitStateChanges updates a State from the given actions
func (sf *factory) CommitStateChanges(chainHeight uint64, tsf []*action.Transfer, vote []*action.Vote) error {
	sf.currentChainHeight = chainHeight
	pending := make(map[common.PKHash]*State)
	addressToPKMap := make(map[string][]byte)

	if err := sf.handleTsf(pending, addressToPKMap, tsf); err != nil {
		return err
	}
	if err := sf.handleVote(pending, addressToPKMap, vote); err != nil {
		return err
	}

	// construct <k, v> list of pending state
	transferK := [][]byte{}
	transferV := [][]byte{}
	for pkhash, state := range pending {
		ss, err := stateToBytes(state)
		if err != nil {
			return err
		}
		addr := make([]byte, len(pkhash))
		copy(addr, pkhash[:])
		transferK = append(transferK, addr)
		transferV = append(transferV, ss)

		// Perform vote update operation on candidate and delegate pools
		if !state.IsCandidate {
			continue
		}
		totalWeight := big.NewInt(0)
		totalWeight.Add(totalWeight, state.VotingWeight)
		if state.Votee == state.Address {
			totalWeight.Add(totalWeight, state.Balance)
		}
		if c, level := sf.inPool(state.Address); level > 0 {
			sf.updateVotes(c, totalWeight)
			continue
		}
		if pubKey, ok := addressToPKMap[state.Address]; ok {
			candidate := &Candidate{
				Address:  state.Address,
				Votes:    totalWeight,
				PubKey:   pubKey,
				minIndex: 0,
				maxIndex: 0,
			}
			sf.updateVotes(candidate, totalWeight)
		}
		// If the candidate who needs vote update but not in the pool
		// and is not involved in a vote activity, then don't considert him
	}
	// commit the state changes to Trie in a batch
	return sf.trie.Commit(transferK, transferV)
}

// Candidates returns array of candidates in candidate pool
func (sf *factory) Candidates() (uint64, []*Candidate) {
	return sf.currentChainHeight, sf.candidateHeap.CandidateList()
}

//======================================
// private functions
//=====================================
func (sf *factory) candidatesBuffer() (uint64, []*Candidate) {
	return sf.currentChainHeight, sf.candidateBufferMinHeap.CandidateList()
}

// getState pulls an existing State
func (sf *factory) getState(addr string) (*State, error) {
	pubKeyHash := iotxaddress.GetPubkeyHash(addr)
	return sf.getStateFromPKHash(pubKeyHash)
}

func (sf *factory) getStateFromPKHash(pubKeyHash []byte) (*State, error) {
	if pubKeyHash == nil {
		return nil, ErrInvalidAddr
	}
	mstate, err := sf.trie.Get(pubKeyHash)
	if errors.Cause(err) == trie.ErrNotExist {
		return nil, ErrAccountNotExist
	}
	if err != nil {
		return nil, err
	}
	return bytesToState(mstate)
}

func (sf *factory) updateVotes(candidate *Candidate, votes *big.Int) {
	candidate.Votes = votes
	c, level := sf.inPool(candidate.Address)
	switch level {
	case candidatePool:
		// if candidate is already in candidate pool
		sf.candidateHeap.update(c, candidate.Votes)
	case candidateBufferPool:
		// if candidate is already in candidate buffer pool
		sf.candidateBufferMinHeap.update(c, candidate.Votes)
		sf.candidateBufferMaxHeap.update(c, candidate.Votes)
	default:
		// candidate is not in any of two pools
		transitCandidate := candidate
		if sf.candidateHeap.shouldTake(transitCandidate.Votes) {
			// Push candidate into candidate pool
			heap.Push(&sf.candidateHeap, transitCandidate)
			transitCandidate = nil
			if sf.candidateHeap.Len() > sf.candidateHeap.Capacity {
				transitCandidate = heap.Pop(&sf.candidateHeap).(*Candidate)
			}
		}
		if transitCandidate != nil && sf.candidateBufferMinHeap.shouldTake(transitCandidate.Votes) {
			// Push candidate into candidate pool
			heap.Push(&sf.candidateBufferMinHeap, transitCandidate)
			heap.Push(&sf.candidateBufferMaxHeap, transitCandidate)
			transitCandidate = nil
			if sf.candidateBufferMinHeap.Len() > sf.candidateBufferMinHeap.Capacity {
				transitCandidate = heap.Pop(&sf.candidateBufferMinHeap).(*Candidate)
				heap.Remove(&sf.candidateBufferMaxHeap, transitCandidate.maxIndex)
			}
		}
	}
	sf.balance()

	// Temporarily leave it here to check the algorithm is correct
	if sf.candidateBufferMinHeap.Len() != sf.candidateBufferMaxHeap.Len() {
		fmt.Println("***************heap not sync***************")
	}
}

func (sf *factory) balance() {
	if sf.candidateHeap.Len() > 0 && sf.candidateBufferMaxHeap.Len() > 0 && sf.candidateHeap.Top().(*Candidate).Votes.Cmp(sf.candidateBufferMaxHeap.Top().(*Candidate).Votes) < 0 {
		cFromCandidatePool := heap.Pop(&sf.candidateHeap).(*Candidate)
		cFromCandidateBufferPool := heap.Pop(&sf.candidateBufferMaxHeap).(*Candidate)
		heap.Remove(&sf.candidateBufferMinHeap, cFromCandidateBufferPool.minIndex)
		heap.Push(&sf.candidateHeap, cFromCandidateBufferPool)
		heap.Push(&sf.candidateBufferMinHeap, cFromCandidatePool)
		heap.Push(&sf.candidateBufferMaxHeap, cFromCandidatePool)
	}
}

func (sf *factory) inPool(address string) (*Candidate, int) {
	if c := sf.candidateHeap.exist(address); c != nil {
		return c, candidatePool // The candidate exists in the Candidate pool
	}
	if c := sf.candidateBufferMinHeap.exist(address); c != nil {
		return c, candidateBufferPool // The candidate exists in the Candidate buffer pool
	}
	return nil, 0
}

func (sf *factory) upsert(pending map[common.PKHash]*State, address string) (*State, error) {
	pkhash := iotxaddress.GetPubkeyHash(address)
	if pkhash == nil {
		return nil, ErrInvalidAddr
	}
	var tempPubKeyHash common.PKHash
	var err error
	copy(tempPubKeyHash[:], pkhash)
	state, exist := pending[tempPubKeyHash]
	if !exist {
		state, err = sf.getState(address)
		switch {
		case err == ErrAccountNotExist:
			if state, err = sf.CreateState(address, 0); err != nil {
				return nil, err
			}
		case err != nil:
			return nil, err
		}
		pending[tempPubKeyHash] = state
	}
	return state, nil
}

func (sf *factory) handleTsf(pending map[common.PKHash]*State, addressToPKMap map[string][]byte, tsf []*action.Transfer) error {
	for _, tx := range tsf {
		if !tx.IsCoinbase {
			// check sender
			sender, err := sf.upsert(pending, tx.Sender)
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
			if len(sender.Votee) > 0 && sender.Votee != sender.Address {
				// sender already voted to a different person
				voteeOfSender, err := sf.upsert(pending, sender.Votee)
				if err != nil {
					return err
				}
				voteeOfSender.VotingWeight.Sub(voteeOfSender.VotingWeight, tx.Amount)
			}
		}
		// check recipient
		recipient, err := sf.upsert(pending, tx.Recipient)
		if err != nil {
			return err
		}
		// update recipient balance
		if err := recipient.AddBalance(tx.Amount); err != nil {
			return err
		}
		// Update recipient votes
		if len(recipient.Votee) > 0 && recipient.Votee != recipient.Address {
			// recipient already voted to a different person
			voteeOfRecipient, err := sf.upsert(pending, recipient.Votee)
			if err != nil {
				return err
			}
			voteeOfRecipient.VotingWeight.Add(voteeOfRecipient.VotingWeight, tx.Amount)
		}
	}
	return nil
}

func (sf *factory) handleVote(pending map[common.PKHash]*State, addressToPKMap map[string][]byte, vote []*action.Vote) error {
	for _, v := range vote {
		selfAddress, err := iotxaddress.GetAddress(v.SelfPubkey, iotxaddress.IsTestnet, iotxaddress.ChainID)
		if err != nil {
			return err
		}
		voteFrom, err := sf.upsert(pending, selfAddress.RawAddress)
		if err != nil {
			return err
		}
		addressToPKMap[voteFrom.Address] = v.SelfPubkey

		voteAddress, err := iotxaddress.GetAddress(v.VotePubkey, iotxaddress.IsTestnet, iotxaddress.ChainID)
		if err != nil {
			return err
		}

		// update voteFrom nonce
		if v.Nonce > voteFrom.Nonce {
			voteFrom.Nonce = v.Nonce
		}
		voteTo, err := sf.upsert(pending, voteAddress.RawAddress)
		if err != nil {
			return err
		}
		addressToPKMap[voteTo.Address] = v.VotePubkey

		// Update old votee's weight
		if len(voteFrom.Votee) > 0 && voteFrom.Votee != voteFrom.Address {
			// voter already voted
			oldVotee, err := sf.upsert(pending, voteFrom.Votee)
			if err != nil {
				return err
			}
			oldVotee.VotingWeight.Sub(oldVotee.VotingWeight, voteFrom.Balance)
			voteFrom.Votee = ""
		}

		if voteFrom.Address != voteTo.Address {
			// Voter votes to a different person
			voteTo.VotingWeight.Add(voteTo.VotingWeight, voteFrom.Balance)
			voteFrom.Votee = voteTo.Address
		} else {
			voteFrom.Votee = voteFrom.Address
			voteFrom.IsCandidate = true
		}
	}
	return nil
}
