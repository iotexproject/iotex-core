// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"bytes"
	"math/big"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/identityset"
)

func TestTotalAmount(t *testing.T) {
	r := require.New(t)

	a := totalAmount{
		count: 1,
	}
	ser, err := a.Serialize()
	r.NoError(err)
	b := totalAmount{}
	// amount = nil leads to unmarshal error
	r.Error(b.Deserialize(ser))

	a.amount = big.NewInt(10)
	ser, err = a.Serialize()
	r.NoError(err)
	r.NoError(b.Deserialize(ser))
	r.Equal(a, b)

	// test sub balance
	r.Equal(state.ErrNotEnoughBalance, a.SubBalance(big.NewInt(11)))
	r.NoError(a.SubBalance(big.NewInt(4)))
	r.Equal(big.NewInt(6), a.amount)
	r.EqualValues(0, a.count)
	r.Equal(state.ErrNotEnoughBalance, a.SubBalance(big.NewInt(1)))

	// test add balance
	a.AddBalance(big.NewInt(1), true)
	r.Equal(big.NewInt(7), a.amount)
	r.EqualValues(1, a.count)
	a.AddBalance(big.NewInt(0), false)
	r.Equal(big.NewInt(7), a.amount)
	r.EqualValues(1, a.count)
}

func TestBucketPool(t *testing.T) {
	r := require.New(t)

	// bucket pool address does not interfere with buckets data
	r.Equal(-1, bytes.Compare(bucketPoolAddrKey, bucketKey(0)))

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	sm := newMockStateManager(ctrl)

	pool, err := NewBucketPool(sm)
	r.NoError(err)
	r.Equal(big.NewInt(0), pool.Total())
	r.EqualValues(0, pool.Count())
	r.Equal(false, pool.exist)

	// add 4 buckets
	addr := identityset.Address(1)
	for i := 0; i < 4; i++ {
		_, err = putBucket(sm, NewVoteBucket(addr, addr, big.NewInt(10000), 21, time.Now(), true))
		r.NoError(err)
	}

	c, err := createCandCenter(sm)
	r.NoError(err)
	sm.WriteView(protocolID, c)
	pool = c.BucketPool()
	total := big.NewInt(40000)
	count := uint64(4)
	r.Equal(total, pool.Total())
	r.Equal(count, pool.Count())
	r.Equal(false, pool.exist)

	tests := []struct {
		debit, newBucket, create, commit bool
		amount                           *big.Int
		expected                         error
	}{
		{true, true, false, false, big.NewInt(1000), nil},
		{false, true, false, false, big.NewInt(200), nil},
		{true, true, false, true, big.NewInt(300), nil},
		{false, true, false, false, big.NewInt(22200), nil},
		{false, true, false, false, big.NewInt(60000), state.ErrNotEnoughBalance},
		{true, false, false, true, big.NewInt(400), nil},
		// below test created staking bucket pool
		{true, false, true, true, big.NewInt(500), nil},
		{false, false, true, false, big.NewInt(1000), nil},
		{true, false, true, true, big.NewInt(600), nil},
	}

	for _, v := range tests {
		csm, err := NewCandidateStateManager(sm)
		r.NoError(err)
		pool = c.BucketPool()
		r.Equal(total, pool.Total())
		r.Equal(count, pool.Count())
		if v.debit {
			err = csm.DebitBucketPool(v.amount, v.newBucket, v.create)
		} else {
			err = csm.CreditBucketPool(v.amount, v.create)
		}
		r.Equal(v.expected, err)

		if v.expected != nil {
			continue
		}

		if v.debit {
			total.Add(total, v.amount)
			if v.newBucket {
				count++
			}
		} else {
			total.Sub(total, v.amount)
			count--
		}

		if v.commit {
			r.NoError(csm.Commit())
		}
		r.Equal(v.create, pool.exist)
	}
	r.True(pool.exist)
	r.Equal(total, pool.Total())
	r.Equal(count, pool.Count())

	var b totalAmount
	_, err = sm.State(&b, protocol.NamespaceOption(StakingNameSpace), protocol.KeyOption(bucketPoolAddrKey))
	r.NoError(err)
	r.Equal(total, b.amount)
	r.Equal(count, b.count)
}
