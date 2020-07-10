// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"fmt"
	"math/big"

	"github.com/golang/protobuf/proto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/staking/stakingpb"
	"github.com/iotexproject/iotex-core/state"
)

// const
const (
	stakingBucketPool = "bucketPool"
)

var (
	bucketPoolAddr = hash.Hash160b([]byte(stakingBucketPool))
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
	bucketPool struct {
		exist bool
		total *totalAmount
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

// newBucketPool creates an instance of bucketPool
func newBucketPool(sr protocol.StateReader) (*bucketPool, error) {
	bp := bucketPool{
		total: &totalAmount{
			amount: big.NewInt(0),
		},
	}

	var err error
	bp.exist, err = bp.poolExist(sr)
	if err != nil {
		return nil, err
	}

	if bp.exist {
		_, err := sr.State(bp.total, protocol.LegacyKeyOption(bucketPoolAddr))
		return &bp, err
	}

	// sum up all existing buckets
	all, err := getAllBuckets(sr)
	if err != nil {
		return nil, err
	}

	for _, v := range all {
		bp.total.amount.Add(bp.total.amount, v.StakedAmount)
	}
	bp.total.count = uint64(len(all))
	return &bp, nil
}

func (bp *bucketPool) Total() *big.Int {
	return new(big.Int).Set(bp.total.amount)
}

func (bp *bucketPool) Count() uint64 {
	return bp.total.count
}

func (bp *bucketPool) Exist() bool {
	return bp.exist
}

func (bp *bucketPool) poolExist(sr protocol.StateReader) (bool, error) {
	_, err := sr.State(bp.total, protocol.LegacyKeyOption(bucketPoolAddr))
	if err == nil {
		return true, nil
	}
	if errors.Cause(err) == state.ErrStateNotExist {
		return false, nil
	}
	return false, err
}

func (bp *bucketPool) Clone() *bucketPool {
	pool := bucketPool{}
	pool.exist = bp.exist
	pool.total = &totalAmount{
		amount: new(big.Int).Set(bp.total.amount),
		count:  bp.total.count,
	}
	return &pool
}

func (bp *bucketPool) SyncPool(sm protocol.StateManager) error {
	if bp.exist {
		_, err := sm.State(bp.total, protocol.LegacyKeyOption(bucketPoolAddr))
		return err
	}

	// get stashed total amount
	ser, err := protocol.UnloadAndAssertBytes(sm, stakingBucketPool)
	switch errors.Cause(err) {
	case protocol.ErrTypeAssertion:
		return errors.Wrap(err, "failed to sync bucket pool")
	case protocol.ErrNoName:
		return nil
	}

	if err := bp.total.Deserialize(ser); err != nil {
		return err
	}
	fmt.Printf("dirty bytes = %x\n", ser)
	fmt.Println("===== amount", bp.total.amount.String())
	fmt.Println("===== count", bp.total.count)
	return nil
}

func (bp *bucketPool) Commit(sr protocol.StateReader) error {
	if bp.exist {
		return nil
	}

	// at Greenland height, the staking protocol address has been created
	// so re-check the existence here
	var err error
	bp.exist, err = bp.poolExist(sr)
	if err != nil {
		return err
	}

	fmt.Printf("commit\n")
	fmt.Println(">>>>> amount", bp.total.amount.String())
	fmt.Println(">>>>>  count", bp.total.count)
	return nil
}

func (bp *bucketPool) CreditPool(sm protocol.StateManager, amount *big.Int, create bool) error {
	if bp.exist {
		if err := bp.total.SubBalance(amount); err != nil {
			return err
		}
		_, err := sm.PutState(bp.total, protocol.LegacyKeyOption(bucketPoolAddr))
		return err
	}

	if err := bp.total.SubBalance(amount); err != nil {
		return err
	}
	return bp.createOrStashPool(sm, create)
}

func (bp *bucketPool) DebitPool(sm protocol.StateManager, amount *big.Int, newBucket, create bool) error {
	if bp.exist {
		bp.total.AddBalance(amount, newBucket)
		_, err := sm.PutState(bp.total, protocol.LegacyKeyOption(bucketPoolAddr))
		return err
	}

	bp.total.AddBalance(amount, newBucket)
	return bp.createOrStashPool(sm, create)
}

func (bp *bucketPool) createOrStashPool(sm protocol.StateManager, create bool) error {
	if create {
		// at Greenland height, create the staking protocol address
		_, err := sm.PutState(bp.total, protocol.LegacyKeyOption(bucketPoolAddr))
		return err
	}
	// stash pool total amount to sm
	ser, err := bp.total.Serialize()
	if err != nil {
		return errors.Wrap(err, "failed to stash pending bucket pool")
	}
	fmt.Printf("stash\n")
	fmt.Println("amount", bp.total.amount.String())
	fmt.Println(" count", bp.total.count)
	return sm.Load(stakingBucketPool, ser)
}
