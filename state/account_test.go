// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package state

import (
	"encoding/hex"
	"math"
	"math/big"
	"testing"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestNonce(t *testing.T) {
	require := require.New(t)
	t.Run("legacy account type", func(t *testing.T) {
		acct, err := NewAccount(LegacyNonceAccountTypeOption())
		require.NoError(err)
		require.Equal(uint64(1), acct.PendingNonce())
		require.Error(acct.SetPendingNonce(0))
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
	t.Run("legacy fresh account type", func(t *testing.T) {
		acct, err := NewAccount(LegacyNonceAccountTypeOption())
		require.NoError(err)
		require.Equal(uint64(1), acct.PendingNonce())
		require.Error(acct.SetPendingNonce(0))
		require.Error(acct.SetPendingNonce(3))
		require.NoError(acct.SetPendingNonce(1))
		require.Equal(uint64(1), acct.PendingNonce())
	})
}

func TestNonceOverflow(t *testing.T) {
	require := require.New(t)
	t.Run("account nonce uint64 max", func(t *testing.T) {
		acct, err := NewAccount()
		require.NoError(err)
		var target uint64 = math.MaxUint64
		acct.nonce = uint64(target)
		require.ErrorIs(acct.SetPendingNonce(target+1), ErrNonceOverflow)
		require.Equal(target, acct.PendingNonce())
	})
	t.Run("account nonce uint64 max-1", func(t *testing.T) {
		acct, err := NewAccount()
		require.NoError(err)
		var target uint64 = math.MaxUint64 - 1
		acct.nonce = uint64(target)
		require.NoError(acct.SetPendingNonce(target + 1))
		require.Equal(target+1, acct.PendingNonce())
	})
	t.Run("legacy account nonce uint64 max", func(t *testing.T) {
		acct, err := NewAccount(LegacyNonceAccountTypeOption())
		require.NoError(err)
		var target uint64 = math.MaxUint64
		acct.nonce = uint64(target)
		require.ErrorIs(acct.SetPendingNonce(target+2), ErrNonceOverflow)
		require.Equal(target+1, acct.PendingNonce())
	})
	t.Run("legacy account nonce uint64 max-1", func(t *testing.T) {
		acct, err := NewAccount(LegacyNonceAccountTypeOption())
		require.NoError(err)
		var target uint64 = math.MaxUint64 - 1
		acct.nonce = uint64(target)
		require.ErrorIs(acct.SetPendingNonce(target+2), ErrNonceOverflow)
		require.Equal(target+1, acct.PendingNonce())
	})
	t.Run("legacy account nonce uint64 max-2", func(t *testing.T) {
		acct, err := NewAccount(LegacyNonceAccountTypeOption())
		require.NoError(err)
		var target uint64 = math.MaxUint64 - 2
		acct.nonce = uint64(target)
		require.NoError(acct.SetPendingNonce(target + 2))
		require.Equal(target+2, acct.PendingNonce())
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

func TestConvertFreshAddress(t *testing.T) {
	// TODO: fix this test @dustinxie
	t.Skip()
	require := require.New(t)

	var (
		s1, _ = NewAccount(LegacyNonceAccountTypeOption())
		s2, _ = NewAccount(LegacyNonceAccountTypeOption())
		s3, _ = NewAccount()
		s4, _ = NewAccount()
	)
	s2.nonce, s4.nonce = 1, 1

	for i, v := range []struct {
		s                    *Account
		accType, cvtType     int32
		first, second, third uint64
	}{
		{s1, 0, 1, 1, 0, 1},
		{s2, 0, 0, 2, 2, 3},
		{s3, 1, 1, 0, 0, 1},
		{s4, 1, 1, 1, 1, 2},
	} {
		require.Equal(v.accType, v.s.accountType)
		require.Equal(v.first, v.s.PendingNonce())
		require.Equal(v.second, v.s.PendingNonceConsideringFreshAccount())
		// trying convert using pending nonce does not take effect
		// require.False(v.s.ConvertFreshAccountToZeroNonceType(v.first))
		require.Equal(v.accType, v.s.accountType)
		// only adjusted nonce can convert legacy fresh address to zero-nonce type
		// require.Equal(v.s.IsLegacyFreshAccount() && v.second == 0, v.s.ConvertFreshAccountToZeroNonceType(v.second))
		require.Equal(v.cvtType, v.s.accountType)
		// after conversion, fresh address is still fresh
		require.Equal(i == 0 || i == 2, v.s.IsNewbieAccount())
		// after conversion, 2 pending nonces become the same
		require.Equal(v.second, v.s.PendingNonce())
		require.Equal(v.second, v.s.PendingNonceConsideringFreshAccount())
		require.NoError(v.s.SetPendingNonce(v.second + 1))
		// for dirty address, 2 pending nonces are the same
		require.Equal(v.third, v.s.PendingNonce())
		require.Equal(v.third, v.s.PendingNonceConsideringFreshAccount())
	}
}
