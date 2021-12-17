// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package state

import (
	"math/big"

	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/action/protocol/account/accountpb"
)

var (
	// ErrNotEnoughBalance is the error that the balance is not enough
	ErrNotEnoughBalance = errors.New("not enough balance")
	// ErrAccountCollision is the error that the account already exists
	ErrAccountCollision = errors.New("account already exists")
)

// Account is the canonical representation of an account.
type Account struct {
	// 0 is reserved from actions in genesis block and coinbase transfers nonces
	// other actions' nonces start from 1
	Nonce        uint64
	Balance      *big.Int
	Root         hash.Hash256 // storage trie root for contract account
	CodeHash     []byte       // hash of the smart contract byte-code for contract account
	IsCandidate  bool
	VotingWeight *big.Int
}

// toProto converts to protobuf's Account
func (st *Account) toProto() *accountpb.Account {
	acPb := &accountpb.Account{}
	acPb.Nonce = st.Nonce
	if st.Balance != nil {
		acPb.Balance = st.Balance.String()
	}
	acPb.Root = make([]byte, len(st.Root))
	copy(acPb.Root, st.Root[:])
	acPb.CodeHash = make([]byte, len(st.CodeHash))
	copy(acPb.CodeHash, st.CodeHash)
	acPb.IsCandidate = st.IsCandidate
	if st.VotingWeight != nil {
		acPb.VotingWeight = st.VotingWeight.Bytes()
	}
	return acPb
}

// Serialize serializes account state into bytes
func (st Account) Serialize() ([]byte, error) {
	return proto.Marshal(st.toProto())
}

// fromProto converts from protobuf's Account
func (st *Account) fromProto(acPb *accountpb.Account) {
	st.Nonce = acPb.Nonce
	st.Balance = big.NewInt(0)
	if acPb.Balance != "" {
		st.Balance.SetString(acPb.Balance, 10)
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
}

// Deserialize deserializes bytes into account state
func (st *Account) Deserialize(buf []byte) error {
	acPb := &accountpb.Account{}
	if err := proto.Unmarshal(buf, acPb); err != nil {
		return err
	}
	st.fromProto(acPb)
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

// PendingNonce returns the pending nonce of account
func (st *Account) PendingNonce() uint64 {
	if st.Nonce == 0 {
		// to be compatible with Ethereum, where a new account's first tx use nonce = 0
		return st.Nonce
	}
	return st.Nonce + 1
}

// IsContract returns true for contract account
func (st *Account) IsContract() bool {
	return len(st.CodeHash) > 0
}

// Clone clones the account state
func (st *Account) Clone() *Account {
	s := *st
	s.Balance = nil
	s.Balance = new(big.Int).Set(st.Balance)
	s.VotingWeight = nil
	if st.VotingWeight != nil {
		s.VotingWeight = new(big.Int).Set(st.VotingWeight)
	}
	if st.CodeHash != nil {
		s.CodeHash = nil
		s.CodeHash = make([]byte, len(st.CodeHash))
		copy(s.CodeHash, st.CodeHash)
	}
	return &s
}

// EmptyAccount returns an empty account
func EmptyAccount() Account {
	return Account{
		Balance:      big.NewInt(0),
		VotingWeight: big.NewInt(0),
	}
}
