// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"math/big"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/staking/stakingpb"
	"github.com/iotexproject/iotex-core/state"
)

// const
const (
	_stakingBucketPool = "bucketPool"
)

var (
	_bucketPoolAddr    = hash.Hash160b([]byte(_stakingBucketPool))
	_bucketPoolAddrKey = append([]byte{_const}, _bucketPoolAddr[:]...)
)

// when a bucket is created, the amount of staked IOTX token is deducted from user, but does not transfer to any address
// in the same way when a bucket are withdrawn, bucket amount is added back to user, but does not come out of any address
//
// for better accounting/auditing, we take protocol's address as the 'bucket pool' address
// 1. at Greenland height we sum up all existing bucket's amount and set the total amount to bucket pool address
// 2. for future bucket creation/deposit/registration, the amount of staked IOTX token will be added to bucket pool (so
//    the pool is 'receiving' token)
// 3. for future bucket withdrawal, the bucket amount will be deducted from bucket pool (so the pool is 'releasing' token)

type (
	// BucketPool implements the bucket pool
	BucketPool struct {
		enableSMStorage bool
		total           *totalAmount
	}

	totalAmount struct {
		amount *big.Int
		count  uint64
	}
)

func (t *totalAmount) Serialize() ([]byte, error) {
	gen := stakingpb.TotalAmount{
		Amount: t.amount.String(),
		Count:  t.count,
	}
	return proto.Marshal(&gen)
}

func (t *totalAmount) Deserialize(data []byte) error {
	gen := stakingpb.TotalAmount{}
	if err := proto.Unmarshal(data, &gen); err != nil {
		return err
	}

	var ok bool
	if t.amount, ok = new(big.Int).SetString(gen.Amount, 10); !ok {
		return state.ErrStateDeserialization
	}

	if t.amount.Cmp(big.NewInt(0)) == -1 {
		return state.ErrNotEnoughBalance
	}
	t.count = gen.Count
	return nil
}

func (t *totalAmount) AddBalance(amount *big.Int, newBucket bool) {
	t.amount.Add(t.amount, amount)
	if newBucket {
		t.count++
	}
}

func (t *totalAmount) SubBalance(amount *big.Int) error {
	if amount.Cmp(t.amount) == 1 || t.count == 0 {
		return state.ErrNotEnoughBalance
	}
	t.amount.Sub(t.amount, amount)
	t.count--
	return nil
}

// NewBucketPool creates an instance of BucketPool
func NewBucketPool(sr protocol.StateReader, enableSMStorage bool) (*BucketPool, error) {
	bp := BucketPool{
		enableSMStorage: enableSMStorage,
		total: &totalAmount{
			amount: big.NewInt(0),
		},
	}

	if bp.enableSMStorage {
		switch _, err := sr.State(bp.total, protocol.NamespaceOption(StakingNameSpace), protocol.KeyOption(_bucketPoolAddrKey)); errors.Cause(err) {
		case nil:
			return &bp, nil
		case state.ErrStateNotExist:
			// fall back to load all buckets
		default:
			return nil, err
		}
	}

	// sum up all existing buckets
	all, _, err := getAllBuckets(sr)
	if err != nil && errors.Cause(err) != state.ErrStateNotExist {
		return nil, err
	}

	for _, v := range all {
		if v.StakedAmount.Cmp(big.NewInt(0)) <= 0 {
			return nil, state.ErrNotEnoughBalance
		}
		bp.total.amount.Add(bp.total.amount, v.StakedAmount)
	}
	bp.total.count = uint64(len(all))
	return &bp, nil
}

// Total returns the total amount staked in bucket pool
func (bp *BucketPool) Total() *big.Int {
	return new(big.Int).Set(bp.total.amount)
}

// Count returns the total number of buckets in bucket pool
func (bp *BucketPool) Count() uint64 {
	return bp.total.count
}

// Copy returns a copy of the bucket pool
func (bp *BucketPool) Copy(enableSMStorage bool) *BucketPool {
	pool := BucketPool{}
	pool.enableSMStorage = enableSMStorage
	pool.total = &totalAmount{
		amount: new(big.Int).Set(bp.total.amount),
		count:  bp.total.count,
	}
	return &pool
}

// Sync syncs the data from state manager
func (bp *BucketPool) Sync(sm protocol.StateManager) error {
	if bp.enableSMStorage {
		_, err := sm.State(bp.total, protocol.NamespaceOption(StakingNameSpace), protocol.KeyOption(_bucketPoolAddrKey))
		return err
	}
	// get stashed total amount
	err := sm.Unload(_protocolID, _stakingBucketPool, bp.total)
	if err != nil && err != protocol.ErrNoName {
		return err
	}
	return nil
}

// Commit is called upon workingset commit
func (bp *BucketPool) Commit(sr protocol.StateReader) error {
	return nil
}

// CreditPool subtracts staked amount out of the pool
func (bp *BucketPool) CreditPool(sm protocol.StateManager, amount *big.Int) error {
	if err := bp.total.SubBalance(amount); err != nil {
		return err
	}

	if bp.enableSMStorage {
		_, err := sm.PutState(bp.total, protocol.NamespaceOption(StakingNameSpace), protocol.KeyOption(_bucketPoolAddrKey))
		return err
	}
	return sm.Load(_protocolID, _stakingBucketPool, bp.total)
}

// DebitPool adds staked amount into the pool
func (bp *BucketPool) DebitPool(sm protocol.StateManager, amount *big.Int, newBucket bool) error {
	bp.total.AddBalance(amount, newBucket)
	if bp.enableSMStorage {
		_, err := sm.PutState(bp.total, protocol.NamespaceOption(StakingNameSpace), protocol.KeyOption(_bucketPoolAddrKey))
		return err
	}
	return sm.Load(_protocolID, _stakingBucketPool, bp.total)
}
