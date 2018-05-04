package statefactory

import (
	"bytes"
	"encoding/gob"
	"errors"
	"math/big"

	"github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/iotxaddress"
	tr "github.com/iotexproject/iotex-core/trie"
)

var (
	stateFactoryKVNameSpace = "StateFactory"
	// ErrNotEnoughBalance is the error that the balance is not enough
	ErrNotEnoughBalance = errors.New("not enough balance")
)

// State is the canonical representation of an account.
type State struct {
	Nonce   uint64
	Balance big.Int
	Address *iotxaddress.Address

	IsCandidate  bool
	VotingWeight *big.Int
	Voters       map[common.Hash32B]*big.Int
}

// StateFactory manages states.
type StateFactory struct {
	db   db.KVStore
	trie tr.Trie
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

// New creates a new StateFactory
func New(db db.KVStore, trie tr.Trie) StateFactory {
	return StateFactory{db: db, trie: trie}
}

// RootHash returns the hash of the root node of the trie
func (sf *StateFactory) RootHash() common.Hash32B {
	return sf.trie.RootHash()
}

// AddState adds a new State with zero balance to the factory
func (sf *StateFactory) AddState(addr *iotxaddress.Address) *State {
	s := State{Address: addr, Balance: *big.NewInt(0)}
	key := iotxaddress.HashPubKey(addr.PublicKey)
	sf.trie.Update(key, stateToBytes(&s))
	return &s
}

// Balance returns balance.
func (sf *StateFactory) Balance(addr iotxaddress.Address) *big.Int {
	key := iotxaddress.HashPubKey(addr.PublicKey)
	state, err := sf.trie.Get(key)
	if err != nil {
		panic(err)
	}

	s := bytesToState(state)
	return &s.Balance
}

// SubBalance minuses balance to the given address
func (sf *StateFactory) SubBalance(addr iotxaddress.Address, amount *big.Int) error {
	key := iotxaddress.HashPubKey(addr.PublicKey)
	state, err := sf.trie.Get(key)
	if err != nil {
		panic(err)
	}

	s := bytesToState(state)
	if amount.Cmp(&s.Balance) == 1 {
		return ErrNotEnoughBalance
	}
	s.Balance.Sub(&s.Balance, amount)
	sf.trie.Update(key, stateToBytes(s))
	return nil
}

// AddBalance adds balance to the given address
func (sf *StateFactory) AddBalance(addr *iotxaddress.Address, amount *big.Int) error {
	key := iotxaddress.HashPubKey(addr.PublicKey)
	ss, err := sf.trie.Get(key)
	if err != nil {
		panic(err)
	}

	var state *State
	if len(ss) == 0 {
		state = sf.AddState(addr)
	} else {
		state = bytesToState(ss)
	}

	state.Balance.Add(&state.Balance, amount)
	sf.trie.Update(key, stateToBytes(state))
	return nil
}

// Nonce returns the nonce for the given address
func (sf *StateFactory) Nonce(addr iotxaddress.Address) uint64 {
	key := iotxaddress.HashPubKey(addr.PublicKey)
	state, err := sf.trie.Get(key)
	if err != nil {
		panic(err)
	}

	s := bytesToState(state)
	return s.Nonce
}

// IncreaseNonce increase nonce by 1
func (sf *StateFactory) IncreaseNonce(addr iotxaddress.Address) error {
	key := iotxaddress.HashPubKey(addr.PublicKey)
	state, err := sf.trie.Get(key)
	if err != nil {
		panic(err)
	}

	s := bytesToState(state)
	s.Nonce = s.Nonce + 1
	sf.trie.Update(key, stateToBytes(s))
	return nil
}
