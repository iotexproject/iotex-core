// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package state

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/golang/mock/gomock"

	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/stretchr/testify/require"
)

func TestEncodeDecode(t *testing.T) {
	require := require.New(t)
	s1 := Account{
		Nonce:        0x10,
		Balance:      big.NewInt(20000000),
		CodeHash:     []byte("testing codehash"),
		VotingWeight: big.NewInt(1000000000),
		Votee:        "testing votee",
	}
	ss, err := s1.Serialize()
	require.NoError(err)
	require.NotEmpty(ss)
	require.Equal(85, len(ss))

	s2 := Account{}
	require.NoError(s2.Deserialize(ss))
	require.Equal(big.NewInt(20000000), s2.Balance)
	require.Equal(uint64(0x10), s2.Nonce)
	require.Equal(hash.ZeroHash256, s2.Root)
	require.Equal([]byte("testing codehash"), s2.CodeHash)
	require.Equal(big.NewInt(1000000000), s2.VotingWeight)
	require.Equal("testing votee", s2.Votee)
}

func TestProto(t *testing.T) {
	require := require.New(t)
	raw := "1201301a200000000000000000000000000000000000000000000000000000000000000000"
	ss, _ := hex.DecodeString(raw)
	s1 := Account{}
	require.NoError(Deserialize(&s1, ss))
	d, err := Serialize(s1)
	require.NoError(err)
	require.Equal(raw, hex.EncodeToString(d))
}

func TestBalance(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	state := &Account{Balance: big.NewInt(20)}
	// Add 10 to the balance
	err := state.AddBalance(big.NewInt(10))
	require.Nil(err)
	// Balance should == 30 now
	require.Equal(0, state.Balance.Cmp(big.NewInt(30)))
}

func TestClone(t *testing.T) {
	require := require.New(t)
	ss := &Account{
		Nonce:        0x10,
		Balance:      big.NewInt(200),
		VotingWeight: big.NewInt(1000),
	}
	account := ss.Clone()
	require.Equal(big.NewInt(200), account.Balance)
	require.Equal(big.NewInt(1000), account.VotingWeight)

	require.Nil(account.AddBalance(big.NewInt(100)))
	account.VotingWeight.Sub(account.VotingWeight, big.NewInt(300))
	require.Equal(big.NewInt(200), ss.Balance)
	require.Equal(big.NewInt(1000), ss.VotingWeight)
	require.Equal(big.NewInt(200+100), account.Balance)
	require.Equal(big.NewInt(1000-300), account.VotingWeight)
}
