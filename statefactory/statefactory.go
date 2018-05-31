// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package statefactory

import (
	"bytes"
	"encoding/gob"
	"math/big"

	"github.com/pkg/errors"

	trx "github.com/iotexproject/iotex-core/blockchain/trx"
	"github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/trie"
)

var (
	stateFactoryKVNameSpace = "StateFactory"

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
	// State is the canonical representation of an account.
	State struct {
		Nonce        uint64
		Balance      *big.Int
		Address      string
		IsCandidate  bool
		VotingWeight *big.Int
		Votee        common.PKHash
		Voters       map[common.PKHash]*big.Int
	}

	// StateFactory defines an interface for managing states
	StateFactory interface {
		CreateState(string, uint64) (*State, error)
		Balance(string) (*big.Int, error)
		CommitStateChanges([]*trx.Transfer, []*trx.Vote) error
		SetNonce(string, uint64) error
		Nonce(string) (uint64, error)
		RootHash() common.Hash32B
	}

	// stateFactory implements StateFactory interface, tracks changes in a map and batch-commits to trie/db
	stateFactory struct {
		trie trie.Trie
	}
)

func stateToBytes(s *State) ([]byte, error) {
	var ss bytes.Buffer
	e := gob.NewEncoder(&ss)
	if err := e.Encode(s); err != nil {
		return nil, ErrFailedToMarshalState
	}
	return ss.Bytes(), nil
}

func bytesToState(ss []byte) (*State, error) {
	var state State
	e := gob.NewDecoder(bytes.NewBuffer(ss))
	if err := e.Decode(&state); err != nil {
		return nil, ErrFailedToUnmarshalState
	}
	return &state, nil
}

//======================================
// functions for State
//======================================

// AddBalance adds balance for state
func (st *State) AddBalance(amount *big.Int) error {
	st.Balance.Add(st.Balance, amount)
	return nil
}

// SubBalance subtracts balance for state
func (st *State) SubBalance(amount *big.Int) error {
	// make sure there's enough fund to spend
	if amount.Cmp(st.Balance) == 1 {
		return ErrNotEnoughBalance
	}
	st.Balance.Sub(st.Balance, amount)
	return nil
}

//======================================
// functions for StateFactory
//======================================

// NewStateFactory creates a new state factory
func NewStateFactory(trie trie.Trie) StateFactory {
	return &stateFactory{trie: trie}
}

// NewStateFactoryTrieDB creates a new stateFactory from Trie
func NewStateFactoryTrieDB(dbPath string) (StateFactory, error) {
	if len(dbPath) == 0 {
		return nil, nil
	}
	tr, err := trie.NewTrie(dbPath)
	if err != nil {
		return nil, err
	}
	return &stateFactory{trie: tr}, nil
}

// CreateState adds a new State with initial balance to the factory
func (sf *stateFactory) CreateState(addr string, init uint64) (*State, error) {
	pubKeyHash := iotxaddress.GetPubkeyHash(addr)
	if pubKeyHash == nil {
		return nil, ErrInvalidAddr
	}
	balance := big.NewInt(0)
	balance.SetUint64(init)
	s := State{Address: addr, Balance: balance}
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
func (sf *stateFactory) Balance(addr string) (*big.Int, error) {
	state, err := sf.getState(addr)
	if err != nil {
		return nil, err
	}
	return state.Balance, nil
}

// Nonce returns the nonce if the account exists
func (sf *stateFactory) Nonce(addr string) (uint64, error) {
	state, err := sf.getState(addr)
	if err != nil {
		return 0, err
	}
	return state.Nonce, nil
}

// SetNonce returns the nonce if the account exists
func (sf *stateFactory) SetNonce(addr string, value uint64) error {
	state, err := sf.getState(addr)
	if err != nil {
		return err
	}
	state.Nonce = value

	mstate, err := stateToBytes(state)
	if err != nil {
		return err
	}
	if err := sf.trie.Upsert(iotxaddress.GetPubkeyHash(addr), mstate); err != nil {
		return err
	}
	return nil
}

// RootHash returns the hash of the root node of the trie
func (sf *stateFactory) RootHash() common.Hash32B {
	return sf.trie.RootHash()
}

// CommitStateChanges updates a State from the given actions
func (sf *stateFactory) CommitStateChanges(tsf []*trx.Transfer, vote []*trx.Vote) error {
	pending := make(map[common.PKHash]*State)
	for _, tx := range tsf {
		var pubKeyHash common.PKHash
		var err error
		// check sender
		pkhash := iotxaddress.GetPubkeyHash(tx.Sender)
		if pkhash == nil {
			return ErrInvalidAddr
		}
		copy(pubKeyHash[:], pkhash)
		sender, exist := pending[pubKeyHash]
		if !exist {
			if sender, err = sf.getState(tx.Sender); err != nil {
				return err
			}
			pending[pubKeyHash] = sender
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
		// check recipient
		if pkhash = iotxaddress.GetPubkeyHash(tx.Recipient); pkhash == nil {
			return ErrInvalidAddr
		}
		copy(pubKeyHash[:], pkhash)
		recipient, exist := pending[pubKeyHash]
		if !exist {
			recipient, err = sf.getState(tx.Recipient)
			switch {
			case err == ErrAccountNotExist:
				if _, e := sf.CreateState(tx.Recipient, 0); e != nil {
					return e
				}
			case err != nil:
				return err
			}
			pending[pubKeyHash] = recipient
		}
		// update recipient balance
		if err := recipient.AddBalance(tx.Amount); err != nil {
			return err
		}
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
	}
	// commit the state changes to Trie in a batch
	return sf.trie.Commit(transferK, transferV)
}

//======================================
// private functions
//=====================================
// getState pulls an existing State
func (sf *stateFactory) getState(addr string) (*State, error) {
	pubKeyHash := iotxaddress.GetPubkeyHash(addr)
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
	state, err := bytesToState(mstate)
	if err != nil {
		return nil, err
	}
	return state, nil
}
