// Copyright (c) 2022 IoTeX Foundation
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
	"github.com/iotexproject/iotex-core/testutil/testdb"
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
	r.Equal(-1, bytes.Compare(_bucketPoolAddrKey, bucketKey(0)))

	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)

	pool, err := newCandidateStateReader(sm).NewBucketPool(false)
	r.NoError(err)
	r.Equal(big.NewInt(0), pool.Total())
	r.EqualValues(0, pool.Count())
	r.Equal(false, pool.enableSMStorage)

	// add 4 buckets
	addr := identityset.Address(1)
	for i := 0; i < 4; i++ {
		_, err = newCandidateStateManager(sm).putBucket(NewVoteBucket(addr, addr, big.NewInt(10000), 21, time.Now(), true))
		r.NoError(err)
	}

	view, _, err := CreateBaseView(sm, false, false)
	r.NoError(err)
	sm.WriteView(_protocolID, view)
	pool = view.bucketPool
	total := big.NewInt(40000)
	count := uint64(4)
	r.Equal(total, pool.Total())
	r.Equal(count, pool.Count())
	r.Equal(false, pool.enableSMStorage)

	tests := []struct {
		debit, newBucket, postGreenland, commit bool
		amount                                  *big.Int
		expected                                error
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

	// simulate bucket pool operation success, but sm did not commit (Snapshot() implements workingset.Reset(), clearing data stored in Dock())
	csm, err := NewCandidateStateManager(sm, false)
	r.NoError(err)
	r.NoError(csm.DebitBucketPool(tests[0].amount, true))
	sm.Snapshot()

	// after that, the base view should not change
	c, err := ConstructBaseView(sm)
	r.NoError(err)
	pool = c.BaseView().bucketPool
	r.Equal(total, pool.Total())
	r.Equal(count, pool.Count())

	var testGreenland bool
	for _, v := range tests {
		csm, err = NewCandidateStateManager(sm, v.postGreenland && testGreenland)
		r.NoError(err)
		// dirty view always follows the latest change
		pool = csm.DirtyView().bucketPool
		r.Equal(total, pool.Total())
		r.Equal(count, pool.Count())
		if v.debit {
			err = csm.DebitBucketPool(v.amount, v.newBucket)
		} else {
			err = csm.CreditBucketPool(v.amount)
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
			// after commit, value should reflect in base view
			c, err = ConstructBaseView(sm)
			r.NoError(err)
			pool = c.BaseView().bucketPool
			r.Equal(total, pool.Total())
			r.Equal(count, pool.Count())
		}

		if !testGreenland && v.postGreenland {
			_, err = sm.PutState(c.BaseView().bucketPool.total, protocol.NamespaceOption(_stakingNameSpace), protocol.KeyOption(_bucketPoolAddrKey))
			r.NoError(err)
			testGreenland = true
		}
	}

	// verify state has been created successfully
	var b totalAmount
	_, err = sm.State(&b, protocol.NamespaceOption(_stakingNameSpace), protocol.KeyOption(_bucketPoolAddrKey))
	r.NoError(err)
	r.Equal(total, b.amount)
	r.Equal(count, b.count)

	// test again bucket pool operation success but sm did not commit
	csm, err = NewCandidateStateManager(sm, true)
	r.NoError(err)
	r.NoError(csm.DebitBucketPool(tests[0].amount, true))
	sm.Snapshot()

	c, err = ConstructBaseView(sm)
	r.NoError(err)
	pool = c.BaseView().bucketPool
	r.Equal(total, pool.Total())
	r.Equal(count, pool.Count())
}
