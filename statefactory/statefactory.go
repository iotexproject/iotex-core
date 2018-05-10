package statefactory

import (
	"bytes"
	"encoding/gob"
	"errors"
	"math/big"
	"sync"

	"github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/trie"
)

var (
	stateFactoryKVNameSpace = "StateFactory"

	// ErrNotEnoughBalance is the error that the balance is not enough
	ErrNotEnoughBalance = errors.New("not enough balance")

	// ErrAccountNotExist is the error that the account does not exist
	ErrAccountNotExist = errors.New("the account does not exist")
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

// New creates a new StateFactory
func New(db db.KVStore, trie trie.Trie) StateFactory {
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
func (sf *StateFactory) Balance(addr *iotxaddress.Address) *big.Int {
	key := iotxaddress.HashPubKey(addr.PublicKey)
	state, err := sf.trie.Get(key)
	if err != nil {
		panic(err)
	}

	s := bytesToState(state)
	return &s.Balance
}

// SubBalance minuses balance to the given address
func (sf *StateFactory) SubBalance(addr *iotxaddress.Address, amount *big.Int) error {
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
func (sf *StateFactory) Nonce(addr *iotxaddress.Address) (uint64, error) {
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
func (sf *StateFactory) SetNonce(addr iotxaddress.Address, value uint64) error {
	key := iotxaddress.HashPubKey(addr.PublicKey)
	state, err := sf.trie.Get(key)
	if err != nil {
		panic(err)
	}

	s := bytesToState(state)
	s.Nonce = value
	sf.trie.Update(key, stateToBytes(s))
	return nil
}

const hashedAddressLen = 20

type hashedAddress [hashedAddressLen]byte

// VritualStateFactory tracks changes to StateFactory in a map but never commits to trie/db
type VritualStateFactory struct {
	sf *StateFactory

	changes map[hashedAddress]*State
	mu      sync.Mutex
}

// SetStateFactory sets the backing layer
func (vs *VritualStateFactory) SetStateFactory(sf *StateFactory) {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	vs.sf = sf

	vs.changes = make(map[hashedAddress]*State)
}

// Nonce returns the nonce if the account exists
func (vs *VritualStateFactory) Nonce(addr *iotxaddress.Address) (uint64, error) {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	var key hashedAddress
	k := iotxaddress.HashPubKey(addr.PublicKey)
	copy(key[:], k[:hashedAddressLen])
	if val, ok := vs.changes[key]; ok {
		return val.Nonce, nil
	}

	state, err := vs.sf.trie.Get(key[:])
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
func (vs *VritualStateFactory) SetNonce(addr *iotxaddress.Address, value uint64) error {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	var key hashedAddress
	k := iotxaddress.HashPubKey(addr.PublicKey)
	copy(key[:], k[:hashedAddressLen])
	if _, ok := vs.changes[key]; ok {
		vs.changes[key].Nonce = value
		return nil
	}

	state, err := vs.sf.trie.Get(key[:])
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
