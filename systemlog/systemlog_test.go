// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package systemlog

import (
	"context"
	"hash/fnv"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
)

func getTestBlocksAndExpected(t *testing.T) ([]*block.Block, []*iotextypes.BlockEvmTransfer) {
	r := require.New(t)

	contractAddr := identityset.Address(31).String()
	addr1, err := address.FromBytes(identityset.PrivateKey(28).PublicKey().Hash())
	r.NoError(err)
	addr2, err := address.FromBytes(identityset.PrivateKey(29).PublicKey().Hash())
	r.NoError(err)
	addr3, err := address.FromBytes(identityset.PrivateKey(30).PublicKey().Hash())
	r.NoError(err)

	// create testing executions
	execution1, err := testutil.SignedExecution(contractAddr, identityset.PrivateKey(28),
		1, big.NewInt(1), 0, big.NewInt(0), nil)
	r.NoError(err)
	execution2, err := testutil.SignedExecution(contractAddr, identityset.PrivateKey(29),
		2, big.NewInt(10), 0, big.NewInt(0), nil)
	require.NoError(t, err)
	hash2 := execution2.Hash()
	execution3, err := testutil.SignedExecution(contractAddr, identityset.PrivateKey(30),
		3, big.NewInt(20), 0, big.NewInt(0), nil)
	require.NoError(t, err)
	hash3 := execution3.Hash()
	failedExecution, err := testutil.SignedExecution(contractAddr, identityset.PrivateKey(30),
		4, big.NewInt(0), 0, big.NewInt(0), nil)
	require.NoError(t, err)

	startHash := hash.Hash256{}
	fnv.New32().Sum(startHash[:])
	blk1, err := block.NewTestingBuilder().
		SetHeight(1).
		SetPrevBlockHash(startHash).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(execution1, execution2).
		SignAndBuild(identityset.PrivateKey(27))
	require.NoError(t, err)

	blk2, err := block.NewTestingBuilder().
		SetHeight(2).
		SetPrevBlockHash(blk1.HashBlock()).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(execution3, failedExecution).
		SignAndBuild(identityset.PrivateKey(27))
	require.NoError(t, err)

	receipt1 := &action.Receipt{
		Status:          1,
		BlockHeight:     blk1.Height(),
		ActionHash:      execution1.Hash(),
		GasConsumed:     0,
		ContractAddress: contractAddr,
	}
	receipt2 := &action.Receipt{
		Status:          1,
		BlockHeight:     blk1.Height(),
		ActionHash:      execution2.Hash(),
		GasConsumed:     0,
		ContractAddress: contractAddr,
		Logs: []*action.Log{
			{
				Address: contractAddr,
				Topics: []hash.Hash256{
					hash.BytesToHash256(action.InContractTransfer[:]),
					hash.BytesToHash256(addr2.Bytes()),
					hash.BytesToHash256(addr3.Bytes()),
				},
				Data:        big.NewInt(3).Bytes(),
				BlockHeight: blk1.Height(),
				ActionHash:  execution2.Hash(),
			},
			{
				Address: contractAddr,
				Topics: []hash.Hash256{
					hash.BytesToHash256(action.InContractTransfer[:]),
					hash.BytesToHash256(addr2.Bytes()),
					hash.BytesToHash256(addr1.Bytes()),
				},
				Data:        big.NewInt(7).Bytes(),
				BlockHeight: blk1.Height(),
				ActionHash:  execution2.Hash(),
			},
		},
	}
	blk1.Receipts = []*action.Receipt{receipt1, receipt2}
	receipt3 := &action.Receipt{
		Status:          1,
		BlockHeight:     blk2.Height(),
		ActionHash:      execution3.Hash(),
		GasConsumed:     0,
		ContractAddress: contractAddr,
		Logs: []*action.Log{
			{
				Address: contractAddr,
				Topics: []hash.Hash256{
					hash.BytesToHash256(action.InContractTransfer[:]),
					hash.BytesToHash256(addr3.Bytes()),
					hash.BytesToHash256(addr2.Bytes()),
				},
				Data:        big.NewInt(8).Bytes(),
				BlockHeight: blk2.Height(),
				ActionHash:  execution3.Hash(),
			},
			{
				Address: contractAddr,
				Topics: []hash.Hash256{
					hash.BytesToHash256(action.InContractTransfer[:]),
					hash.BytesToHash256(addr3.Bytes()),
					hash.BytesToHash256(addr1.Bytes()),
				},
				Data:        big.NewInt(7).Bytes(),
				BlockHeight: blk2.Height(),
				ActionHash:  execution3.Hash(),
			},
			// not system log
			{
				Address: contractAddr,
				Topics: []hash.Hash256{
					hash.Hash256b(startHash[:]),
					hash.BytesToHash256(addr3.Bytes()),
					hash.BytesToHash256(addr1.Bytes()),
				},
				Data:        big.NewInt(7).Bytes(),
				BlockHeight: blk2.Height(),
				ActionHash:  execution3.Hash(),
			},
		},
	}
	failedReceipt := &action.Receipt{
		Status:          0,
		BlockHeight:     blk2.Height(),
		ActionHash:      failedExecution.Hash(),
		GasConsumed:     0,
		ContractAddress: contractAddr,
	}
	blk2.Receipts = []*action.Receipt{receipt3, failedReceipt}

	expectedEvmTransfers := []*iotextypes.BlockEvmTransfer{
		{
			BlockHeight:     blk1.Height(),
			NumEvmTransfers: 2,
			ActionEvmTransfers: []*iotextypes.ActionEvmTransfer{
				{
					ActionHash:      hash2[:],
					NumEvmTransfers: 2,
					EvmTransfers: []*iotextypes.EvmTransfer{
						{
							Amount: big.NewInt(3).Bytes(),
							From:   addr2.String(),
							To:     addr3.String(),
						},
						{
							Amount: big.NewInt(7).Bytes(),
							From:   addr2.String(),
							To:     addr1.String(),
						},
					},
				},
			},
		},
		{
			BlockHeight:     blk2.Height(),
			NumEvmTransfers: 2,
			ActionEvmTransfers: []*iotextypes.ActionEvmTransfer{
				{
					ActionHash:      hash3[:],
					NumEvmTransfers: 2,
					EvmTransfers: []*iotextypes.EvmTransfer{
						{
							Amount: big.NewInt(8).Bytes(),
							From:   addr3.String(),
							To:     addr2.String(),
						},
						{
							Amount: big.NewInt(7).Bytes(),
							From:   addr3.String(),
							To:     addr1.String(),
						},
					},
				},
			},
		},
	}

	return []*block.Block{&blk1, &blk2}, expectedEvmTransfers
}

func TestSystemLogIndexer(t *testing.T) {
	r := require.New(t)

	testIndexer := func(kv db.KVStore, t *testing.T) {
		ctx := context.Background()
		indexer, err := NewIndexer(kv)
		r.NoError(err)
		r.NoError(indexer.Start(ctx))
		tipHeight, err := indexer.Height()
		r.NoError(err)
		r.Equal(uint64(0), tipHeight)

		blocks, expectedList := getTestBlocksAndExpected(t)
		for _, blk := range blocks {
			r.NoError(indexer.PutBlock(ctx, blk))
		}
		tipHeight, err = indexer.Height()
		r.NoError(err)
		r.Equal(blocks[1].Height(), tipHeight)

		for _, expectedBlock := range expectedList {
			actualBlock, err := indexer.GetEvmTransfersByBlockHeight(expectedBlock.BlockHeight)
			r.NoError(err)
			checkBlockEvmTransfers(t, expectedBlock, actualBlock)
			for _, expectedAction := range expectedBlock.ActionEvmTransfers {
				actualAction, err := indexer.GetEvmTransfersByActionHash(hash.BytesToHash256(expectedAction.ActionHash))
				r.NoError(err)
				checkActionEvmTransfers(t, expectedAction, actualAction)
			}
		}

		r.NoError(indexer.DeleteTipBlock(blocks[1]))
		tipHeight, err = indexer.Height()
		r.NoError(err)
		r.Equal(blocks[0].Height(), tipHeight)

		_, err = indexer.GetEvmTransfersByBlockHeight(blocks[1].Height())
		r.Error(err)
		for _, act := range expectedList[1].ActionEvmTransfers {
			_, err = indexer.GetEvmTransfersByActionHash(hash.BytesToHash256(act.ActionHash))
			r.Error(err)
		}

		_, err = indexer.GetEvmTransfersByBlockHeight(blocks[0].Height())
		r.NoError(err)
		for _, act := range expectedList[0].ActionEvmTransfers {
			_, err = indexer.GetEvmTransfersByActionHash(hash.BytesToHash256(act.ActionHash))
			r.NoError(err)
		}

		r.NoError(indexer.Stop(ctx))
	}

	path := "systemlog.test"
	testPath, err := testutil.PathOfTempFile(path)
	r.NoError(err)

	cfg := config.Default.DB
	cfg.DbPath = testPath

	t.Run("Bolt DB indexer", func(t *testing.T) {
		testutil.CleanupPath(t, testPath)
		defer testutil.CleanupPath(t, testPath)
		testIndexer(db.NewBoltDB(cfg), t)
	})

	t.Run("In-memory KV indexer", func(t *testing.T) {
		testIndexer(db.NewMemKVStore(), t)
	})
}

func checkActionEvmTransfers(t *testing.T, expect *iotextypes.ActionEvmTransfer, actual *iotextypes.ActionEvmTransfer) {
	r := require.New(t)
	r.Equal(expect.ActionHash, actual.ActionHash)
	r.Equal(expect.NumEvmTransfers, actual.NumEvmTransfers)
	for i, exp := range expect.EvmTransfers {
		r.Equal(exp.From, actual.EvmTransfers[i].From)
		r.Equal(exp.To, actual.EvmTransfers[i].To)
		r.Equal(exp.Amount, actual.EvmTransfers[i].Amount)
	}
}

func checkBlockEvmTransfers(t *testing.T, expect *iotextypes.BlockEvmTransfer, actual *iotextypes.BlockEvmTransfer) {
	r := require.New(t)
	r.Equal(expect.BlockHeight, actual.BlockHeight)
	r.Equal(expect.NumEvmTransfers, actual.NumEvmTransfers)
	for i, exp := range expect.ActionEvmTransfers {
		r.Equal(exp.ActionHash, actual.ActionEvmTransfers[i].ActionHash)
		checkActionEvmTransfers(t, exp, actual.ActionEvmTransfers[i])
	}
}
