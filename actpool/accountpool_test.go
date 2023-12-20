package actpool

import (
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
)

func TestAccountPool_PopPeek(t *testing.T) {
	r := require.New(t)
	balance := big.NewInt(0).SetBytes([]byte("100000000000000000000000"))
	expireTime := time.Hour
	t.Run("empty pool", func(t *testing.T) {
		ap := newAccountPool()
		r.Nil(ap.PopPeek())
	})
	t.Run("one action", func(t *testing.T) {
		ap := newAccountPool()
		tsf1, err := action.SignedTransfer(_addr2, _priKey1, 1, big.NewInt(100), nil, uint64(0), big.NewInt(1))
		r.NoError(err)
		r.NoError(ap.PutAction("acc", nil, 0, balance, expireTime, tsf1))
		r.Equal(&tsf1, ap.PopPeek())
	})
	t.Run("peek with pending nonce", func(t *testing.T) {
		ap := newAccountPool()
		tsf1, err := action.SignedTransfer(_addr1, _priKey1, 1, big.NewInt(100), nil, uint64(0), big.NewInt(1))
		r.NoError(err)
		tsf2, err := action.SignedTransfer(_addr2, _priKey1, 2, big.NewInt(100), nil, uint64(0), big.NewInt(1))
		r.NoError(err)
		r.NoError(ap.PutAction(_addr1, nil, 1, balance, expireTime, tsf1))
		r.NoError(ap.PutAction(_addr2, nil, 1, balance, expireTime, tsf2))
		r.Equal(&tsf2, ap.PopPeek())
		t.Run("even if with higher price", func(t *testing.T) {
			tsf2, err := action.SignedTransfer(_addr2, _priKey1, 2, big.NewInt(100), nil, uint64(0), big.NewInt(2))
			r.NoError(err)
			r.NoError(ap.PutAction(_addr2, nil, 1, balance, expireTime, tsf2))
			r.Equal(&tsf2, ap.PopPeek())
		})
	})
	t.Run("peek with lower gas price", func(t *testing.T) {
		ap := newAccountPool()
		tsf1, err := action.SignedTransfer(_addr1, _priKey1, 2, big.NewInt(100), nil, uint64(0), big.NewInt(2))
		r.NoError(err)
		tsf2, err := action.SignedTransfer(_addr2, _priKey1, 2, big.NewInt(100), nil, uint64(0), big.NewInt(1))
		r.NoError(err)
		r.NoError(ap.PutAction(_addr1, nil, 1, balance, expireTime, tsf1))
		r.NoError(ap.PutAction(_addr2, nil, 1, balance, expireTime, tsf2))
		r.Equal(&tsf2, ap.PopPeek())
		t.Run("peek with pending nonce even if has higher price ", func(t *testing.T) {
			tsf3, err := action.SignedTransfer(_addr1, _priKey1, 1, big.NewInt(100), nil, uint64(0), big.NewInt(1))
			r.NoError(err)
			r.NoError(ap.PutAction(_addr2, nil, 1, balance, expireTime, tsf3))
		})
	})
}
