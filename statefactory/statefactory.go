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
	"sync"

	"github.com/pkg/errors"

	trx "github.com/iotexproject/iotex-core/blockchain/trx"
	"github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/trie"
)

var (
	stateFactoryKVNameSpace = "StateFactory"

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
		Address      *iotxaddress.Address
		IsCandidate  bool
		VotingWeight *big.Int
		Voters       map[common.Hash32B]*big.Int
	}

	// StateFactory defines an interface for managing states
	StateFactory interface {
		CreateState(*iotxaddress.Address, uint64) (*State, error)
		Balance(*iotxaddress.Address) (*big.Int, error)
		UpdateStatesWithTransfer([]*trx.Tx) error
		SetNonce(*iotxaddress.Address, uint64) error
		Nonce(*iotxaddress.Address) (uint64, error)
		RootHash() common.Hash32B
	}

	// stateFactory implements StateFactory interface
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

// NewStateFactory creates a new stateFactory
func NewStateFactory(trie trie.Trie) StateFactory {
	return &stateFactory{trie: trie}
}

// RootHash returns the hash of the root node of the trie
func (sf *stateFactory) RootHash() common.Hash32B {
	return sf.trie.RootHash()
}

// CreateState adds a new State with initial balance to the factory
func (sf *stateFactory) CreateState(addr *iotxaddress.Address, init uint64) (*State, error) {
	balance := big.NewInt(0)
	balance.SetUint64(init)
	s := State{Address: addr, Balance: balance}
	mstate, err := stateToBytes(&s)
	if err != nil {
		return nil, err
	}
	if err := sf.trie.Upsert(iotxaddress.HashPubKey(addr.PublicKey), mstate); err != nil {
		return nil, err
	}
	return &s, nil
}

// Balance returns balance.
func (sf *stateFactory) Balance(addr *iotxaddress.Address) (*big.Int, error) {
	state, err := sf.getState(addr)
	if err != nil {
		return nil, err
	}
	return state.Balance, nil
}

// UpdateStatesWithTransfer updates a State from the given value transfer
func (sf *stateFactory) UpdateStatesWithTransfer(txs []*trx.Tx) error {
	var ss []byte
	transferK := [][]byte{}
	transferV := [][]byte{}
	for _, tx := range txs {
		// check sender
		sender, err := sf.getState(&iotxaddress.Address{PublicKey: tx.SenderPublicKey})
		if err != nil {
			return err
		}
		if tx.Amount.Cmp(sender.Balance) == 1 {
			return ErrNotEnoughBalance
		}
		// check recipient
		receiver, err := sf.getState(tx.Recipient)
		switch {
		case err == ErrAccountNotExist:
			if _, e := sf.CreateState(tx.Recipient, 0); e != nil {
				return e
			}
		case err != nil:
			return err
		}
		// update sender balance
		if err := sender.SubBalance(tx.Amount); err != nil {
			return err
		}
		if ss, err = stateToBytes(sender); err != nil {
			return err
		}
		transferK = append(transferK, iotxaddress.HashPubKey(tx.SenderPublicKey))
		transferV = append(transferV, ss)
		// update recipient balance
		if err := receiver.AddBalance(tx.Amount); err != nil {
			return err
		}
		if ss, err = stateToBytes(receiver); err != nil {
			return err
		}
		transferK = append(transferK, iotxaddress.HashPubKey(tx.Recipient.PublicKey))
		transferV = append(transferV, ss)
	}
	// commit the state changes to Trie in a batch
	return sf.trie.Commit(transferK, transferV)
}

// Nonce returns the nonce for the given address
func (sf *stateFactory) Nonce(addr *iotxaddress.Address) (uint64, error) {
	state, err := sf.getState(addr)
	if err != nil {
		return 0, err
	}
	return state.Nonce, nil
}

// SetNonce sets nonce to a given value
func (sf *stateFactory) SetNonce(addr *iotxaddress.Address, value uint64) error {
	state, err := sf.getState(addr)
	if err != nil {
		return err
	}
	state.Nonce = value

	mstate, err := stateToBytes(state)
	if err != nil {
		return err
	}
	key := iotxaddress.HashPubKey(addr.PublicKey)
	if err := sf.trie.Upsert(key, mstate); err != nil {
		return err
	}
	return nil
}

// getState pulls an existing State
func (sf *stateFactory) getState(addr *iotxaddress.Address) (*State, error) {
	mstate, err := sf.trie.Get(iotxaddress.HashPubKey(addr.PublicKey))
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

//======================================
// functions for State
//======================================
func (st *State) AddBalance(amount *big.Int) error {
	st.Balance.Add(st.Balance, amount)
	return nil
}

func (st *State) SubBalance(amount *big.Int) error {
	// make sure there's enough fund to spend
	if amount.Cmp(st.Balance) == 1 {
		return ErrNotEnoughBalance
	}
	st.Balance.Sub(st.Balance, amount)
	return nil
}

//======================================
// functions for VirtualStateFactory
//======================================
const hashedAddressLen = 20

type hashedAddress [hashedAddressLen]byte

// VirtualStateFactory implements StateFactory interface, tracks changes in a map but never commits to trie/db
type virtualStateFactory struct {
	changes map[hashedAddress]*State
	mu      sync.Mutex
	trie    trie.Trie
}

// NewVirtualStateFactory creates a new virtual state factory
func NewVirtualStateFactory(trie trie.Trie) StateFactory {
	return &virtualStateFactory{trie: trie, changes: make(map[hashedAddress]*State)}
}

func (vs *virtualStateFactory) CreateState(addr *iotxaddress.Address, init uint64) (*State, error) {
	balance := big.NewInt(0)
	balance.SetUint64(init)
	s := State{Address: addr, Balance: balance}
	mstate, err := stateToBytes(&s)
	if err != nil {
		return nil, err
	}
	if err := vs.trie.Upsert(iotxaddress.HashPubKey(addr.PublicKey), mstate); err != nil {
		return nil, err
	}
	return &s, nil
}

func (vs *virtualStateFactory) Balance(addr *iotxaddress.Address) (*big.Int, error) {
	state, err := vs.getState(addr)
	if err != nil {
		return nil, err
	}
	return state.Balance, nil
}

// Nonce returns the nonce if the account exists
func (vs *virtualStateFactory) Nonce(addr *iotxaddress.Address) (uint64, error) {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	var key hashedAddress
	k := iotxaddress.HashPubKey(addr.PublicKey)
	copy(key[:], k[:hashedAddressLen])
	if val, ok := vs.changes[key]; ok {
		return val.Nonce, nil
	}

	state, err := vs.getState(addr)
	if err != nil {
		return 0, err
	}
	vs.changes[key] = state
	return vs.changes[key].Nonce, nil
}

// SetNonce returns the nonce if the account exists
func (vs *virtualStateFactory) SetNonce(addr *iotxaddress.Address, value uint64) error {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	var key hashedAddress
	k := iotxaddress.HashPubKey(addr.PublicKey)
	copy(key[:], k[:hashedAddressLen])
	if _, ok := vs.changes[key]; ok {
		vs.changes[key].Nonce = value
		return nil
	}

	state, err := vs.getState(addr)
	if err != nil {
		return err
	}
	vs.changes[key] = state
	vs.changes[key].Nonce = value
	return nil
}

func (vs *virtualStateFactory) UpdateStatesWithTransfer([]*trx.Tx) error {
	// TODO
	return nil
}

func (vs *virtualStateFactory) RootHash() common.Hash32B {
	// TODO
	return [32]byte{}
}

func (vs *virtualStateFactory) getState(addr *iotxaddress.Address) (*State, error) {
	mstate, err := vs.trie.Get(iotxaddress.HashPubKey(addr.PublicKey))
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
