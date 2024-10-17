package actpool

import (
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
)

var (
	_balance    = big.NewInt(0).SetBytes([]byte("100000000000000000000000"))
	_expireTime = time.Hour
)

func TestAccountPool_PopPeek(t *testing.T) {
	r := require.New(t)
	t.Run("empty pool", func(t *testing.T) {
		ap := newAccountPool()
		r.Nil(ap.PopPeek())
	})
	t.Run("one action", func(t *testing.T) {
		ap := newAccountPool()
		tsf1, err := action.SignedTransfer(_addr2, _priKey1, 1, big.NewInt(100), nil, uint64(0), big.NewInt(1))
		r.NoError(err)
		r.NoError(ap.PutAction(_addr1, nil, 0, _balance, _expireTime, tsf1))
		r.Equal(tsf1, ap.PopPeek())
		r.Equal(0, ap.Account(_addr1).Len())
	})
	t.Run("multiple actions in one account", func(t *testing.T) {
		ap := newAccountPool()
		tsf1, err := action.SignedTransfer(_addr1, _priKey1, 1, big.NewInt(100), nil, uint64(0), big.NewInt(1))
		r.NoError(err)
		tsf2, err := action.SignedTransfer(_addr1, _priKey1, 2, big.NewInt(100), nil, uint64(0), big.NewInt(1))
		r.NoError(err)
		r.NoError(ap.PutAction(_addr1, nil, 1, _balance, _expireTime, tsf1))
		r.NoError(ap.PutAction(_addr1, nil, 1, _balance, _expireTime, tsf2))
		r.Equal(tsf2, ap.PopPeek())
		r.Equal(tsf1, ap.PopPeek())
		r.Nil(ap.PopPeek())
		r.Equal(0, ap.Account(_addr1).Len())
	})
	t.Run("peek with pending nonce", func(t *testing.T) {
		ap := newAccountPool()
		tsf1, err := action.SignedTransfer(_addr1, _priKey1, 1, big.NewInt(100), nil, uint64(0), big.NewInt(1))
		r.NoError(err)
		tsf2, err := action.SignedTransfer(_addr2, _priKey2, 2, big.NewInt(100), nil, uint64(0), big.NewInt(1))
		r.NoError(err)
		r.NoError(ap.PutAction(_addr1, nil, 1, _balance, _expireTime, tsf1))
		r.NoError(ap.PutAction(_addr2, nil, 1, _balance, _expireTime, tsf2))
		r.Equal(tsf2, ap.PopPeek())
		t.Run("even if with higher price", func(t *testing.T) {
			tsf2, err := action.SignedTransfer(_addr2, _priKey2, 2, big.NewInt(100), nil, uint64(0), big.NewInt(2))
			r.NoError(err)
			r.NoError(ap.PutAction(_addr2, nil, 1, _balance, _expireTime, tsf2))
			r.Equal(tsf2, ap.PopPeek())
		})
	})
	t.Run("peek with lower gas price", func(t *testing.T) {
		ap := newAccountPool()
		tsf1, err := action.SignedTransfer(_addr1, _priKey1, 2, big.NewInt(100), nil, uint64(0), big.NewInt(2))
		r.NoError(err)
		tsf2, err := action.SignedTransfer(_addr2, _priKey2, 2, big.NewInt(100), nil, uint64(0), big.NewInt(1))
		r.NoError(err)
		r.NoError(ap.PutAction(_addr1, nil, 1, _balance, _expireTime, tsf1))
		r.NoError(ap.PutAction(_addr2, nil, 1, _balance, _expireTime, tsf2))
		r.Equal(tsf2, ap.PopPeek())
		r.Equal(tsf1, ap.PopPeek())
		t.Run("peek with pending nonce even if has higher price ", func(t *testing.T) {
			tsf1, err = action.SignedTransfer(_addr1, _priKey1, 1, big.NewInt(100), nil, uint64(0), big.NewInt(2))
			r.NoError(err)
			tsf2, err = action.SignedTransfer(_addr1, _priKey2, 1, big.NewInt(100), nil, uint64(0), big.NewInt(1))
			r.NoError(err)
			r.NoError(ap.PutAction(_addr1, nil, 1, _balance, _expireTime, tsf1))
			r.NoError(ap.PutAction(_addr2, nil, 1, _balance, _expireTime, tsf2))
			r.Equal(tsf2, ap.PopPeek())
			r.Equal(tsf1, ap.PopPeek())
		})
	})
	t.Run("multiple actions in multiple accounts", func(t *testing.T) {
		ap := newAccountPool()
		tsf1, err := action.SignedTransfer(_addr1, _priKey1, 1, big.NewInt(100), nil, uint64(0), big.NewInt(2))
		r.NoError(err)
		tsf2, err := action.SignedTransfer(_addr2, _priKey1, 2, big.NewInt(100), nil, uint64(0), big.NewInt(1))
		r.NoError(err)
		tsf3, err := action.SignedTransfer(_addr2, _priKey2, 2, big.NewInt(100), nil, uint64(0), big.NewInt(1))
		r.NoError(err)
		tsf4, err := action.SignedTransfer(_addr2, _priKey2, 3, big.NewInt(100), nil, uint64(0), big.NewInt(1))
		r.NoError(err)
		tsf5, err := action.SignedTransfer(_addr2, _priKey3, 1, big.NewInt(100), nil, uint64(0), big.NewInt(1))
		r.NoError(err)
		tsf6, err := action.SignedTransfer(_addr2, _priKey3, 3, big.NewInt(100), nil, uint64(0), big.NewInt(1))
		r.NoError(err)
		tsf7, err := action.SignedTransfer(_addr2, _priKey4, 2, big.NewInt(100), nil, uint64(0), big.NewInt(1))
		r.NoError(err)
		tsf8, err := action.SignedTransfer(_addr2, _priKey4, 3, big.NewInt(100), nil, uint64(0), big.NewInt(1))
		r.NoError(err)
		r.NoError(ap.PutAction(_addr1, nil, 1, _balance, _expireTime, tsf1))
		r.NoError(ap.PutAction(_addr1, nil, 1, _balance, _expireTime, tsf2))
		r.NoError(ap.PutAction(_addr2, nil, 1, _balance, _expireTime, tsf3))
		r.NoError(ap.PutAction(_addr2, nil, 1, _balance, _expireTime, tsf4))
		r.NoError(ap.PutAction(_addr3, nil, 1, _balance, _expireTime, tsf5))
		r.NoError(ap.PutAction(_addr3, nil, 1, _balance, _expireTime, tsf6))
		r.NoError(ap.PutAction(_addr4, nil, 1, _balance, _expireTime, tsf7))
		r.NoError(ap.PutAction(_addr4, nil, 1, _balance, _expireTime, tsf8))
		r.Equal(tsf4, ap.PopPeek())
		r.Equal(tsf3, ap.PopPeek())
		r.Equal(tsf8, ap.PopPeek())
		r.Equal(tsf7, ap.PopPeek())
		r.Equal(tsf6, ap.PopPeek())
		r.Equal(tsf5, ap.PopPeek())
		r.Equal(tsf2, ap.PopPeek())
		r.Equal(tsf1, ap.PopPeek())
		r.Nil(ap.PopPeek())
	})
}

func TestAccountPool_PopAccount(t *testing.T) {
	r := require.New(t)
	ap := newAccountPool()

	// Create a sample account
	tsf1, err := action.SignedTransfer(_addr1, _priKey1, 1, big.NewInt(100), nil, uint64(0), big.NewInt(1))
	r.NoError(err)
	r.NoError(ap.PutAction(_addr1, nil, 1, _balance, _expireTime, tsf1))

	// Test when the account exists
	result := ap.PopAccount(_addr1)
	r.Equal(1, result.Len())
	r.Equal(tsf1, result.AllActs()[0])
	r.Nil(ap.Account(_addr1))

	// Test when the account does not exist
	result = ap.PopAccount("nonExistentAddress")
	r.Nil(result)
}

func TestAccountPool_Range(t *testing.T) {
	r := require.New(t)
	ap := newAccountPool()

	// Create a sample account
	tsf1, err := action.SignedTransfer(_addr1, _priKey1, 1, big.NewInt(100), nil, uint64(0), big.NewInt(1))
	r.NoError(err)
	tsf2, err := action.SignedTransfer(_addr1, _priKey1, 2, big.NewInt(100), nil, uint64(0), big.NewInt(1))
	r.NoError(err)
	tsf3, err := action.SignedTransfer(_addr1, _priKey2, 2, big.NewInt(100), nil, uint64(0), big.NewInt(1))
	r.NoError(err)
	tsf4, err := action.SignedTransfer(_addr1, _priKey2, 1, big.NewInt(100), nil, uint64(0), big.NewInt(2))
	r.NoError(err)
	r.NoError(ap.PutAction(_addr1, nil, 1, _balance, _expireTime, tsf1))
	r.NoError(ap.PutAction(_addr1, nil, 1, _balance, _expireTime, tsf2))
	r.NoError(ap.PutAction(_addr2, nil, 1, _balance, _expireTime, tsf3))
	r.Equal(2, ap.Account(_addr1).Len())
	r.Equal(1, ap.Account(_addr2).Len())
	// Define a callback function
	callback := func(addr string, acct ActQueue) {
		if addr == _addr2 {
			r.NoError(acct.Put(tsf4))
		} else if addr == _addr1 {
			acct.PopActionWithLargestNonce()
		}
	}
	// Call the Range method
	ap.Range(callback)
	// Verify the results
	r.Equal(1, ap.Account(_addr1).Len())
	r.Equal(2, ap.Account(_addr2).Len())
	r.Equal(tsf1, ap.PopPeek())
	r.Equal(tsf3, ap.PopPeek())
	r.Equal(tsf4, ap.PopPeek())
	r.Nil(ap.PopPeek())
}

func TestAccountPool_DeleteIfEmpty(t *testing.T) {
	r := require.New(t)
	ap := newAccountPool()

	// Create a sample account
	tsf1, err := action.SignedTransfer(_addr1, _priKey1, 1, big.NewInt(100), nil, uint64(0), big.NewInt(1))
	r.NoError(err)
	r.NoError(ap.PutAction(_addr1, nil, 1, _balance, _expireTime, tsf1))

	// Test when the account is not empty
	ap.DeleteIfEmpty(_addr1)
	r.NotNil(ap.Account(_addr1))

	// Test when the account is empty
	ap.PopPeek()
	ap.DeleteIfEmpty(_addr1)
	r.Nil(ap.Account(_addr1))
}
