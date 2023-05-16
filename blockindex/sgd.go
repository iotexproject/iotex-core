// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockindex

import (
	"context"
	"errors"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
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

var (
	_sgdABI abi.ABI
)

const (
	_sgdContractInterfaceABI = `[
			{
				"anonymous": false,
				"inputs": [
					{
						"indexed": false,
						"internalType": "address",
						"name": "previousAdmin",
						"type": "address"
					},
					{
						"indexed": false,
						"internalType": "address",
						"name": "newAdmin",
						"type": "address"
					}
				],
				"name": "AdminChanged",
				"type": "event"
			},
			{
				"anonymous": false,
				"inputs": [
					{
						"indexed": true,
						"internalType": "address",
						"name": "beacon",
						"type": "address"
					}
				],
				"name": "BeaconUpgraded",
				"type": "event"
			},
			{
				"anonymous": false,
				"inputs": [
					{
						"indexed": false,
						"internalType": "address",
						"name": "contractAddress",
						"type": "address"
					}
				],
				"name": "ContractApproved",
				"type": "event"
			},
			{
				"anonymous": false,
				"inputs": [
					{
						"indexed": false,
						"internalType": "address",
						"name": "contractAddress",
						"type": "address"
					},
					{
						"indexed": false,
						"internalType": "address",
						"name": "recipient",
						"type": "address"
					}
				],
				"name": "ContractRegistered",
				"type": "event"
			},
			{
				"anonymous": false,
				"inputs": [
					{
						"indexed": false,
						"internalType": "address",
						"name": "contractAddress",
						"type": "address"
					}
				],
				"name": "ContractRemoved",
				"type": "event"
			},
			{
				"anonymous": false,
				"inputs": [
					{
						"indexed": false,
						"internalType": "address",
						"name": "contractAddress",
						"type": "address"
					}
				],
				"name": "ContractUnapproved",
				"type": "event"
			},
			{
				"anonymous": false,
				"inputs": [
					{
						"indexed": false,
						"internalType": "uint8",
						"name": "version",
						"type": "uint8"
					}
				],
				"name": "Initialized",
				"type": "event"
			},
			{
				"anonymous": false,
				"inputs": [
					{
						"indexed": true,
						"internalType": "address",
						"name": "previousOwner",
						"type": "address"
					},
					{
						"indexed": true,
						"internalType": "address",
						"name": "newOwner",
						"type": "address"
					}
				],
				"name": "OwnershipTransferred",
				"type": "event"
			},
			{
				"anonymous": false,
				"inputs": [
					{
						"indexed": true,
						"internalType": "address",
						"name": "implementation",
						"type": "address"
					}
				],
				"name": "Upgraded",
				"type": "event"
			}
]`
)

func init() {
	var err error
	_sgdABI, err = abi.JSON(strings.NewReader(_sgdContractInterfaceABI))
	if err != nil {
		panic(err)
	}
}

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
		CheckContract(context.Context, string) (address.Address, uint64, bool, error)
	}

	sgdRegistry struct {
		contract string
		kvStore  db.KVStore
		kvCache  cache.LRUCache
	}
)

func newSgdIndex(contract, receiver string) *indexpb.SGDIndex {
	return &indexpb.SGDIndex{
		Contract: contract,
		Receiver: receiver,
	}
}

// NewSGDRegistry creates a new SGDIndexer
func NewSGDRegistry(contract string, kv db.KVStore, cacheSize int) SGDIndexer {
	if kv == nil {
		panic("nil kvstore")
	}
	kvCache := cache.NewDummyLruCache()
	if cacheSize > 0 {
		kvCache = cache.NewThreadSafeLruCache(cacheSize)
	}
	return &sgdRegistry{
		contract: contract,
		kvStore:  kv,
		kvCache:  kvCache,
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
		r  *action.Receipt
		ok bool
	)
	b := batch.NewBatch()
	receipts := getReceiptsFromBlock(blk)
	for _, selp := range blk.Actions {
		actHash, err := selp.Hash()
		if err != nil {
			continue
		}
		r, ok = receipts[actHash]
		if !ok || r.Status != uint64(iotextypes.ReceiptStatus_Success) {
			continue
		}
		for _, log := range r.Logs() {
			if log.Address != sgd.contract {
				continue
			}
			if err := sgd.handleEvent(b, log); err != nil {
				return err
			}
		}
	}
	b.Put(_sgdToHeightNS, _sgdCurrentHeight, byteutil.Uint64ToBytesBigEndian(blk.Height()), "failed to put current height")
	return sgd.kvStore.WriteBatch(b)
}

func (sgd *sgdRegistry) handleEvent(b batch.KVStoreBatch, log *action.Log) error {
	abiEvent, err := _sgdABI.EventByID(common.Hash(log.Topics[0]))
	if err != nil {
		return err
	}
	switch abiEvent.Name {
	case "ContractRegistered":
		return sgd.handleContractRegistered(b, log)
	case "ContractApproved":
		return sgd.handleContractApproved(b, log)
	case "ContractUnapproved":
		return sgd.handleContractUnapproved(b, log)
	case "ContractRemoved":
		return sgd.handleContractRemoved(b, log)
	default:
		//skip other events
	}
	return nil
}

func (sgd *sgdRegistry) handleContractRegistered(b batch.KVStoreBatch, log *action.Log) error {
	var (
		sgdIndex *indexpb.SGDIndex
		err      error
		event    struct {
			ContractAddress common.Address
			Recipient       common.Address
		}
	)
	if err := _sgdABI.UnpackIntoInterface(&event, "ContractRegistered", log.Data); err != nil {
		return err
	}
	contract, err := address.FromBytes(event.ContractAddress.Bytes())
	if err != nil {
		return err
	}
	recipient, err := address.FromBytes(event.Recipient.Bytes())
	if err != nil {
		return err
	}
	sgdIndex = newSgdIndex(contract.String(), recipient.String())
	return sgd.putIndex(b, sgdIndex)
}

func (sgd *sgdRegistry) handleContractApproved(b batch.KVStoreBatch, log *action.Log) error {
	var (
		sgdIndex *indexpb.SGDIndex
		err      error
		event    struct {
			ContractAddress common.Address
		}
	)
	if err := _sgdABI.UnpackIntoInterface(&event, "ContractApproved", log.Data); err != nil {
		return err
	}
	contract, err := address.FromBytes(event.ContractAddress.Bytes())
	if err != nil {
		return err
	}
	sgdIndex, err = sgd.GetSGDIndex(contract.String())
	if err != nil {
		return err
	}
	sgdIndex.Approved = true
	return sgd.putIndex(b, sgdIndex)
}

func (sgd *sgdRegistry) handleContractUnapproved(b batch.KVStoreBatch, log *action.Log) error {
	var (
		sgdIndex *indexpb.SGDIndex
		err      error
		event    struct {
			ContractAddress common.Address
		}
	)
	if err := _sgdABI.UnpackIntoInterface(&event, "ContractUnapproved", log.Data); err != nil {
		return err
	}
	contract, err := address.FromBytes(event.ContractAddress.Bytes())
	if err != nil {
		return err
	}
	sgdIndex, err = sgd.GetSGDIndex(contract.String())
	if err != nil {
		return err
	}
	sgdIndex.Approved = false
	return sgd.putIndex(b, sgdIndex)
}

func (sgd *sgdRegistry) handleContractRemoved(b batch.KVStoreBatch, log *action.Log) error {
	var (
		err   error
		event struct {
			ContractAddress common.Address
		}
	)
	if err := _sgdABI.UnpackIntoInterface(&event, "ContractRemoved", log.Data); err != nil {
		return err
	}
	contract, err := address.FromBytes(event.ContractAddress.Bytes())
	if err != nil {
		return err
	}
	return sgd.deleteIndex(b, contract.String())
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

func (sgd *sgdRegistry) deleteIndex(b batch.KVStoreBatch, contract string) error {
	b.Delete(_sgdBucket, []byte(contract), "failed to delete sgd index")
	sgd.kvCache.Remove(contract)
	return nil
}

// DeleteTipBlock deletes the tip block from SGDIndexer
func (sgd *sgdRegistry) DeleteTipBlock(context.Context, *block.Block) error {
	return errors.New("cannot remove block from indexer")
}

// CheckContract checks if the contract is a SGD contract
func (sgd *sgdRegistry) CheckContract(ctx context.Context, contract string) (address.Address, uint64, bool, error) {
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
			return nil, 0, false, err
		}
	}

	addr, err := address.FromString(sgdIndex.Receiver)
	if err != nil {
		// if the receiver is no set or invalid
		return nil, 0, true, nil
	}
	percentage := uint64(20)
	return addr, percentage, sgdIndex.Approved, nil
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
