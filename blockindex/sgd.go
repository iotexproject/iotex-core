// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockindex

import (
	"context"
	"errors"
	"time"

	"github.com/iotexproject/go-pkgs/cache"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/blockindex/indexpb"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/db/batch"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

const (
	_sgdBucket     = "sg"
	_sgdToHeightNS = "hh"
)

var _sgdCurrentHeight = []byte("currentHeight")

type (
	// SGDIndexer is the interface for Sharing of Gas-fee with DApps indexer
	SGDIndexer interface {
		blockdao.BlockIndexer
		// CheckContract
		CheckContract(contract string) (address.Address, uint64, bool)
		// GetSGDIndex
		GetSGDIndex(contract string) (*indexpb.SGDIndex, error)
	}

	sgdRegistry struct {
		kvStore db.KVStore
		kvCache cache.LRUCache
	}

	sgdAct struct {
		sender     address.Address
		contract   string
		createTime time.Time
	}
)

func newSgdIndex(contract, deployer string, createTime uint64) *indexpb.SGDIndex {
	return &indexpb.SGDIndex{
		Contract:   contract,
		CreateTime: createTime,
		Deployer:   deployer,
	}
}

// NewSGDRegistry creates a new SGDIndexer
func NewSGDRegistry(kv db.KVStore, cacheSize int) SGDIndexer {
	if kv == nil {
		panic("nil kvstore")
	}
	kvCache := cache.NewDummyLruCache()
	if cacheSize > 0 {
		kvCache = cache.NewThreadSafeLruCache(cacheSize)
	}
	return &sgdRegistry{
		kvStore: kv,
		kvCache: kvCache,
	}
}

// Start starts the SGDIndexer
func (sgd *sgdRegistry) Start(ctx context.Context) error {
	return sgd.kvStore.Start(ctx)
}

// Stop stops the SGDIndexer
func (sgd *sgdRegistry) Stop(ctx context.Context) error {
	return sgd.kvStore.Stop(ctx)
}

// Height returns the current height of the SGDIndexer
func (sgd *sgdRegistry) Height() (uint64, error) {
	h, err := sgd.kvStore.Get(_sgdToHeightNS, _sgdCurrentHeight)
	if err != nil {
		// if db not exist, return 0, nil, after PutBlock, the height will be increased
		if errors.Is(err, db.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	return byteutil.BytesToUint64BigEndian(h), nil
}

// PutBlock puts a block into SGDIndexer
func (sgd *sgdRegistry) PutBlock(ctx context.Context, blk *block.Block) error {
	var (
		index *indexpb.SGDIndex
		r     *action.Receipt
		ok    bool
	)
	b := batch.NewBatch()
	receipts := getReceiptsFromBlock(blk)
	for _, selp := range blk.Actions {
		act := selp.Action()
		actHash, err := selp.Hash()
		if err != nil {
			continue
		}
		switch act := act.(type) {
		case *action.Execution:
			r, ok = receipts[actHash]
			if !ok || r.Status != uint64(iotextypes.ReceiptStatus_Success) {
				continue
			}
			sender, _ := address.FromBytes(selp.SrcPubkey().Hash())
			if len(r.ContractAddress) > 0 {
				// contract creation
				index = newSgdIndex(r.ContractAddress, sender.String(), uint64(blk.Header.Timestamp().Unix()))
			} else {
				index, err = sgd.GetSGDIndex(act.Destination())
				if err != nil {
					return err
				}
				index.CallTimes++
			}
			if err := sgd.putIndex(b, index); err != nil {
				return err
			}
		default:
		}
	}
	b.Put(_sgdToHeightNS, _sgdCurrentHeight, byteutil.Uint64ToBytesBigEndian(blk.Height()), "failed to put current height")
	return sgd.kvStore.WriteBatch(b)
}

func (sgd *sgdRegistry) putIndex(b batch.KVStoreBatch, sgdIndex *indexpb.SGDIndex) error {
	sgdIndexBytes, err := proto.Marshal(sgdIndex)
	if err != nil {
		return err
	}
	b.Put(_sgdBucket, []byte(sgdIndex.Contract), sgdIndexBytes, "failed to put sgd index")
	sgd.kvCache.Add(sgdIndex.Contract, sgdIndex)
	return nil
}

// DeleteTipBlock deletes the tip block from SGDIndexer
func (sgd *sgdRegistry) DeleteTipBlock(context.Context, *block.Block) error {
	return errors.New("cannot remove block from indexer")
}

// CheckContract checks if the contract is a SGD contract
func (sgd *sgdRegistry) CheckContract(contract string) (address.Address, uint64, bool) {
	var (
		sgdIndex *indexpb.SGDIndex
		err      error
	)
	//check if the SGDIndex is in cache
	if v, ok := sgd.kvCache.Get(contract); ok {
		sgdIndex = v.(*indexpb.SGDIndex)
	} else {
		sgdIndex, err = sgd.GetSGDIndex(contract)
		if err != nil {
			return nil, 0, false
		}
	}

	addr, err := address.FromString(sgdIndex.Receiver)
	if err != nil {
		// if the receiver is no set or invalid
		return nil, 0, true
	}
	percentage := uint64(20)
	return addr, percentage, true
}

// GetSGDIndex returns the SGDIndex of the contract
func (sgd *sgdRegistry) GetSGDIndex(contract string) (*indexpb.SGDIndex, error) {
	//if not in cache, get it from db
	buf, err := sgd.kvStore.Get(_sgdBucket, []byte(contract))
	if err != nil {
		return nil, err
	}
	sgdIndex := &indexpb.SGDIndex{}
	if err := proto.Unmarshal(buf, sgdIndex); err != nil {
		return nil, err
	}
	//put the SGDIndex into cache
	sgd.kvCache.Add(contract, sgdIndex)
	return sgdIndex, nil
}

func getReceiptsFromBlock(blk *block.Block) map[hash.Hash256]*action.Receipt {
	receipts := make(map[hash.Hash256]*action.Receipt, len(blk.Receipts))
	for _, receipt := range blk.Receipts {
		receipts[receipt.ActionHash] = receipt
	}
	return receipts
}
