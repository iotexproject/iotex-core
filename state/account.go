// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package state

import (
	"math/big"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/proto"
)

var (
	// ErrNotEnoughBalance is the error that the balance is not enough
	ErrNotEnoughBalance = errors.New("not enough balance")
	// ErrAccountCollision is the error that the account already exists
	ErrAccountCollision = errors.New("account already exists")
	// EmptyAccount indicates an empty account
	// This is a read-only variable for comparison purpose. Caller should not modify it.
	EmptyAccount = &Account{
		Balance:      big.NewInt(0),
		VotingWeight: big.NewInt(0),
	}
)

// Account is the canonical representation of an account.
type Account struct {
	// 0 is reserved from actions in genesis block and coinbase transfers nonces
	// other actions' nonces start from 1
	Nonce        uint64
	Balance      *big.Int
	Root         hash.Hash32B // storage trie root for contract account
	CodeHash     []byte       // hash of the smart contract byte-code for contract account
	IsCandidate  bool
	VotingWeight *big.Int
	Votee        string
}

// ToProto converts to protobuf's AccountPb
func (st *Account) ToProto() *iproto.AccountPb {
	acPb := &iproto.AccountPb{}
	acPb.Nonce = st.Nonce
	if st.Balance != nil {
		acPb.Balance = st.Balance.Bytes()
	}
	acPb.Root = make([]byte, hash.HashSize)
	copy(acPb.Root, st.Root[:])
	acPb.CodeHash = make([]byte, len(st.CodeHash))
	copy(acPb.CodeHash, st.CodeHash)
	acPb.IsCandidate = st.IsCandidate
	if st.VotingWeight != nil {
		acPb.VotingWeight = st.VotingWeight.Bytes()
	}
	acPb.Votee = st.Votee
	return acPb
}

// Serialize serializes account state into bytes
func (st Account) Serialize() ([]byte, error) {
	return proto.Marshal(st.ToProto())
}

// FromProto converts from protobuf's AccountPb
func (st *Account) FromProto(acPb *iproto.AccountPb) {
	st.Nonce = acPb.Nonce
	st.Balance = big.NewInt(0)
	if acPb.Balance != nil {
		st.Balance.SetBytes(acPb.Balance)
	}
	copy(st.Root[:], acPb.Root)
	st.CodeHash = nil
	if acPb.CodeHash != nil {
		st.CodeHash = make([]byte, len(acPb.CodeHash))
		copy(st.CodeHash, acPb.CodeHash)
	}
	st.IsCandidate = acPb.IsCandidate
	st.VotingWeight = big.NewInt(0)
	if acPb.VotingWeight != nil {
		st.VotingWeight.SetBytes(acPb.VotingWeight)
	}
	st.Votee = acPb.Votee
}

// Deserialize deserializes bytes into account state
func (st *Account) Deserialize(buf []byte) error {
	acPb := &iproto.AccountPb{}
	if err := proto.Unmarshal(buf, acPb); err != nil {
		return err
	}
	st.FromProto(acPb)
	return nil
}

// AddBalance adds balance for account state
func (st *Account) AddBalance(amount *big.Int) error {
	st.Balance.Add(st.Balance, amount)
	return nil
}

// SubBalance subtracts balance for account state
func (st *Account) SubBalance(amount *big.Int) error {
	// make sure there's enough fund to spend
	if amount.Cmp(st.Balance) == 1 {
		return ErrNotEnoughBalance
	}
	st.Balance.Sub(st.Balance, amount)
	return nil
}

// Clone clones the account state
func (st *Account) Clone() *Account {
	s := *st
	s.Balance = nil
	s.Balance = new(big.Int).Set(st.Balance)
	s.VotingWeight = nil
	s.VotingWeight = new(big.Int).Set(st.VotingWeight)
	if st.CodeHash != nil {
		s.CodeHash = nil
		s.CodeHash = make([]byte, len(st.CodeHash))
		copy(s.CodeHash, st.CodeHash)
	}
	return &s
}
