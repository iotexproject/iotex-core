// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockindex

import (
	"context"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/blockindex/indexpb"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/db/batch"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/state"
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
				"name": "ContractDisapproved",
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
	//TODO (millken): currently we fix the percentage to 30%, we can make it configurable in the future
	_sgdPercentage = uint64(30)
)

var _sgdCurrentHeight = []byte("currentHeight")

type (
	// SGDRegistry is the interface for Sharing of Gas-fee with DApps
	SGDRegistry interface {
		blockdao.BlockIndexerWithStart
		// CheckContract returns the contract's eligibility for SGD and percentage
		CheckContract(context.Context, string, uint64) (address.Address, uint64, bool, error)
		// FetchContracts returns all contracts that are eligible for SGD
		FetchContracts(context.Context, uint64) ([]*SGDIndex, error)
	}

	sgdRegistry struct {
		contract    string
		startHeight uint64
		kvStore     db.KVStore
	}
	// SGDIndex is the struct for SGDIndex
	SGDIndex struct {
		Contract address.Address
		Receiver address.Address
		Approved bool
	}
)

func sgdIndexFromPb(pb *indexpb.SGDIndex) (*SGDIndex, error) {
	contract, err := address.FromBytes(pb.Contract)
	if err != nil {
		return nil, err
	}
	receiver, err := address.FromBytes(pb.Receiver)
	if err != nil {
		return nil, err
	}
	return &SGDIndex{
		Contract: contract,
		Receiver: receiver,
		Approved: pb.Approved,
	}, nil
}

func newSgdIndex(contract, receiver []byte) *indexpb.SGDIndex {
	return &indexpb.SGDIndex{
		Contract: contract,
		Receiver: receiver,
	}
}

// NewSGDRegistry creates a new SGDIndexer
func NewSGDRegistry(contract string, startHeight uint64, kv db.KVStore) SGDRegistry {
	if kv == nil {
		panic("nil kvstore")
	}
	if contract != "" {
		if _, err := address.FromString(contract); err != nil {
			panic("invalid contract address")
		}
	}
	return &sgdRegistry{
		contract:    contract,
		startHeight: startHeight,
		kvStore:     kv,
	}
}

// Start starts the SGDIndexer
func (sgd *sgdRegistry) Start(ctx context.Context) error {
	err := sgd.kvStore.Start(ctx)
	if err != nil {
		return err
	}
	_, err = sgd.Height()
	if err != nil && errors.Is(err, db.ErrNotExist) {
		return sgd.kvStore.Put(_sgdToHeightNS, _sgdCurrentHeight, byteutil.Uint64ToBytesBigEndian(0))
	}
	return err
}

// Stop stops the SGDIndexer
func (sgd *sgdRegistry) Stop(ctx context.Context) error {
	return sgd.kvStore.Stop(ctx)
}

// Height returns the current height of the SGDIndexer
func (sgd *sgdRegistry) Height() (uint64, error) {
	h, err := sgd.kvStore.Get(_sgdToHeightNS, _sgdCurrentHeight)
	if err != nil {
		return 0, err
	}
	return byteutil.BytesToUint64BigEndian(h), nil
}

// StartHeight returns the start height of the indexer
func (sgd *sgdRegistry) StartHeight() uint64 {
	return sgd.startHeight
}

// PutBlock puts a block into SGDIndexer
func (sgd *sgdRegistry) PutBlock(ctx context.Context, blk *block.Block) error {
	tipHeight, err := sgd.Height()
	if err != nil {
		return err
	}
	expectHeight := tipHeight + 1
	if expectHeight < sgd.startHeight {
		expectHeight = sgd.startHeight
	}
	if blk.Height() < expectHeight {
		return nil
	}
	if blk.Height() > expectHeight {
		return errors.Errorf("invalid block height %d, expect %d", blk.Height(), expectHeight)
	}

	var (
		r  *action.Receipt
		ok bool
		b  = batch.NewBatch()
	)
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
	case "ContractDisapproved":
		return sgd.handleContractDisapproved(b, log)
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
		event    struct {
			ContractAddress common.Address
			Recipient       common.Address
		}
	)
	if err := _sgdABI.UnpackIntoInterface(&event, "ContractRegistered", log.Data); err != nil {
		return err
	}

	sgdIndex = newSgdIndex(event.ContractAddress.Bytes(), event.Recipient.Bytes())
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

	sgdIndex, err = sgd.getSGDIndex(event.ContractAddress.Bytes())
	if err != nil {
		return err
	}
	if sgdIndex.Approved {
		return errors.New("contract is approved")
	}
	sgdIndex.Approved = true
	return sgd.putIndex(b, sgdIndex)
}

func (sgd *sgdRegistry) handleContractDisapproved(b batch.KVStoreBatch, log *action.Log) error {
	var (
		sgdIndex *indexpb.SGDIndex
		err      error
		event    struct {
			ContractAddress common.Address
		}
	)
	if err := _sgdABI.UnpackIntoInterface(&event, "ContractDisapproved", log.Data); err != nil {
		return err
	}

	sgdIndex, err = sgd.getSGDIndex(event.ContractAddress.Bytes())
	if err != nil {
		return err
	}
	if !sgdIndex.Approved {
		return errors.New("contract is not approved")
	}
	sgdIndex.Approved = false
	return sgd.putIndex(b, sgdIndex)
}

func (sgd *sgdRegistry) handleContractRemoved(b batch.KVStoreBatch, log *action.Log) error {
	var (
		event struct {
			ContractAddress common.Address
		}
	)
	if err := _sgdABI.UnpackIntoInterface(&event, "ContractRemoved", log.Data); err != nil {
		return err
	}
	return sgd.deleteIndex(b, event.ContractAddress.Bytes())
}

func (sgd *sgdRegistry) putIndex(b batch.KVStoreBatch, sgdIndex *indexpb.SGDIndex) error {
	sgdIndexBytes, err := proto.Marshal(sgdIndex)
	if err != nil {
		return err
	}
	b.Put(_sgdBucket, sgdIndex.Contract, sgdIndexBytes, "failed to put sgd index")
	return nil
}

func (sgd *sgdRegistry) deleteIndex(b batch.KVStoreBatch, contract []byte) error {
	b.Delete(_sgdBucket, contract, "failed to delete sgd index")
	return nil
}

// DeleteTipBlock deletes the tip block from SGDIndexer
func (sgd *sgdRegistry) DeleteTipBlock(context.Context, *block.Block) error {
	return errors.New("cannot remove block from indexer")
}

// CheckContract checks if the contract is a SGD contract
func (sgd *sgdRegistry) CheckContract(ctx context.Context, contract string, height uint64) (address.Address, uint64, bool, error) {
	if err := sgd.validateQueryHeight(height); err != nil {
		return nil, 0, false, err
	}
	addr, err := address.FromString(contract)
	if err != nil {
		return nil, 0, false, err
	}
	sgdIndex, err := sgd.getSGDIndex(addr.Bytes())
	if err != nil {
		return nil, 0, false, err
	}

	addr, err = address.FromBytes(sgdIndex.Receiver)
	return addr, _sgdPercentage, sgdIndex.Approved, err
}

// getSGDIndex returns the SGDIndex of the contract
func (sgd *sgdRegistry) getSGDIndex(contract []byte) (*indexpb.SGDIndex, error) {
	buf, err := sgd.kvStore.Get(_sgdBucket, contract)
	if err != nil {
		return nil, err
	}
	sgdIndex := &indexpb.SGDIndex{}
	if err := proto.Unmarshal(buf, sgdIndex); err != nil {
		return nil, err
	}
	return sgdIndex, nil
}

// FetchContracts returns all contracts that are eligible for SGD
func (sgd *sgdRegistry) FetchContracts(ctx context.Context, height uint64) ([]*SGDIndex, error) {
	if err := sgd.validateQueryHeight(height); err != nil {
		return nil, err
	}
	_, values, err := sgd.kvStore.Filter(_sgdBucket, func(k, v []byte) bool { return true }, nil, nil)
	if err != nil {
		if errors.Cause(err) == db.ErrNotExist || errors.Cause(err) == db.ErrBucketNotExist {
			return nil, errors.Wrapf(state.ErrStateNotExist, "failed to get sgd states of ns = %x", _sgdBucket)
		}
		return nil, err
	}
	sgdIndexes := make([]*SGDIndex, 0, len(values))
	sgdIndexPb := &indexpb.SGDIndex{}
	for _, v := range values {
		if err := proto.Unmarshal(v, sgdIndexPb); err != nil {
			return nil, err
		}
		sgdIndex, err := sgdIndexFromPb(sgdIndexPb)
		if err != nil {
			return nil, err
		}
		sgdIndexes = append(sgdIndexes, sgdIndex)
	}
	return sgdIndexes, nil
}

func (sgd *sgdRegistry) validateQueryHeight(height uint64) error {
	// 0 means latest height
	if height == 0 {
		return nil
	}
	tipHeight, err := sgd.Height()
	if err != nil {
		return err
	}
	if height != tipHeight {
		return errors.Errorf("invalid height %d, expect %d", height, tipHeight)
	}
	return nil
}

func getReceiptsFromBlock(blk *block.Block) map[hash.Hash256]*action.Receipt {
	receipts := make(map[hash.Hash256]*action.Receipt, len(blk.Receipts))
	for _, receipt := range blk.Receipts {
		receipts[receipt.ActionHash] = receipt
	}
	return receipts
}
