// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package state

import (
	"bytes"
	"encoding/gob"
	"math/big"

	"github.com/iotexproject/iotex-core-internal/pkg/hash"
)

// State is the canonical representation of an account.
type State struct {
	// 0 is reserved from actions in genesis block and coinbase transfers nonces
	// other actions' nonces start from 1
	Nonce        uint64
	Balance      *big.Int
	Root         hash.Hash32B
	CodeHash     []byte // hash of the smart contract code if the account is a contract
	Address      string
	IsCandidate  bool
	VotingWeight *big.Int
	Votee        string
	Voters       map[string]*big.Int
}

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
