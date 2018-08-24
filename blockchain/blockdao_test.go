// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"context"
	"hash/fnv"
	"math/big"
	"math/rand"
	"testing"

	"github.com/facebookgo/clock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/blockchain/action"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestBlockDAO(t *testing.T) {
	getBlocks := func() []*Block {
		amount := uint64(50 << 22)
		// create testing transfers
		cbTsf1 := action.NewCoinBaseTransfer(big.NewInt(int64((amount))), testaddress.Addrinfo["alfa"].RawAddress)
		assert.NotNil(t, cbTsf1)
		cbTsf2 := action.NewCoinBaseTransfer(big.NewInt(int64((amount))), testaddress.Addrinfo["bravo"].RawAddress)
		assert.NotNil(t, cbTsf2)
		cbTsf3 := action.NewCoinBaseTransfer(big.NewInt(int64((amount))), testaddress.Addrinfo["charlie"].RawAddress)
		assert.NotNil(t, cbTsf3)

		// create testing votes
		vote1, err := action.NewVote(1, testaddress.Addrinfo["alfa"].RawAddress, testaddress.Addrinfo["alfa"].RawAddress)
		require.NoError(t, err)
		vote2, err := action.NewVote(1, testaddress.Addrinfo["bravo"].RawAddress, testaddress.Addrinfo["bravo"].RawAddress)
		require.NoError(t, err)
		vote3, err := action.NewVote(1, testaddress.Addrinfo["charlie"].RawAddress, testaddress.Addrinfo["charlie"].RawAddress)
		require.NoError(t, err)

		// create testing executions
		execution1, err := action.NewExecution(testaddress.Addrinfo["alfa"].RawAddress, testaddress.Addrinfo["delta"].RawAddress, 1, big.NewInt(1), 0, 0, nil)
		require.NoError(t, err)
		execution2, err := action.NewExecution(testaddress.Addrinfo["bravo"].RawAddress, testaddress.Addrinfo["delta"].RawAddress, 2, big.NewInt(0), 0, 0, nil)
		require.NoError(t, err)
		execution3, err := action.NewExecution(testaddress.Addrinfo["charlie"].RawAddress, testaddress.Addrinfo["delta"].RawAddress, 3, big.NewInt(2), 0, 0, nil)
		require.NoError(t, err)

		hash1 := hash.Hash32B{}
		fnv.New32().Sum(hash1[:])
		blk1 := NewBlock(0, 1, hash1, clock.New(), []*action.Transfer{cbTsf1}, []*action.Vote{vote1}, []*action.Execution{execution1})
		hash2 := hash.Hash32B{}
		fnv.New32().Sum(hash2[:])
		blk2 := NewBlock(0, 2, hash2, clock.New(), []*action.Transfer{cbTsf2}, []*action.Vote{vote2}, []*action.Execution{execution2})
		hash3 := hash.Hash32B{}
		fnv.New32().Sum(hash3[:])
		blk3 := NewBlock(0, 3, hash3, clock.New(), []*action.Transfer{cbTsf3}, []*action.Vote{vote3}, []*action.Execution{execution3})
		return []*Block{blk1, blk2, blk3}
	}

	blks := getBlocks()
	assert.Equal(t, 3, len(blks))

	testBlockDao := func(kvstore db.KVStore, t *testing.T) {
		ctx := context.Background()
		dao := newBlockDAO(kvstore)
		err := dao.Start(ctx)
		assert.Nil(t, err)
		defer func() {
			err = dao.Stop(ctx)
			assert.Nil(t, err)
		}()

		height, err := dao.getBlockchainHeight()
		assert.Nil(t, err)
		assert.Equal(t, uint64(0), height)

		// block put order is 0 2 1
		err = dao.putBlock(blks[0])
		assert.Nil(t, err)
		blk, err := dao.getBlock(blks[0].HashBlock())
		assert.Nil(t, err)
		assert.NotNil(t, blk)
		assert.Equal(t, blks[0].Transfers[0].Hash(), blk.Transfers[0].Hash())
		height, err = dao.getBlockchainHeight()
		assert.Nil(t, err)
		assert.Equal(t, uint64(1), height)

		err = dao.putBlock(blks[2])
		assert.Nil(t, err)
		blk, err = dao.getBlock(blks[2].HashBlock())
		assert.Nil(t, err)
		assert.NotNil(t, blk)
		assert.Equal(t, blks[2].Transfers[0].Hash(), blk.Transfers[0].Hash())
		height, err = dao.getBlockchainHeight()
		assert.Nil(t, err)
		assert.Equal(t, uint64(3), height)

		err = dao.putBlock(blks[1])
		assert.Nil(t, err)
		blk, err = dao.getBlock(blks[1].HashBlock())
		assert.Nil(t, err)
		assert.NotNil(t, blk)
		assert.Equal(t, blks[1].Transfers[0].Hash(), blk.Transfers[0].Hash())
		height, err = dao.getBlockchainHeight()
		assert.Nil(t, err)
		assert.Equal(t, uint64(3), height)

		// test getting hash by height
		hash, err := dao.getBlockHash(1)
		assert.Nil(t, err)
		assert.Equal(t, blks[0].HashBlock(), hash)

		hash, err = dao.getBlockHash(2)
		assert.Nil(t, err)
		assert.Equal(t, blks[1].HashBlock(), hash)

		hash, err = dao.getBlockHash(3)
		assert.Nil(t, err)
		assert.Equal(t, blks[2].HashBlock(), hash)

		// test getting height by hash
		height, err = dao.getBlockHeight(blks[0].HashBlock())
		assert.Nil(t, err)
		assert.Equal(t, blks[0].Height(), height)

		height, err = dao.getBlockHeight(blks[1].HashBlock())
		assert.Nil(t, err)
		assert.Equal(t, blks[1].Height(), height)

		height, err = dao.getBlockHeight(blks[2].HashBlock())
		assert.Nil(t, err)
		assert.Equal(t, blks[2].Height(), height)
	}

	testActionsDao := func(kvstore db.KVStore, t *testing.T) {
		ctx := context.Background()
		dao := newBlockDAO(kvstore)
		err := dao.Start(ctx)
		assert.Nil(t, err)
		defer func() {
			err = dao.Stop(ctx)
			assert.Nil(t, err)
		}()

		err = dao.putBlock(blks[0])
		assert.Nil(t, err)
		err = dao.putBlock(blks[1])
		assert.Nil(t, err)
		err = dao.putBlock(blks[2])

		transferHash1 := blks[0].Transfers[0].Hash()
		transferHash2 := blks[1].Transfers[0].Hash()
		transferHash3 := blks[2].Transfers[0].Hash()
		voteHash1 := blks[0].Votes[0].Hash()
		voteHash2 := blks[1].Votes[0].Hash()
		voteHash3 := blks[2].Votes[0].Hash()
		executionHash1 := blks[0].Executions[0].Hash()
		executionHash2 := blks[1].Executions[0].Hash()
		executionHash3 := blks[2].Executions[0].Hash()

		blkHash1 := blks[0].HashBlock()
		blkHash2 := blks[1].HashBlock()
		blkHash3 := blks[2].HashBlock()

		// Test getBlockHashByTransferHash
		blkHash, err := dao.getBlockHashByTransferHash(transferHash1)
		require.NoError(t, err)
		require.Equal(t, blkHash1, blkHash)
		blkHash, err = dao.getBlockHashByTransferHash(transferHash2)
		require.NoError(t, err)
		require.Equal(t, blkHash2, blkHash)
		blkHash, err = dao.getBlockHashByTransferHash(transferHash3)
		require.NoError(t, err)
		require.Equal(t, blkHash3, blkHash)

		// Test getBlockHashByVoteHash
		blkHash, err = dao.getBlockHashByVoteHash(voteHash1)
		require.NoError(t, err)
		require.Equal(t, blkHash1, blkHash)
		blkHash, err = dao.getBlockHashByVoteHash(voteHash2)
		require.NoError(t, err)
		require.Equal(t, blkHash2, blkHash)
		blkHash, err = dao.getBlockHashByVoteHash(voteHash3)
		require.NoError(t, err)
		require.Equal(t, blkHash3, blkHash)

		// Test getBlockHashByExecutionHash
		blkHash, err = dao.getBlockHashByExecutionHash(executionHash1)
		require.NoError(t, err)
		require.Equal(t, blkHash1, blkHash)
		blkHash, err = dao.getBlockHashByExecutionHash(executionHash2)
		require.NoError(t, err)
		require.Equal(t, blkHash2, blkHash)
		blkHash, err = dao.getBlockHashByExecutionHash(executionHash3)
		require.NoError(t, err)
		require.Equal(t, blkHash3, blkHash)

		// Test get transfers
		senderTransferCount, err := dao.getTransferCountBySenderAddress(testaddress.Addrinfo["alfa"].RawAddress)
		require.NoError(t, err)
		require.Equal(t, uint64(0), senderTransferCount)
		senderTransfers, err := dao.getTransfersBySenderAddress(testaddress.Addrinfo["alfa"].RawAddress)
		require.NoError(t, err)
		require.Equal(t, 0, len(senderTransfers))
		recipientTransferCount, err := dao.getTransferCountByRecipientAddress(testaddress.Addrinfo["alfa"].RawAddress)
		require.NoError(t, err)
		require.Equal(t, uint64(1), recipientTransferCount)
		recipientTransfers, err := dao.getTransfersByRecipientAddress(testaddress.Addrinfo["alfa"].RawAddress)
		require.NoError(t, err)
		require.Equal(t, 1, len(recipientTransfers))
		require.Equal(t, transferHash1, recipientTransfers[0])

		senderTransferCount, err = dao.getTransferCountBySenderAddress(testaddress.Addrinfo["bravo"].RawAddress)
		require.NoError(t, err)
		require.Equal(t, uint64(0), senderTransferCount)
		senderTransfers, err = dao.getTransfersBySenderAddress(testaddress.Addrinfo["bravo"].RawAddress)
		require.NoError(t, err)
		require.Equal(t, 0, len(senderTransfers))
		recipientTransferCount, err = dao.getTransferCountByRecipientAddress(testaddress.Addrinfo["bravo"].RawAddress)
		require.NoError(t, err)
		require.Equal(t, uint64(1), recipientTransferCount)
		recipientTransfers, err = dao.getTransfersByRecipientAddress(testaddress.Addrinfo["bravo"].RawAddress)
		require.NoError(t, err)
		require.Equal(t, 1, len(recipientTransfers))
		require.Equal(t, transferHash2, recipientTransfers[0])

		senderTransferCount, err = dao.getTransferCountBySenderAddress(testaddress.Addrinfo["charlie"].RawAddress)
		require.NoError(t, err)
		require.Equal(t, uint64(0), senderTransferCount)
		senderTransfers, err = dao.getTransfersBySenderAddress(testaddress.Addrinfo["charlie"].RawAddress)
		require.NoError(t, err)
		require.Equal(t, 0, len(senderTransfers))
		recipientTransferCount, err = dao.getTransferCountByRecipientAddress(testaddress.Addrinfo["charlie"].RawAddress)
		require.NoError(t, err)
		require.Equal(t, uint64(1), recipientTransferCount)
		recipientTransfers, err = dao.getTransfersByRecipientAddress(testaddress.Addrinfo["charlie"].RawAddress)
		require.NoError(t, err)
		require.Equal(t, 1, len(recipientTransfers))
		require.Equal(t, transferHash3, recipientTransfers[0])

		// Test get votes
		senderVoteCount, err := dao.getVoteCountBySenderAddress(testaddress.Addrinfo["alfa"].RawAddress)
		require.NoError(t, err)
		require.Equal(t, uint64(1), senderVoteCount)
		senderVotes, err := dao.getVotesBySenderAddress(testaddress.Addrinfo["alfa"].RawAddress)
		require.NoError(t, err)
		require.Equal(t, 1, len(senderVotes))
		require.Equal(t, voteHash1, senderVotes[0])
		recipientVoteCount, err := dao.getVoteCountByRecipientAddress(testaddress.Addrinfo["alfa"].RawAddress)
		require.NoError(t, err)
		require.Equal(t, uint64(1), recipientVoteCount)
		recipientVotes, err := dao.getVotesByRecipientAddress(testaddress.Addrinfo["alfa"].RawAddress)
		require.NoError(t, err)
		require.Equal(t, 1, len(recipientTransfers))
		require.Equal(t, voteHash1, recipientVotes[0])

		senderVoteCount, err = dao.getVoteCountBySenderAddress(testaddress.Addrinfo["bravo"].RawAddress)
		require.NoError(t, err)
		require.Equal(t, uint64(1), senderVoteCount)
		senderVotes, err = dao.getVotesBySenderAddress(testaddress.Addrinfo["bravo"].RawAddress)
		require.NoError(t, err)
		require.Equal(t, 1, len(senderVotes))
		require.Equal(t, voteHash2, senderVotes[0])
		recipientVoteCount, err = dao.getVoteCountByRecipientAddress(testaddress.Addrinfo["bravo"].RawAddress)
		require.NoError(t, err)
		require.Equal(t, uint64(1), recipientVoteCount)
		recipientVotes, err = dao.getVotesByRecipientAddress(testaddress.Addrinfo["bravo"].RawAddress)
		require.NoError(t, err)
		require.Equal(t, 1, len(recipientTransfers))
		require.Equal(t, voteHash2, recipientVotes[0])

		senderVoteCount, err = dao.getVoteCountBySenderAddress(testaddress.Addrinfo["charlie"].RawAddress)
		require.NoError(t, err)
		require.Equal(t, uint64(1), senderVoteCount)
		senderVotes, err = dao.getVotesBySenderAddress(testaddress.Addrinfo["charlie"].RawAddress)
		require.NoError(t, err)
		require.Equal(t, 1, len(senderVotes))
		require.Equal(t, voteHash3, senderVotes[0])
		recipientVoteCount, err = dao.getVoteCountByRecipientAddress(testaddress.Addrinfo["charlie"].RawAddress)
		require.NoError(t, err)
		require.Equal(t, uint64(1), recipientVoteCount)
		recipientVotes, err = dao.getVotesByRecipientAddress(testaddress.Addrinfo["charlie"].RawAddress)
		require.NoError(t, err)
		require.Equal(t, 1, len(recipientTransfers))
		require.Equal(t, voteHash3, recipientVotes[0])

		// Test get executions
		executorExecutionCount, err := dao.getExecutionCountByExecutorAddress(testaddress.Addrinfo["alfa"].RawAddress)
		require.NoError(t, err)
		require.Equal(t, uint64(1), executorExecutionCount)
		executorExecutions, err := dao.getExecutionsByExecutorAddress(testaddress.Addrinfo["alfa"].RawAddress)
		require.NoError(t, err)
		require.Equal(t, 1, len(executorExecutions))
		require.Equal(t, executionHash1, executorExecutions[0])
		contractExecutionCount, err := dao.getExecutionCountByContractAddress(testaddress.Addrinfo["alfa"].RawAddress)
		require.NoError(t, err)
		require.Equal(t, uint64(0), contractExecutionCount)
		contractExecutions, err := dao.getExecutionsByContractAddress(testaddress.Addrinfo["alfa"].RawAddress)
		require.NoError(t, err)
		require.Equal(t, 0, len(contractExecutions))

		executorExecutionCount, err = dao.getExecutionCountByExecutorAddress(testaddress.Addrinfo["bravo"].RawAddress)
		require.NoError(t, err)
		require.Equal(t, uint64(1), executorExecutionCount)
		executorExecutions, err = dao.getExecutionsByExecutorAddress(testaddress.Addrinfo["bravo"].RawAddress)
		require.NoError(t, err)
		require.Equal(t, 1, len(executorExecutions))
		require.Equal(t, executionHash2, executorExecutions[0])
		contractExecutionCount, err = dao.getExecutionCountByContractAddress(testaddress.Addrinfo["bravo"].RawAddress)
		require.NoError(t, err)
		require.Equal(t, uint64(0), contractExecutionCount)
		contractExecutions, err = dao.getExecutionsByContractAddress(testaddress.Addrinfo["bravo"].RawAddress)
		require.NoError(t, err)
		require.Equal(t, 0, len(contractExecutions))

		executorExecutionCount, err = dao.getExecutionCountByExecutorAddress(testaddress.Addrinfo["charlie"].RawAddress)
		require.NoError(t, err)
		require.Equal(t, uint64(1), executorExecutionCount)
		executorExecutions, err = dao.getExecutionsByExecutorAddress(testaddress.Addrinfo["charlie"].RawAddress)
		require.NoError(t, err)
		require.Equal(t, 1, len(executorExecutions))
		require.Equal(t, executionHash3, executorExecutions[0])
		contractExecutionCount, err = dao.getExecutionCountByContractAddress(testaddress.Addrinfo["charlie"].RawAddress)
		require.NoError(t, err)
		require.Equal(t, uint64(0), contractExecutionCount)
		contractExecutions, err = dao.getExecutionsByContractAddress(testaddress.Addrinfo["charlie"].RawAddress)
		require.NoError(t, err)
		require.Equal(t, 0, len(contractExecutions))

		contractExecutionCount, err = dao.getExecutionCountByContractAddress(testaddress.Addrinfo["delta"].RawAddress)
		require.NoError(t, err)
		require.Equal(t, uint64(3), contractExecutionCount)
		contractExecutions, err = dao.getExecutionsByContractAddress(testaddress.Addrinfo["delta"].RawAddress)
		require.NoError(t, err)
		require.Equal(t, 3, len(contractExecutions))
		require.Equal(t, executionHash1, contractExecutions[0])
		require.Equal(t, executionHash2, contractExecutions[1])
		require.Equal(t, executionHash3, contractExecutions[2])
	}

	t.Run("In-memory KV Store for blocks", func(t *testing.T) {
		testBlockDao(db.NewMemKVStore(), t)
	})

	path := "/tmp/test-kv-store-" + string(rand.Int())
	cfg := &config.Default.DB
	t.Run("Bolt DB for blocks", func(t *testing.T) {
		testutil.CleanupPath(t, path)
		defer testutil.CleanupPath(t, path)
		testBlockDao(db.NewBoltDB(path, cfg), t)
	})

	t.Run("In-memory KV Store for actions", func(t *testing.T) {
		testActionsDao(db.NewMemKVStore(), t)
	})

	t.Run("Bolt DB for actions", func(t *testing.T) {
		testutil.CleanupPath(t, path)
		defer testutil.CleanupPath(t, path)
		testActionsDao(db.NewBoltDB(path, cfg), t)
	})
}
