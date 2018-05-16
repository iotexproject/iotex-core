// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package statefactory

import (
	"bytes"
	"encoding/gob"
	"errors"
	"math/big"
	"sync"

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

	// ErrImpossibleTransition is the error that the state transition is not possible
	ErrImpossibleTransition = errors.New("impossible state transition")
)

// State is the canonical representation of an account.
type State struct {
	Nonce   uint64
	Balance *big.Int
	Address *iotxaddress.Address

	IsCandidate  bool
	VotingWeight *big.Int
	Voters       map[common.Hash32B]*big.Int
}

// StateFactory defines an interface for managing states
type StateFactory interface {
	RootHash() common.Hash32B

	CreateState(addr *iotxaddress.Address) *State
	UpdateStateWithTransfer(senderPubKey []byte, amount *big.Int, recipient *iotxaddress.Address) error

	SetNonce(addr *iotxaddress.Address, value uint64) error
	Nonce(addr *iotxaddress.Address) (uint64, error)

	AddBalance(addr *iotxaddress.Address, amount *big.Int) error
	SubBalance(addr *iotxaddress.Address, amount *big.Int) error
	Balance(addr *iotxaddress.Address) (*big.Int, error)
}

// stateFactory implements StateFactory interface
type stateFactory struct {
	trie trie.Trie
}

func stateToBytes(s *State) []byte {
	var ss bytes.Buffer
	e := gob.NewEncoder(&ss)
	if err := e.Encode(s); err != nil {
		panic(err)
	}
	return ss.Bytes()
}

func bytesToState(ss []byte) *State {
	var state State
	e := gob.NewDecoder(bytes.NewBuffer(ss))
	if err := e.Decode(&state); err != nil {
		panic(err)
	}
	return &state
}

// NewStateFactory creates a new stateFactory
func NewStateFactory(trie trie.Trie) StateFactory {
	return &stateFactory{trie: trie}
}

// RootHash returns the hash of the root node of the trie
func (sf *stateFactory) RootHash() common.Hash32B {
	return sf.trie.RootHash()
}

// UpdateStateWithTransfer updates a State from the given value transfer
func (sf *stateFactory) UpdateStateWithTransfer(senderPubKey []byte, amount *big.Int, recipient *iotxaddress.Address) error {
	sender := iotxaddress.HashPubKey(senderPubKey)
	state, err := sf.trie.Get(sender)
	if err != nil {
		panic(err)
	}
	if state == nil {
		return ErrAccountNotExist
	}

	// check sender
	if amount.Cmp(bytesToState(state).Balance) == 1 {
		return ErrImpossibleTransition
	}

	// check recipient
	_, err = sf.Balance(recipient)
	if err == ErrAccountNotExist {
		sf.CreateState(recipient)
	}

	if err := sf.SubBalance(&iotxaddress.Address{PublicKey: senderPubKey}, amount); err != nil {
		return err
	}
	if err := sf.AddBalance(recipient, amount); err != nil {
		return err
	}
	return nil
}

// CreateState adds a new State with zero balance to the factory
func (sf *stateFactory) CreateState(addr *iotxaddress.Address) *State {
	s := State{Address: addr, Balance: big.NewInt(0)}
	key := iotxaddress.HashPubKey(addr.PublicKey)
	sf.trie.Update(key, stateToBytes(&s))
	return &s
}

// Balance returns balance.
func (sf *stateFactory) Balance(addr *iotxaddress.Address) (*big.Int, error) {
	key := iotxaddress.HashPubKey(addr.PublicKey)
	state, err := sf.trie.Get(key)
	if err != nil {
		panic(err)
	}
	if state == nil {
		return nil, ErrAccountNotExist
	}

	s := bytesToState(state)
	return s.Balance, nil
}

// SubBalance minuses balance to the given address
func (sf *stateFactory) SubBalance(addr *iotxaddress.Address, amount *big.Int) error {
	key := iotxaddress.HashPubKey(addr.PublicKey)
	state, err := sf.trie.Get(key)
	if err != nil {
		panic(err)
	}
	if state == nil {
		return ErrAccountNotExist
	}

	s := bytesToState(state)
	if amount.Cmp(s.Balance) == 1 {
		return ErrNotEnoughBalance
	}
	s.Balance.Sub(s.Balance, amount)
	sf.trie.Update(key, stateToBytes(s))
	return nil
}

// AddBalance adds balance to the given address
func (sf *stateFactory) AddBalance(addr *iotxaddress.Address, amount *big.Int) error {
	key := iotxaddress.HashPubKey(addr.PublicKey)
	ss, err := sf.trie.Get(key)
	if err != nil {
		panic(err)
	}
	if len(ss) == 0 {
		return ErrAccountNotExist
	}

	state := bytesToState(ss)
	state.Balance.Add(state.Balance, amount)
	sf.trie.Update(key, stateToBytes(state))
	return nil
}

// Nonce returns the nonce for the given address
func (sf *stateFactory) Nonce(addr *iotxaddress.Address) (uint64, error) {
	key := iotxaddress.HashPubKey(addr.PublicKey)
	state, err := sf.trie.Get(key)
	if err != nil {
		panic(err)
	}
	if state == nil {
		return 0, ErrAccountNotExist
	}

	s := bytesToState(state)
	return s.Nonce, nil
}

// SetNonce sets nonce to a given value
func (sf *stateFactory) SetNonce(addr *iotxaddress.Address, value uint64) error {
	key := iotxaddress.HashPubKey(addr.PublicKey)
	state, err := sf.trie.Get(key)
	if err != nil {
		panic(err)
	}
	if state == nil {
		return ErrAccountNotExist
	}

	s := bytesToState(state)
	s.Nonce = value
	sf.trie.Update(key, stateToBytes(s))
	return nil
}

const hashedAddressLen = 20

type hashedAddress [hashedAddressLen]byte

// VirtualStateFactory implements StateFactory interface, tracks changes in a map but never commits to trie/db
type VirtualStateFactory struct {
	changes map[hashedAddress]*State
	mu      sync.Mutex
	trie    trie.Trie
}

// NewVirtualStateFactory creates a new virtual state factory
func NewVirtualStateFactory(trie trie.Trie) StateFactory {
	return &VirtualStateFactory{trie: trie, changes: make(map[hashedAddress]*State)}
}

// Nonce returns the nonce if the account exists
func (vs *VirtualStateFactory) Nonce(addr *iotxaddress.Address) (uint64, error) {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	var key hashedAddress
	k := iotxaddress.HashPubKey(addr.PublicKey)
	copy(key[:], k[:hashedAddressLen])
	if val, ok := vs.changes[key]; ok {
		return val.Nonce, nil
	}

	state, err := vs.trie.Get(key[:])
	if err != nil {
		panic(err)
	}
	if state == nil {
		return 0, ErrAccountNotExist
	}

	vs.changes[key] = bytesToState(state)
	return vs.changes[key].Nonce, nil
}

// SetNonce returns the nonce if the account exists
func (vs *VirtualStateFactory) SetNonce(addr *iotxaddress.Address, value uint64) error {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	var key hashedAddress
	k := iotxaddress.HashPubKey(addr.PublicKey)
	copy(key[:], k[:hashedAddressLen])
	if _, ok := vs.changes[key]; ok {
		vs.changes[key].Nonce = value
		return nil
	}

	state, err := vs.trie.Get(key[:])
	if err != nil {
		panic(err)
	}
	if state == nil {
		return ErrAccountNotExist
	}

	vs.changes[key] = bytesToState(state)
	vs.changes[key].Nonce = value
	return nil
}

func (vs *VirtualStateFactory) AddBalance(addr *iotxaddress.Address, amount *big.Int) error {
	// TODO
	return nil
}
func (vs *VirtualStateFactory) SubBalance(addr *iotxaddress.Address, amount *big.Int) error {
	// TODO
	return nil
}

func (vs *VirtualStateFactory) Balance(addr *iotxaddress.Address) (*big.Int, error) {
	// TODO
	return nil, nil
}
func (vs *VirtualStateFactory) CreateState(addr *iotxaddress.Address) *State {
	// TODO
	return nil
}

func (vs *VirtualStateFactory) UpdateStateWithTransfer(senderPubKey []byte, amount *big.Int, recipient *iotxaddress.Address) error {
	// TODO
	return nil
}

func (vs *VirtualStateFactory) RootHash() common.Hash32B {
	// TODO
	return [32]byte{}
}
