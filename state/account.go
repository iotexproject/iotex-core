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
	// ErrInvalidAmount is the error that the amount to add is negative
	ErrInvalidAmount = errors.New("invalid amount")
	// ErrAccountCollision is the error that the account already exists
	ErrAccountCollision = errors.New("account already exists")
	// ErrInvalidNonce is the error that the nonce to set is invalid
	ErrInvalidNonce = errors.New("invalid nonce")
)

// DelegateCandidateOption is an option to create a delegate candidate account
func DelegateCandidateOption() AccountCreationOption {
	return func(account *Account) error {
		account.isCandidate = true
		return nil
	}
}

type (
	// AccountCreationOption is to create new account with specific settings
	AccountCreationOption func(*Account) error

	// Account is the canonical representation of an account.
	Account struct {
		// 0 is reserved from actions in genesis block and coinbase transfers nonces
		// other actions' nonces start from 1
		nonce        uint64
		Balance      *big.Int
		Root         hash.Hash256 // storage trie root for contract account
		CodeHash     []byte       // hash of the smart contract byte-code for contract account
		isCandidate  bool
		votingWeight *big.Int
	}
)

// ToProto converts to protobuf's Account
func (st *Account) ToProto() *accountpb.Account {
	acPb := &accountpb.Account{}
	acPb.Nonce = st.nonce
	if st.Balance != nil {
		acPb.Balance = st.Balance.String()
	}
	acPb.Root = make([]byte, len(st.Root))
	copy(acPb.Root, st.Root[:])
	acPb.CodeHash = make([]byte, len(st.CodeHash))
	copy(acPb.CodeHash, st.CodeHash)
	acPb.IsCandidate = st.isCandidate
	if st.votingWeight != nil {
		acPb.VotingWeight = st.votingWeight.Bytes()
	}
	return acPb
}

// Serialize serializes account state into bytes
func (st Account) Serialize() ([]byte, error) {
	return proto.Marshal(st.ToProto())
}

// FromProto converts from protobuf's Account
func (st *Account) FromProto(acPb *accountpb.Account) {
	st.nonce = acPb.Nonce
	if acPb.Balance == "" {
		st.Balance = big.NewInt(0)
	} else {
		balance, ok := new(big.Int).SetString(acPb.Balance, 10)
		if !ok {
			errors.Errorf("invalid balance %s", acPb.Balance)
		}
		st.Balance = balance
	}
	copy(st.Root[:], acPb.Root)
	st.CodeHash = nil
	if acPb.CodeHash != nil {
		st.CodeHash = make([]byte, len(acPb.CodeHash))
		copy(st.CodeHash, acPb.CodeHash)
	}
	st.isCandidate = acPb.IsCandidate
	st.votingWeight = big.NewInt(0)
	if acPb.VotingWeight != nil {
		st.votingWeight.SetBytes(acPb.VotingWeight)
	}
}

// Deserialize deserializes bytes into account state
func (st *Account) Deserialize(buf []byte) error {
	acPb := &accountpb.Account{}
	if err := proto.Unmarshal(buf, acPb); err != nil {
		return err
	}
	st.FromProto(acPb)
	return nil
}

// SetNonce sets the nonce of the account
func (st *Account) SetNonce(nonce uint64) error {
	if nonce != st.nonce+1 {
		return errors.Wrapf(ErrInvalidNonce, "actual value %d, %d expected", nonce, st.nonce+1)
	}
	st.nonce = nonce
	return nil
}

// PendingNonce returns the pending nonce of the account
func (st *Account) PendingNonce() uint64 {
	return st.nonce + 1
}

// HasSufficientBalance returns true if balance is larger than amount
func (st *Account) HasSufficientBalance(amount *big.Int) bool {
	if amount == nil {
		return true
	}
	return amount.Cmp(st.Balance) <= 0
}

// AddBalance adds balance for account state
func (st *Account) AddBalance(amount *big.Int) error {
	if amount == nil || amount.Sign() < 0 {
		return errors.Wrapf(ErrInvalidAmount, "amount %s shouldn't be negative", amount.String())
	}
	if st.Balance != nil {
		st.Balance = new(big.Int).Add(st.Balance, amount)
	} else {
		st.Balance = new(big.Int).Set(amount)
	}
	return nil
}

// SubBalance subtracts balance for account state
func (st *Account) SubBalance(amount *big.Int) error {
	if amount == nil || amount.Cmp(big.NewInt(0)) < 0 {
		return errors.Wrapf(ErrInvalidAmount, "amount %s shouldn't be negative", amount.String())
	}
	// make sure there's enough fund to spend
	if amount.Cmp(st.Balance) == 1 {
		return ErrNotEnoughBalance
	}
	st.Balance.Sub(st.Balance, amount)
	return nil
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
	s.votingWeight = nil
	if st.votingWeight != nil {
		s.votingWeight = new(big.Int).Set(st.votingWeight)
	}
	if st.CodeHash != nil {
		s.CodeHash = nil
		s.CodeHash = make([]byte, len(st.CodeHash))
		copy(s.CodeHash, st.CodeHash)
	}
	return &s
}

// NewEmptyAccount returns an empty account
func NewEmptyAccount() *Account {
	return &Account{
		Balance:      big.NewInt(0),
		votingWeight: big.NewInt(0),
	}
}

// NewAccount creates a new account with options
func NewAccount(opts ...AccountCreationOption) (*Account, error) {
	account := &Account{
		Balance:      big.NewInt(0),
		votingWeight: big.NewInt(0),
	}
	for _, opt := range opts {
		if err := opt(account); err != nil {
			return nil, errors.Wrap(err, "failed to apply account creation option")
		}
	}
	return account, nil
}
