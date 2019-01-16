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

	"github.com/iotexproject/iotex-core/pkg/util/byteutil"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestBlockDAO(t *testing.T) {
	getBlocks := func() []*block.Block {
		amount := uint64(50 << 22)
		// create testing transfers
		cbTsf1 := action.NewCoinBaseTransfer(1, big.NewInt(int64((amount))), testaddress.Addrinfo["alfa"].Bech32())
		assert.NotNil(t, cbTsf1)
		bd := &action.EnvelopeBuilder{}
		elp := bd.SetNonce(1).
			SetDestinationAddress(testaddress.Addrinfo["alfa"].Bech32()).
			SetGasLimit(genesis.ActionGasLimit).
			SetAction(cbTsf1).Build()
		scbTsf1, err := action.Sign(elp, testaddress.Addrinfo["alfa"].Bech32(), testaddress.Keyinfo["alfa"].PriKey)
		require.NoError(t, err)

		cbTsf2 := action.NewCoinBaseTransfer(1, big.NewInt(int64((amount))), testaddress.Addrinfo["bravo"].Bech32())
		assert.NotNil(t, cbTsf2)
		bd = &action.EnvelopeBuilder{}
		elp = bd.SetNonce(1).
			SetDestinationAddress(testaddress.Addrinfo["bravo"].Bech32()).
			SetGasLimit(genesis.ActionGasLimit).
			SetAction(cbTsf2).Build()
		scbTsf2, err := action.Sign(elp, testaddress.Addrinfo["bravo"].Bech32(), testaddress.Keyinfo["bravo"].PriKey)
		require.NoError(t, err)

		require.NoError(t, err)
		cbTsf3 := action.NewCoinBaseTransfer(1, big.NewInt(int64((amount))), testaddress.Addrinfo["charlie"].Bech32())
		assert.NotNil(t, cbTsf3)
		bd = &action.EnvelopeBuilder{}
		elp = bd.SetNonce(1).
			SetDestinationAddress(testaddress.Addrinfo["charlie"].Bech32()).
			SetGasLimit(genesis.ActionGasLimit).
			SetAction(cbTsf3).Build()
		scbTsf3, err := action.Sign(elp, testaddress.Addrinfo["charlie"].Bech32(), testaddress.Keyinfo["charlie"].PriKey)
		require.NoError(t, err)

		// create testing votes
		vote1, err := testutil.SignedVote(testaddress.Addrinfo["alfa"].Bech32(), testaddress.Addrinfo["alfa"].Bech32(), testaddress.Keyinfo["alfa"].PriKey, 1, 100000, big.NewInt(10))
		require.NoError(t, err)
		vote2, err := testutil.SignedVote(testaddress.Addrinfo["bravo"].Bech32(), testaddress.Addrinfo["bravo"].Bech32(), testaddress.Keyinfo["bravo"].PriKey, 1, 100000, big.NewInt(10))
		require.NoError(t, err)
		vote3, err := testutil.SignedVote(testaddress.Addrinfo["charlie"].Bech32(), testaddress.Addrinfo["charlie"].Bech32(), testaddress.Keyinfo["charlie"].PriKey, 1, 100000, big.NewInt(10))
		require.NoError(t, err)

		// create testing executions
		execution1, err := testutil.SignedExecution(testaddress.Addrinfo["alfa"].Bech32(), testaddress.Addrinfo["delta"].Bech32(), testaddress.Keyinfo["alfa"].PriKey, 1, big.NewInt(1), 0, big.NewInt(0), nil)
		require.NoError(t, err)
		execution2, err := testutil.SignedExecution(testaddress.Addrinfo["bravo"].Bech32(), testaddress.Addrinfo["delta"].Bech32(), testaddress.Keyinfo["bravo"].PriKey, 2, big.NewInt(0), 0, big.NewInt(0), nil)
		require.NoError(t, err)
		execution3, err := testutil.SignedExecution(testaddress.Addrinfo["charlie"].Bech32(), testaddress.Addrinfo["delta"].Bech32(), testaddress.Keyinfo["charlie"].PriKey, 3, big.NewInt(2), 0, big.NewInt(0), nil)
		require.NoError(t, err)

		// create testing create deposit actions
		deposit1 := action.NewCreateDeposit(
			4,
			big.NewInt(1),
			testaddress.Addrinfo["alfa"].Bech32(),
			testaddress.Addrinfo["delta"].Bech32(),
			testutil.TestGasLimit,
			big.NewInt(0),
		)
		bd = &action.EnvelopeBuilder{}
		elp = bd.SetNonce(4).
			SetDestinationAddress(testaddress.Addrinfo["delta"].Bech32()).
			SetGasLimit(testutil.TestGasLimit).
			SetAction(deposit1).Build()
		sdeposit1, err := action.Sign(elp, testaddress.Addrinfo["alfa"].Bech32(), testaddress.Keyinfo["alfa"].PriKey)
		require.NoError(t, err)

		deposit2 := action.NewCreateDeposit(
			5,
			big.NewInt(2),
			testaddress.Addrinfo["bravo"].Bech32(),
			testaddress.Addrinfo["delta"].Bech32(),
			testutil.TestGasLimit,
			big.NewInt(0),
		)
		bd = &action.EnvelopeBuilder{}
		elp = bd.SetNonce(5).
			SetDestinationAddress(testaddress.Addrinfo["delta"].Bech32()).
			SetGasLimit(testutil.TestGasLimit).
			SetAction(deposit2).Build()
		sdeposit2, err := action.Sign(elp, testaddress.Addrinfo["bravo"].Bech32(), testaddress.Keyinfo["bravo"].PriKey)
		require.NoError(t, err)

		deposit3 := action.NewCreateDeposit(
			6,
			big.NewInt(3),
			testaddress.Addrinfo["charlie"].Bech32(),
			testaddress.Addrinfo["delta"].Bech32(),
			testutil.TestGasLimit,
			big.NewInt(0),
		)
		bd = &action.EnvelopeBuilder{}
		elp = bd.SetNonce(6).
			SetDestinationAddress(testaddress.Addrinfo["delta"].Bech32()).
			SetGasLimit(testutil.TestGasLimit).
			SetAction(deposit3).Build()
		sdeposit3, err := action.Sign(elp, testaddress.Addrinfo["charlie"].Bech32(), testaddress.Keyinfo["charlie"].PriKey)
		require.NoError(t, err)

		hash1 := hash.Hash32B{}
		fnv.New32().Sum(hash1[:])
		blk1, err := block.NewTestingBuilder().
			SetHeight(1).
			SetPrevBlockHash(hash1).
			SetTimeStamp(testutil.TimestampNow()).
			AddActions(scbTsf1, vote1, execution1, sdeposit1).
			SignAndBuild(testaddress.Keyinfo["producer"].PubKey, testaddress.Keyinfo["producer"].PriKey)
		require.NoError(t, err)

		hash2 := hash.Hash32B{}
		fnv.New32().Sum(hash2[:])
		blk2, err := block.NewTestingBuilder().
			SetHeight(2).
			SetPrevBlockHash(hash2).
			SetTimeStamp(testutil.TimestampNow()).
			AddActions(scbTsf2, vote2, execution2, sdeposit2).
			SignAndBuild(testaddress.Keyinfo["producer"].PubKey, testaddress.Keyinfo["producer"].PriKey)
		require.NoError(t, err)

		hash3 := hash.Hash32B{}
		fnv.New32().Sum(hash3[:])
		blk3, err := block.NewTestingBuilder().
			SetHeight(3).
			SetPrevBlockHash(hash3).
			SetTimeStamp(testutil.TimestampNow()).
			AddActions(scbTsf3, vote3, execution3, sdeposit3).
			SignAndBuild(testaddress.Keyinfo["producer"].PubKey, testaddress.Keyinfo["producer"].PriKey)
		require.NoError(t, err)
		return []*block.Block{&blk1, &blk2, &blk3}
	}

	blks := getBlocks()
	assert.Equal(t, 3, len(blks))

	testBlockDao := func(kvstore db.KVStore, t *testing.T) {
		ctx := context.Background()
		dao := newBlockDAO(kvstore, config.Default.Explorer.Enabled)
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
		assert.Equal(t, blks[0].Actions[0].Hash(), blk.Actions[0].Hash())
		height, err = dao.getBlockchainHeight()
		assert.Nil(t, err)
		assert.Equal(t, uint64(1), height)

		err = dao.putBlock(blks[2])
		assert.Nil(t, err)
		blk, err = dao.getBlock(blks[2].HashBlock())
		assert.Nil(t, err)
		assert.NotNil(t, blk)
		assert.Equal(t, blks[2].Actions[0].Hash(), blk.Actions[0].Hash())
		height, err = dao.getBlockchainHeight()
		assert.Nil(t, err)
		assert.Equal(t, uint64(3), height)

		err = dao.putBlock(blks[1])
		assert.Nil(t, err)
		blk, err = dao.getBlock(blks[1].HashBlock())
		assert.Nil(t, err)
		assert.NotNil(t, blk)
		assert.Equal(t, blks[1].Actions[0].Hash(), blk.Actions[0].Hash())
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
		dao := newBlockDAO(kvstore, true)
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

		transfers1, votes1, executions1 := action.ClassifyActions(blks[0].Actions)
		transfers2, votes2, executions2 := action.ClassifyActions(blks[1].Actions)
		transfers3, votes3, executions3 := action.ClassifyActions(blks[2].Actions)

		transferHash1 := transfers1[0].Hash()
		transferHash2 := transfers2[0].Hash()
		transferHash3 := transfers3[0].Hash()
		voteHash1 := votes1[0].Hash()
		voteHash2 := votes2[0].Hash()
		voteHash3 := votes3[0].Hash()
		executionHash1 := executions1[0].Hash()
		executionHash2 := executions2[0].Hash()
		executionHash3 := executions3[0].Hash()
		depositHash1 := blks[0].Actions[3].Hash()
		depositHash2 := blks[1].Actions[3].Hash()
		depositHash3 := blks[2].Actions[3].Hash()

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

		// Test getBlockHashByActionHash
		blkHash, err = dao.getBlockHashByActionHash(depositHash1)
		require.NoError(t, err)
		require.Equal(t, blkHash1, blkHash)
		blkHash, err = dao.getBlockHashByActionHash(depositHash2)
		require.NoError(t, err)
		require.Equal(t, blkHash2, blkHash)
		blkHash, err = dao.getBlockHashByActionHash(depositHash3)
		require.NoError(t, err)
		require.Equal(t, blkHash3, blkHash)

		// Test get transfers
		senderTransferCount, err := dao.getTransferCountBySenderAddress(testaddress.Addrinfo["alfa"].Bech32())
		require.NoError(t, err)
		require.Equal(t, uint64(1), senderTransferCount)
		senderTransfers, err := dao.getTransfersBySenderAddress(testaddress.Addrinfo["alfa"].Bech32())
		require.NoError(t, err)
		require.Equal(t, 1, len(senderTransfers))
		recipientTransferCount, err := dao.getTransferCountByRecipientAddress(testaddress.Addrinfo["alfa"].Bech32())
		require.NoError(t, err)
		require.Equal(t, uint64(1), recipientTransferCount)
		recipientTransfers, err := dao.getTransfersByRecipientAddress(testaddress.Addrinfo["alfa"].Bech32())
		require.NoError(t, err)
		require.Equal(t, 1, len(recipientTransfers))
		require.Equal(t, transferHash1, recipientTransfers[0])

		senderTransferCount, err = dao.getTransferCountBySenderAddress(testaddress.Addrinfo["bravo"].Bech32())
		require.NoError(t, err)
		require.Equal(t, uint64(1), senderTransferCount)
		senderTransfers, err = dao.getTransfersBySenderAddress(testaddress.Addrinfo["bravo"].Bech32())
		require.NoError(t, err)
		require.Equal(t, 1, len(senderTransfers))
		recipientTransferCount, err = dao.getTransferCountByRecipientAddress(testaddress.Addrinfo["bravo"].Bech32())
		require.NoError(t, err)
		require.Equal(t, uint64(1), recipientTransferCount)
		recipientTransfers, err = dao.getTransfersByRecipientAddress(testaddress.Addrinfo["bravo"].Bech32())
		require.NoError(t, err)
		require.Equal(t, 1, len(recipientTransfers))
		require.Equal(t, transferHash2, recipientTransfers[0])

		senderTransferCount, err = dao.getTransferCountBySenderAddress(testaddress.Addrinfo["charlie"].Bech32())
		require.NoError(t, err)
		require.Equal(t, uint64(1), senderTransferCount)
		senderTransfers, err = dao.getTransfersBySenderAddress(testaddress.Addrinfo["charlie"].Bech32())
		require.NoError(t, err)
		require.Equal(t, 1, len(senderTransfers))
		recipientTransferCount, err = dao.getTransferCountByRecipientAddress(testaddress.Addrinfo["charlie"].Bech32())
		require.NoError(t, err)
		require.Equal(t, uint64(1), recipientTransferCount)
		recipientTransfers, err = dao.getTransfersByRecipientAddress(testaddress.Addrinfo["charlie"].Bech32())
		require.NoError(t, err)
		require.Equal(t, 1, len(recipientTransfers))
		require.Equal(t, transferHash3, recipientTransfers[0])

		// Test get votes
		senderVoteCount, err := dao.getVoteCountBySenderAddress(testaddress.Addrinfo["alfa"].Bech32())
		require.NoError(t, err)
		require.Equal(t, uint64(1), senderVoteCount)
		senderVotes, err := dao.getVotesBySenderAddress(testaddress.Addrinfo["alfa"].Bech32())
		require.NoError(t, err)
		require.Equal(t, 1, len(senderVotes))
		require.Equal(t, voteHash1, senderVotes[0])
		recipientVoteCount, err := dao.getVoteCountByRecipientAddress(testaddress.Addrinfo["alfa"].Bech32())
		require.NoError(t, err)
		require.Equal(t, uint64(1), recipientVoteCount)
		recipientVotes, err := dao.getVotesByRecipientAddress(testaddress.Addrinfo["alfa"].Bech32())
		require.NoError(t, err)
		require.Equal(t, 1, len(recipientTransfers))
		require.Equal(t, voteHash1, recipientVotes[0])

		senderVoteCount, err = dao.getVoteCountBySenderAddress(testaddress.Addrinfo["bravo"].Bech32())
		require.NoError(t, err)
		require.Equal(t, uint64(1), senderVoteCount)
		senderVotes, err = dao.getVotesBySenderAddress(testaddress.Addrinfo["bravo"].Bech32())
		require.NoError(t, err)
		require.Equal(t, 1, len(senderVotes))
		require.Equal(t, voteHash2, senderVotes[0])
		recipientVoteCount, err = dao.getVoteCountByRecipientAddress(testaddress.Addrinfo["bravo"].Bech32())
		require.NoError(t, err)
		require.Equal(t, uint64(1), recipientVoteCount)
		recipientVotes, err = dao.getVotesByRecipientAddress(testaddress.Addrinfo["bravo"].Bech32())
		require.NoError(t, err)
		require.Equal(t, 1, len(recipientTransfers))
		require.Equal(t, voteHash2, recipientVotes[0])

		senderVoteCount, err = dao.getVoteCountBySenderAddress(testaddress.Addrinfo["charlie"].Bech32())
		require.NoError(t, err)
		require.Equal(t, uint64(1), senderVoteCount)
		senderVotes, err = dao.getVotesBySenderAddress(testaddress.Addrinfo["charlie"].Bech32())
		require.NoError(t, err)
		require.Equal(t, 1, len(senderVotes))
		require.Equal(t, voteHash3, senderVotes[0])
		recipientVoteCount, err = dao.getVoteCountByRecipientAddress(testaddress.Addrinfo["charlie"].Bech32())
		require.NoError(t, err)
		require.Equal(t, uint64(1), recipientVoteCount)
		recipientVotes, err = dao.getVotesByRecipientAddress(testaddress.Addrinfo["charlie"].Bech32())
		require.NoError(t, err)
		require.Equal(t, 1, len(recipientTransfers))
		require.Equal(t, voteHash3, recipientVotes[0])

		// Test get executions
		executorExecutionCount, err := dao.getExecutionCountByExecutorAddress(testaddress.Addrinfo["alfa"].Bech32())
		require.NoError(t, err)
		require.Equal(t, uint64(1), executorExecutionCount)
		executorExecutions, err := dao.getExecutionsByExecutorAddress(testaddress.Addrinfo["alfa"].Bech32())
		require.NoError(t, err)
		require.Equal(t, 1, len(executorExecutions))
		require.Equal(t, executionHash1, executorExecutions[0])
		contractExecutionCount, err := dao.getExecutionCountByContractAddress(testaddress.Addrinfo["alfa"].Bech32())
		require.NoError(t, err)
		require.Equal(t, uint64(0), contractExecutionCount)
		contractExecutions, err := dao.getExecutionsByContractAddress(testaddress.Addrinfo["alfa"].Bech32())
		require.NoError(t, err)
		require.Equal(t, 0, len(contractExecutions))

		executorExecutionCount, err = dao.getExecutionCountByExecutorAddress(testaddress.Addrinfo["bravo"].Bech32())
		require.NoError(t, err)
		require.Equal(t, uint64(1), executorExecutionCount)
		executorExecutions, err = dao.getExecutionsByExecutorAddress(testaddress.Addrinfo["bravo"].Bech32())
		require.NoError(t, err)
		require.Equal(t, 1, len(executorExecutions))
		require.Equal(t, executionHash2, executorExecutions[0])
		contractExecutionCount, err = dao.getExecutionCountByContractAddress(testaddress.Addrinfo["bravo"].Bech32())
		require.NoError(t, err)
		require.Equal(t, uint64(0), contractExecutionCount)
		contractExecutions, err = dao.getExecutionsByContractAddress(testaddress.Addrinfo["bravo"].Bech32())
		require.NoError(t, err)
		require.Equal(t, 0, len(contractExecutions))

		executorExecutionCount, err = dao.getExecutionCountByExecutorAddress(testaddress.Addrinfo["charlie"].Bech32())
		require.NoError(t, err)
		require.Equal(t, uint64(1), executorExecutionCount)
		executorExecutions, err = dao.getExecutionsByExecutorAddress(testaddress.Addrinfo["charlie"].Bech32())
		require.NoError(t, err)
		require.Equal(t, 1, len(executorExecutions))
		require.Equal(t, executionHash3, executorExecutions[0])
		contractExecutionCount, err = dao.getExecutionCountByContractAddress(testaddress.Addrinfo["charlie"].Bech32())
		require.NoError(t, err)
		require.Equal(t, uint64(0), contractExecutionCount)
		contractExecutions, err = dao.getExecutionsByContractAddress(testaddress.Addrinfo["charlie"].Bech32())
		require.NoError(t, err)
		require.Equal(t, 0, len(contractExecutions))

		contractExecutionCount, err = dao.getExecutionCountByContractAddress(testaddress.Addrinfo["delta"].Bech32())
		require.NoError(t, err)
		require.Equal(t, uint64(3), contractExecutionCount)
		contractExecutions, err = dao.getExecutionsByContractAddress(testaddress.Addrinfo["delta"].Bech32())
		require.NoError(t, err)
		require.Equal(t, 3, len(contractExecutions))
		require.Equal(t, executionHash1, contractExecutions[0])
		require.Equal(t, executionHash2, contractExecutions[1])
		require.Equal(t, executionHash3, contractExecutions[2])

		// Test get actions
		senderActionCount, err := dao.getActionCountBySenderAddress(testaddress.Addrinfo["alfa"].Bech32())
		require.NoError(t, err)
		require.Equal(t, uint64(4), senderActionCount)
		senderActions, err := dao.getActionsBySenderAddress(testaddress.Addrinfo["alfa"].Bech32())
		require.NoError(t, err)
		require.Equal(t, 4, len(senderActions))
		require.Equal(t, depositHash1, senderActions[3])
		recipientActionCount, err := dao.getActionCountByRecipientAddress(testaddress.Addrinfo["alfa"].Bech32())
		require.NoError(t, err)
		require.Equal(t, uint64(2), recipientActionCount)
		recipientActions, err := dao.getActionsByRecipientAddress(testaddress.Addrinfo["alfa"].Bech32())
		require.NoError(t, err)
		require.Equal(t, 2, len(recipientActions))

		senderActionCount, err = dao.getActionCountBySenderAddress(testaddress.Addrinfo["bravo"].Bech32())
		require.NoError(t, err)
		require.Equal(t, uint64(4), senderActionCount)
		senderActions, err = dao.getActionsBySenderAddress(testaddress.Addrinfo["bravo"].Bech32())
		require.NoError(t, err)
		require.Equal(t, 4, len(senderActions))
		require.Equal(t, depositHash2, senderActions[3])
		recipientActionCount, err = dao.getActionCountByRecipientAddress(testaddress.Addrinfo["bravo"].Bech32())
		require.NoError(t, err)
		require.Equal(t, uint64(2), recipientActionCount)
		recipientActions, err = dao.getActionsByRecipientAddress(testaddress.Addrinfo["bravo"].Bech32())
		require.NoError(t, err)
		require.Equal(t, 2, len(recipientActions))

		senderActionCount, err = dao.getActionCountBySenderAddress(testaddress.Addrinfo["charlie"].Bech32())
		require.NoError(t, err)
		require.Equal(t, uint64(4), senderActionCount)
		senderActions, err = dao.getActionsBySenderAddress(testaddress.Addrinfo["charlie"].Bech32())
		require.NoError(t, err)
		require.Equal(t, 4, len(senderActions))
		require.Equal(t, depositHash3, senderActions[3])
		recipientActionCount, err = dao.getActionCountByRecipientAddress(testaddress.Addrinfo["charlie"].Bech32())
		require.NoError(t, err)
		require.Equal(t, uint64(2), recipientActionCount)
		recipientActions, err = dao.getActionsByRecipientAddress(testaddress.Addrinfo["charlie"].Bech32())
		require.NoError(t, err)
		require.Equal(t, 2, len(recipientActions))

		recipientActionCount, err = dao.getActionCountByRecipientAddress(testaddress.Addrinfo["delta"].Bech32())
		require.NoError(t, err)
		require.Equal(t, uint64(6), recipientActionCount)
		recipientActions, err = dao.getActionsByRecipientAddress(testaddress.Addrinfo["delta"].Bech32())
		require.NoError(t, err)
		require.Equal(t, 6, len(recipientActions))
		require.Equal(t, depositHash1, recipientActions[1])
		require.Equal(t, depositHash2, recipientActions[3])
		require.Equal(t, depositHash3, recipientActions[5])
	}

	testDeleteDao := func(kvstore db.KVStore, t *testing.T) {
		require := require.New(t)

		ctx := context.Background()
		dao := newBlockDAO(kvstore, true)
		err := dao.Start(ctx)
		require.NoError(err)
		defer func() {
			err = dao.Stop(ctx)
			assert.Nil(t, err)
		}()

		// Put blocks first
		err = dao.putBlock(blks[0])
		require.NoError(err)
		err = dao.putBlock(blks[1])
		require.NoError(err)
		err = dao.putBlock(blks[2])
		require.NoError(err)

		tipHeight, err := dao.getBlockchainHeight()
		require.NoError(err)
		require.Equal(uint64(3), tipHeight)
		blk, err := dao.getBlock(blks[2].HashBlock())
		require.NoError(err)
		require.NotNil(blk)
		transferCount, err := dao.getTotalTransfers()
		require.NoError(err)
		require.Equal(uint64(3), transferCount)
		voteCount, err := dao.getTotalVotes()
		require.NoError(err)
		require.Equal(uint64(3), voteCount)
		executionCount, err := dao.getTotalExecutions()
		require.NoError(err)
		require.Equal(uint64(3), executionCount)

		transfers, votes, executions := action.ClassifyActions(blks[2].Actions)
		transferHash := transfers[0].Hash()
		blkHash, err := dao.getBlockHashByTransferHash(transferHash)
		require.NoError(err)
		require.Equal(blks[2].HashBlock(), blkHash)
		voteHash := votes[0].Hash()
		blkHash, err = dao.getBlockHashByVoteHash(voteHash)
		require.NoError(err)
		require.Equal(blks[2].HashBlock(), blkHash)
		executionHash := executions[0].Hash()
		blkHash, err = dao.getBlockHashByExecutionHash(executionHash)
		require.NoError(err)
		require.Equal(blks[2].HashBlock(), blkHash)

		charlieAddr := testaddress.Addrinfo["charlie"].Bech32()
		deltaAddr := testaddress.Addrinfo["delta"].Bech32()

		transfersFromCharlie, _ := dao.getTransfersBySenderAddress(charlieAddr)
		require.Equal(1, len(transfersFromCharlie))
		transfersToCharlie, _ := dao.getTransfersByRecipientAddress(charlieAddr)
		require.Equal(1, len(transfersToCharlie))
		transferFromCharlieCount, _ := dao.getTransferCountBySenderAddress(charlieAddr)
		require.Equal(uint64(1), transferFromCharlieCount)
		transferToCharlieCount, _ := dao.getTransferCountByRecipientAddress(charlieAddr)
		require.Equal(uint64(1), transferToCharlieCount)

		votesFromCharlie, _ := dao.getVotesBySenderAddress(charlieAddr)
		require.Equal(1, len(votesFromCharlie))
		votesToCharlie, _ := dao.getVotesByRecipientAddress(charlieAddr)
		require.Equal(1, len(votesToCharlie))
		voteFromCharlieCount, _ := dao.getVoteCountBySenderAddress(charlieAddr)
		require.Equal(uint64(1), voteFromCharlieCount)
		voteToCharlieCount, _ := dao.getVoteCountByRecipientAddress(charlieAddr)
		require.Equal(uint64(1), voteToCharlieCount)

		execsFromCharlie, _ := dao.getExecutionsByExecutorAddress(charlieAddr)
		require.Equal(1, len(execsFromCharlie))
		execsToCharlie, _ := dao.getExecutionsByContractAddress(charlieAddr)
		require.Equal(0, len(execsToCharlie))
		execFromCharlieCount, _ := dao.getExecutionCountByExecutorAddress(charlieAddr)
		require.Equal(uint64(1), execFromCharlieCount)
		execToCharlieCount, _ := dao.getExecutionCountByContractAddress(charlieAddr)
		require.Equal(uint64(0), execToCharlieCount)

		execsFromDelta, _ := dao.getExecutionsByExecutorAddress(deltaAddr)
		require.Equal(0, len(execsFromDelta))
		execsToDelta, _ := dao.getExecutionsByContractAddress(deltaAddr)
		require.Equal(3, len(execsToDelta))
		execFromDeltaCount, _ := dao.getExecutionCountByExecutorAddress(deltaAddr)
		require.Equal(uint64(0), execFromDeltaCount)
		execToDeltaCount, _ := dao.getExecutionCountByContractAddress(deltaAddr)
		require.Equal(uint64(3), execToDeltaCount)

		_, err = dao.getBlockchainHeight()
		require.NoError(err)

		// Delete tip block
		err = dao.deleteTipBlock()
		require.NoError(err)
		tipHeight, err = dao.getBlockchainHeight()
		require.NoError(err)
		require.Equal(uint64(2), tipHeight)
		blk, err = dao.getBlock(blks[2].HashBlock())
		require.Equal(db.ErrNotExist, errors.Cause(err))
		require.Nil(blk)
		transferCount, err = dao.getTotalTransfers()
		require.NoError(err)
		require.Equal(uint64(2), transferCount)
		voteCount, err = dao.getTotalVotes()
		require.NoError(err)
		require.Equal(uint64(2), voteCount)
		executionCount, err = dao.getTotalExecutions()
		require.NoError(err)
		require.Equal(uint64(2), executionCount)

		blkHash, err = dao.getBlockHashByTransferHash(transferHash)
		require.Equal(db.ErrNotExist, errors.Cause(err))
		require.Equal(hash.ZeroHash32B, blkHash)
		blkHash, err = dao.getBlockHashByVoteHash(voteHash)
		require.Equal(db.ErrNotExist, errors.Cause(err))
		require.Equal(hash.ZeroHash32B, blkHash)
		blkHash, err = dao.getBlockHashByExecutionHash(executionHash)
		require.Equal(db.ErrNotExist, errors.Cause(err))
		require.Equal(hash.ZeroHash32B, blkHash)

		transfersFromCharlie, _ = dao.getTransfersBySenderAddress(charlieAddr)
		require.Equal(0, len(transfersFromCharlie))
		transfersToCharlie, _ = dao.getTransfersByRecipientAddress(charlieAddr)
		require.Equal(0, len(transfersToCharlie))
		transferFromCharlieCount, _ = dao.getTransferCountBySenderAddress(charlieAddr)
		require.Equal(uint64(0), transferFromCharlieCount)
		transferToCharlieCount, _ = dao.getTransferCountByRecipientAddress(charlieAddr)
		require.Equal(uint64(0), transferToCharlieCount)

		votesFromCharlie, _ = dao.getVotesBySenderAddress(charlieAddr)
		require.Equal(0, len(votesFromCharlie))
		votesToCharlie, _ = dao.getVotesByRecipientAddress(charlieAddr)
		require.Equal(0, len(votesToCharlie))
		voteFromCharlieCount, _ = dao.getVoteCountBySenderAddress(charlieAddr)
		require.Equal(uint64(0), voteFromCharlieCount)
		voteToCharlieCount, _ = dao.getVoteCountByRecipientAddress(charlieAddr)
		require.Equal(uint64(0), voteToCharlieCount)

		execsFromCharlie, _ = dao.getExecutionsByExecutorAddress(charlieAddr)
		require.Equal(0, len(execsFromCharlie))
		execsToCharlie, _ = dao.getExecutionsByContractAddress(charlieAddr)
		require.Equal(0, len(execsToCharlie))
		execFromCharlieCount, _ = dao.getExecutionCountByExecutorAddress(charlieAddr)
		require.Equal(uint64(0), execFromCharlieCount)
		execToCharlieCount, _ = dao.getExecutionCountByContractAddress(charlieAddr)
		require.Equal(uint64(0), execToCharlieCount)

		execsFromDelta, _ = dao.getExecutionsByExecutorAddress(deltaAddr)
		require.Equal(0, len(execsFromDelta))
		execsToDelta, _ = dao.getExecutionsByContractAddress(deltaAddr)
		require.Equal(2, len(execsToDelta))
		execFromDeltaCount, _ = dao.getExecutionCountByExecutorAddress(deltaAddr)
		require.Equal(uint64(0), execFromDeltaCount)
		execToDeltaCount, _ = dao.getExecutionCountByContractAddress(deltaAddr)
		require.Equal(uint64(2), execToDeltaCount)
	}

	t.Run("In-memory KV Store for blocks", func(t *testing.T) {
		testBlockDao(db.NewMemKVStore(), t)
	})

	path := "/tmp/test-kv-store-" + string(rand.Int())
	cfg := config.Default.DB
	cfg.DbPath = path
	t.Run("Bolt DB for blocks", func(t *testing.T) {
		testutil.CleanupPath(t, path)
		defer testutil.CleanupPath(t, path)
		testBlockDao(db.NewOnDiskDB(cfg), t)
	})

	t.Run("In-memory KV Store for actions", func(t *testing.T) {
		testActionsDao(db.NewMemKVStore(), t)
	})

	t.Run("Bolt DB for actions", func(t *testing.T) {
		testutil.CleanupPath(t, path)
		defer testutil.CleanupPath(t, path)
		testActionsDao(db.NewOnDiskDB(cfg), t)
	})

	t.Run("In-memory KV Store deletions", func(t *testing.T) {
		testDeleteDao(db.NewMemKVStore(), t)
	})

	t.Run("Bolt DB deletions", func(t *testing.T) {
		testutil.CleanupPath(t, path)
		defer testutil.CleanupPath(t, path)
		testDeleteDao(db.NewOnDiskDB(cfg), t)
	})
}

func TestBlockDao_putReceipts(t *testing.T) {
	blkDao := newBlockDAO(db.NewMemKVStore(), true)
	receipts := []*action.Receipt{
		{
			Hash:            byteutil.BytesTo32B(hash.Hash256b([]byte("1"))),
			ReturnValue:     []byte("1"),
			Status:          1,
			GasConsumed:     1,
			ContractAddress: "1",
			Logs:            []*action.Log{},
		},
		{
			Hash:            byteutil.BytesTo32B(hash.Hash256b([]byte("1"))),
			ReturnValue:     []byte("2"),
			Status:          2,
			GasConsumed:     2,
			ContractAddress: "2",
			Logs:            []*action.Log{},
		},
	}
	require.NoError(t, blkDao.putReceipts(1, receipts))
	for _, receipt := range receipts {
		r, err := blkDao.getReceiptByActionHash(receipt.Hash)
		require.NoError(t, err)
		assert.Equal(t, receipt.Hash, r.Hash)
	}
}
