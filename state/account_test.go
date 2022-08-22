// Copyright (c) 2019 IoTeX Foundation
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
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/stretchr/testify/require"
)

func TestNonce(t *testing.T) {
	require := require.New(t)
	t.Run("legacy account type", func(t *testing.T) {
		acct, err := NewAccount(LegacyNonceAccountTypeOption())
		require.NoError(err)
		require.Equal(uint64(1), acct.PendingNonce())
		require.Error(acct.SetPendingNonce(0))
		require.Error(acct.SetPendingNonce(1))
		require.Error(acct.SetPendingNonce(3))
		require.NoError(acct.SetPendingNonce(2))
		require.Equal(uint64(2), acct.PendingNonce())
	})
	t.Run("zero nonce account type", func(t *testing.T) {
		acct, err := NewAccount()
		require.NoError(err)
		require.Equal(uint64(0), acct.PendingNonce())
		require.Error(acct.SetPendingNonce(2))
		require.NoError(acct.SetPendingNonce(1))
		require.Equal(uint64(1), acct.PendingNonce())
	})
}

func TestEncodeDecode(t *testing.T) {
	require := require.New(t)

	for _, test := range []struct {
		accountType int32
		expectedLen int
	}{
		{
			1, 66,
		},
		{
			0, 64,
		},
	} {
		acc := Account{
			accountType: test.accountType,
			Balance:     big.NewInt(20000000),
			nonce:       0x10,
			CodeHash:    []byte("testing codehash"),
		}
		ss, err := acc.Serialize()
		require.NoError(err)
		require.NotEmpty(ss)
		require.Equal(test.expectedLen, len(ss))

		s2 := Account{}
		require.NoError(s2.Deserialize(ss))
		require.Equal(acc.accountType, s2.accountType)
		require.Equal(acc.Balance, s2.Balance)
		require.Equal(acc.nonce, s2.nonce)
		require.Equal(hash.ZeroHash256, s2.Root)
		require.Equal(acc.CodeHash, s2.CodeHash)
	}
}

func TestProto(t *testing.T) {
	require := require.New(t)

	for _, test := range []struct {
		accountType int32
		raw         string
	}{
		{
			0, "1201301a200000000000000000000000000000000000000000000000000000000000000000",
		},
		{
			1, "1201301a2000000000000000000000000000000000000000000000000000000000000000003801",
		},
	} {
		acc := Account{accountType: test.accountType}
		ss, _ := hex.DecodeString(test.raw)
		require.NoError(acc.Deserialize(ss))
		bytes, err := acc.Serialize()
		require.NoError(err)
		require.Equal(test.raw, hex.EncodeToString(bytes))
	}
}

func TestBalance(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	state := &Account{Balance: big.NewInt(20)}
	// Add 10 to the balance
	require.NoError(state.AddBalance(big.NewInt(10)))
	// Balance should == 30 now
	require.Equal(0, state.Balance.Cmp(big.NewInt(30)))
	// Sub 40 to the balance
	require.Equal(ErrNotEnoughBalance, state.SubBalance(big.NewInt(40)))

	require.True(state.HasSufficientBalance(big.NewInt(30)))
	require.False(state.HasSufficientBalance(big.NewInt(31)))

	require.Contains(state.AddBalance(big.NewInt(-1)).Error(), ErrInvalidAmount.Error())
	require.Contains(state.SubBalance(big.NewInt(-1)).Error(), ErrInvalidAmount.Error())
	require.Contains(state.AddBalance(nil).Error(), ErrInvalidAmount.Error())
	require.Contains(state.SubBalance(nil).Error(), ErrInvalidAmount.Error())
}

func TestClone(t *testing.T) {
	require := require.New(t)
	ss := &Account{
		nonce:   0x10,
		Balance: big.NewInt(200),
	}
	account := ss.Clone()
	require.Equal(big.NewInt(200), account.Balance)

	require.NoError(account.AddBalance(big.NewInt(100)))
	require.Equal(big.NewInt(200), ss.Balance)
	require.Equal(big.NewInt(200+100), account.Balance)
}
