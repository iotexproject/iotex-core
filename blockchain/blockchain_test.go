// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/facebookgo/clock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/blockchain/action"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/pkg/hash"
	_hash "github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/state"
	ta "github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/testutil"
)

const (
	testDBPath   = "db.test"
	testTriePath = "trie.test"
)

func addTestingTsfBlocks(bc Blockchain) error {
	// Add block 1
	// test --> A, B, C, D, E, F
	tsf1, _ := action.NewTransfer(1, big.NewInt(20), ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["alfa"].RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	tsf1, _ = tsf1.Sign(ta.Addrinfo["producer"])
	tsf2, _ := action.NewTransfer(2, big.NewInt(30), ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["bravo"].RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	tsf2, _ = tsf2.Sign(ta.Addrinfo["producer"])
	tsf3, _ := action.NewTransfer(3, big.NewInt(50), ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["charlie"].RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	tsf3, _ = tsf3.Sign(ta.Addrinfo["producer"])
	tsf4, _ := action.NewTransfer(4, big.NewInt(70), ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["delta"].RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	tsf4, _ = tsf4.Sign(ta.Addrinfo["producer"])
	tsf5, _ := action.NewTransfer(5, big.NewInt(110), ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["echo"].RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	tsf5, _ = tsf5.Sign(ta.Addrinfo["producer"])
	tsf6, _ := action.NewTransfer(6, big.NewInt(50<<20), ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["foxtrot"].RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	tsf6, _ = tsf6.Sign(ta.Addrinfo["producer"])

	blk, err := bc.MintNewBlock([]*action.Transfer{tsf1, tsf2, tsf3, tsf4, tsf5, tsf6}, nil, nil, ta.Addrinfo["producer"], "")
	if err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	// Add block 2
	// Charlie --> A, B, D, E, test
	tsf1, _ = action.NewTransfer(1, big.NewInt(1), ta.Addrinfo["charlie"].RawAddress, ta.Addrinfo["alfa"].RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	tsf1, _ = tsf1.Sign(ta.Addrinfo["charlie"])
	tsf2, _ = action.NewTransfer(2, big.NewInt(1), ta.Addrinfo["charlie"].RawAddress, ta.Addrinfo["bravo"].RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	tsf2, _ = tsf2.Sign(ta.Addrinfo["charlie"])
	tsf3, _ = action.NewTransfer(3, big.NewInt(1), ta.Addrinfo["charlie"].RawAddress, ta.Addrinfo["delta"].RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	tsf3, _ = tsf3.Sign(ta.Addrinfo["charlie"])
	tsf4, _ = action.NewTransfer(4, big.NewInt(1), ta.Addrinfo["charlie"].RawAddress, ta.Addrinfo["echo"].RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	tsf4, _ = tsf4.Sign(ta.Addrinfo["charlie"])
	tsf5, _ = action.NewTransfer(5, big.NewInt(1), ta.Addrinfo["charlie"].RawAddress, ta.Addrinfo["producer"].RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	tsf5, _ = tsf5.Sign(ta.Addrinfo["charlie"])
	blk, err = bc.MintNewBlock([]*action.Transfer{tsf1, tsf2, tsf3, tsf4, tsf5}, nil, nil, ta.Addrinfo["producer"], "")
	if err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	// Add block 3
	// Delta --> B, E, F, test
	tsf1, _ = action.NewTransfer(1, big.NewInt(1), ta.Addrinfo["delta"].RawAddress, ta.Addrinfo["bravo"].RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	tsf1, _ = tsf1.Sign(ta.Addrinfo["delta"])
	tsf2, _ = action.NewTransfer(2, big.NewInt(1), ta.Addrinfo["delta"].RawAddress, ta.Addrinfo["echo"].RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	tsf2, _ = tsf2.Sign(ta.Addrinfo["delta"])
	tsf3, _ = action.NewTransfer(3, big.NewInt(1), ta.Addrinfo["delta"].RawAddress, ta.Addrinfo["foxtrot"].RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	tsf3, _ = tsf3.Sign(ta.Addrinfo["delta"])
	tsf4, _ = action.NewTransfer(4, big.NewInt(1), ta.Addrinfo["delta"].RawAddress, ta.Addrinfo["producer"].RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	tsf4, _ = tsf4.Sign(ta.Addrinfo["delta"])
	blk, err = bc.MintNewBlock([]*action.Transfer{tsf1, tsf2, tsf3, tsf4}, nil, nil, ta.Addrinfo["producer"], "")
	if err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	// Add block 4
	// Delta --> A, B, C, D, F, test
	tsf1, _ = action.NewTransfer(1, big.NewInt(2), ta.Addrinfo["echo"].RawAddress, ta.Addrinfo["alfa"].RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	tsf1, _ = tsf1.Sign(ta.Addrinfo["echo"])
	tsf2, _ = action.NewTransfer(2, big.NewInt(2), ta.Addrinfo["echo"].RawAddress, ta.Addrinfo["bravo"].RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	tsf2, _ = tsf2.Sign(ta.Addrinfo["echo"])
	tsf3, _ = action.NewTransfer(3, big.NewInt(2), ta.Addrinfo["echo"].RawAddress, ta.Addrinfo["charlie"].RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	tsf3, _ = tsf3.Sign(ta.Addrinfo["echo"])
	tsf4, _ = action.NewTransfer(4, big.NewInt(2), ta.Addrinfo["echo"].RawAddress, ta.Addrinfo["delta"].RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	tsf4, _ = tsf4.Sign(ta.Addrinfo["echo"])
	tsf5, _ = action.NewTransfer(5, big.NewInt(2), ta.Addrinfo["echo"].RawAddress, ta.Addrinfo["foxtrot"].RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	tsf5, _ = tsf5.Sign(ta.Addrinfo["echo"])
	tsf6, _ = action.NewTransfer(6, big.NewInt(2), ta.Addrinfo["echo"].RawAddress, ta.Addrinfo["producer"].RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	tsf6, _ = tsf6.Sign(ta.Addrinfo["echo"])
	vote1, _ := action.NewVote(6, ta.Addrinfo["charlie"].RawAddress, ta.Addrinfo["alfa"].RawAddress, uint64(100000), big.NewInt(10))
	vote2, _ := action.NewVote(1, ta.Addrinfo["alfa"].RawAddress, ta.Addrinfo["charlie"].RawAddress, uint64(100000), big.NewInt(10))
	vote1, err = vote1.Sign(ta.Addrinfo["charlie"])
	if err != nil {
		return err
	}

	vote2, err = vote2.Sign(ta.Addrinfo["alfa"])
	if err != nil {
		return err
	}

	blk, err = bc.MintNewBlock([]*action.Transfer{tsf1, tsf2, tsf3, tsf4, tsf5, tsf6}, []*action.Vote{vote1, vote2}, nil, ta.Addrinfo["producer"], "")
	if err != nil {
		return err
	}
	return bc.CommitBlock(blk)
}

func TestCreateBlockchain(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()

	cfg := config.Default
	// disable account-based testing
	cfg.Chain.TrieDBPath = ""
	// Disable block reward to make bookkeeping easier
	Gen.BlockReward = uint64(0)

	// create chain
	bc := NewBlockchain(&cfg, InMemDaoOption())
	require.NoError(t, bc.Start(ctx))
	assert.NotNil(bc)
	height := bc.TipHeight()
	assert.Equal(0, int(height))
	fmt.Printf("Create blockchain pass, height = %d\n", height)
	defer func() {
		err := bc.Stop(ctx)
		assert.NoError(err)
	}()

	// verify Genesis block
	genesis, _ := bc.GetBlockByHeight(0)
	assert.NotNil(genesis)
	// serialize
	data, err := genesis.Serialize()
	assert.Nil(err)

	assert.Equal(23, len(genesis.Transfers))
	assert.Equal(21, len(genesis.Votes))

	fmt.Printf("Block size match pass\n")
	fmt.Printf("Marshaling Block pass\n")

	// deserialize
	deserialize := Block{}
	err = deserialize.Deserialize(data)
	assert.Nil(err)
	fmt.Printf("Unmarshaling Block pass\n")

	hash := genesis.HashBlock()
	assert.Equal(hash, deserialize.HashBlock())
	fmt.Printf("Serialize/Deserialize Block hash = %x match\n", hash)

	hash = genesis.TxRoot()
	assert.Equal(hash, deserialize.TxRoot())
	fmt.Printf("Serialize/Deserialize Block merkle = %x match\n", hash)

	// add 4 sample blocks
	assert.Nil(addTestingTsfBlocks(bc))
	height = bc.TipHeight()
	assert.Equal(4, int(height))
}

func TestLoadBlockchainfromDB(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)
	defer testutil.CleanupPath(t, testDBPath)

	// Disable block reward to make bookkeeping easier
	Gen.BlockReward = uint64(0)

	cfg := config.Default
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Explorer.Enabled = true

	sf, err := state.NewFactory(&cfg, state.DefaultTrieOption())
	require.Nil(err)
	require.NoError(sf.Start(context.Background()))
	_, err = sf.LoadOrCreateState(ta.Addrinfo["producer"].RawAddress, Gen.TotalSupply)
	require.NoError(err)

	// Create a blockchain from scratch
	bc := NewBlockchain(&cfg, PrecreatedStateFactoryOption(sf), BoltDBDaoOption())
	require.NoError(bc.Start(ctx))
	require.NotNil(bc)
	height := bc.TipHeight()
	fmt.Printf("Open blockchain pass, height = %d\n", height)
	require.Nil(addTestingTsfBlocks(bc))
	err = bc.Stop(ctx)
	require.NoError(err)

	// Load a blockchain from DB
	bc = NewBlockchain(&cfg, PrecreatedStateFactoryOption(sf), BoltDBDaoOption())
	require.NoError(bc.Start(ctx))
	defer func() {
		err := bc.Stop(ctx)
		require.NoError(err)
	}()
	require.NotNil(bc)

	// check hash<-->height mapping
	hash, err := bc.GetHashByHeight(0)
	require.Nil(err)
	height, err = bc.GetHeightByHash(hash)
	require.Nil(err)
	require.Equal(uint64(0), height)
	blk, err := bc.GetBlockByHash(hash)
	require.Nil(err)
	require.Equal(hash, blk.HashBlock())
	fmt.Printf("Genesis hash = %x\n", hash)

	hash1, err := bc.GetHashByHeight(1)
	require.Nil(err)
	height, err = bc.GetHeightByHash(hash1)
	require.Nil(err)
	require.Equal(uint64(1), height)
	blk, err = bc.GetBlockByHash(hash1)
	require.Nil(err)
	require.Equal(hash1, blk.HashBlock())
	fmt.Printf("block 1 hash = %x\n", hash1)

	hash2, err := bc.GetHashByHeight(2)
	require.Nil(err)
	height, err = bc.GetHeightByHash(hash2)
	require.Nil(err)
	require.Equal(uint64(2), height)
	blk, err = bc.GetBlockByHash(hash2)
	require.Nil(err)
	require.Equal(hash2, blk.HashBlock())
	fmt.Printf("block 2 hash = %x\n", hash2)

	hash3, err := bc.GetHashByHeight(3)
	require.Nil(err)
	height, err = bc.GetHeightByHash(hash3)
	require.Nil(err)
	require.Equal(uint64(3), height)
	blk, err = bc.GetBlockByHash(hash3)
	require.Nil(err)
	require.Equal(hash3, blk.HashBlock())
	fmt.Printf("block 3 hash = %x\n", hash3)

	hash4, err := bc.GetHashByHeight(4)
	require.Nil(err)
	height, err = bc.GetHeightByHash(hash4)
	require.Nil(err)
	require.Equal(uint64(4), height)
	blk, err = bc.GetBlockByHash(hash4)
	require.Nil(err)
	require.Equal(hash4, blk.HashBlock())
	fmt.Printf("block 4 hash = %x\n", hash4)

	empblk, err := bc.GetBlockByHash(_hash.ZeroHash32B)
	require.Nil(empblk)
	require.NotNil(err.Error())

	blk, err = bc.GetBlockByHeight(60000)
	require.Nil(blk)
	require.NotNil(err)

	// add wrong blocks
	h := bc.TipHeight()
	hash = bc.TipHash()
	blk, err = bc.GetBlockByHeight(h)
	require.Nil(err)
	require.Equal(hash, blk.HashBlock())
	fmt.Printf("Current tip = %d hash = %x\n", h, hash)

	// add block with wrong height
	cbTsf := action.NewCoinBaseTransfer(big.NewInt(50), ta.Addrinfo["bravo"].RawAddress)
	require.NotNil(cbTsf)
	blk = NewBlock(0, h+2, hash, clock.New(), []*action.Transfer{cbTsf}, nil, nil)
	err = bc.ValidateBlock(blk)
	require.NotNil(err)
	fmt.Printf("Cannot validate block %d: %v\n", blk.Height(), err)

	// add block with zero prev hash
	cbTsf2 := action.NewCoinBaseTransfer(big.NewInt(50), ta.Addrinfo["bravo"].RawAddress)
	require.NotNil(cbTsf2)
	blk = NewBlock(0, h+1, _hash.ZeroHash32B, clock.New(), []*action.Transfer{cbTsf2}, nil, nil)
	err = bc.ValidateBlock(blk)
	require.NotNil(err)
	fmt.Printf("Cannot validate block %d: %v\n", blk.Height(), err)

	// cannot add existing block again
	blk, err = bc.GetBlockByHeight(3)
	require.NotNil(blk)
	require.Nil(err)
	err = bc.(*blockchain).commitBlock(blk)
	require.NotNil(err)
	fmt.Printf("Cannot add block 3 again: %v\n", err)

	// check all Tx from block 4
	blk, err = bc.GetBlockByHeight(4)
	require.Nil(err)
	require.Equal(hash4, blk.HashBlock())
	for _, transfer := range blk.Transfers {
		transferHash := transfer.Hash()
		hash, err := bc.GetBlockHashByTransferHash(transferHash)
		require.Nil(err)
		require.Equal(hash, hash4)
		transfer1, err := bc.GetTransferByTransferHash(transferHash)
		require.Nil(err)
		require.Equal(transfer1.Hash(), transferHash)
	}

	for _, vote := range blk.Votes {
		voteHash := vote.Hash()
		hash, err := bc.GetBlockHashByVoteHash(voteHash)
		require.Nil(err)
		require.Equal(hash, hash4)
		vote1, err := bc.GetVoteByVoteHash(voteHash)
		require.Nil(err)
		require.Equal(vote1.Hash(), voteHash)
	}

	fromTransfers, err := bc.GetTransfersFromAddress(ta.Addrinfo["charlie"].RawAddress)
	require.Nil(err)
	require.Equal(len(fromTransfers), 5)

	toTransfers, err := bc.GetTransfersToAddress(ta.Addrinfo["charlie"].RawAddress)
	require.Nil(err)
	require.Equal(len(toTransfers), 2)

	fromVotes, err := bc.GetVotesFromAddress(ta.Addrinfo["charlie"].RawAddress)
	require.Nil(err)
	require.Equal(len(fromVotes), 1)

	fromVotes, err = bc.GetVotesFromAddress(ta.Addrinfo["alfa"].RawAddress)
	require.Nil(err)
	require.Equal(len(fromVotes), 1)

	toVotes, err := bc.GetVotesToAddress(ta.Addrinfo["charlie"].RawAddress)
	require.Nil(err)
	require.Equal(len(toVotes), 1)

	toVotes, err = bc.GetVotesToAddress(ta.Addrinfo["alfa"].RawAddress)
	require.Nil(err)
	require.Equal(len(toVotes), 1)

	totalTransfers, err := bc.GetTotalTransfers()
	require.Nil(err)
	require.Equal(totalTransfers, uint64(48))

	totalVotes, err := bc.GetTotalVotes()
	require.Nil(err)
	require.Equal(totalVotes, uint64(23))

	_, err = bc.GetTransferByTransferHash(_hash.ZeroHash32B)
	require.NotNil(err)
	_, err = bc.GetVoteByVoteHash(_hash.ZeroHash32B)
	require.NotNil(err)
	_, err = bc.StateByAddr("")
	require.NotNil(err)
}

func TestLoadBlockchainfromDBWithoutExplorer(t *testing.T) {
	require := require.New(t)
	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)
	defer testutil.CleanupPath(t, testDBPath)
	ctx := context.Background()
	// Disable block reward to make bookkeeping easier
	Gen.BlockReward = uint64(0)
	cfg := config.Default
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	sf, err := state.NewFactory(&cfg, state.DefaultTrieOption())
	require.Nil(err)
	require.NoError(sf.Start(context.Background()))
	_, err = sf.LoadOrCreateState(ta.Addrinfo["producer"].RawAddress, Gen.TotalSupply)
	require.NoError(err)
	// Create a blockchain from scratch
	bc := NewBlockchain(&cfg, PrecreatedStateFactoryOption(sf), BoltDBDaoOption())
	require.NoError(bc.Start(ctx))
	require.NotNil(bc)
	height := bc.TipHeight()
	fmt.Printf("Open blockchain pass, height = %d\n", height)
	require.Nil(addTestingTsfBlocks(bc))
	err = bc.Stop(ctx)
	require.NoError(err)
	// Load a blockchain from DB
	bc = NewBlockchain(&cfg, PrecreatedStateFactoryOption(sf), BoltDBDaoOption())
	require.NoError(bc.Start(ctx))
	defer func() {
		err := bc.Stop(ctx)
		require.NoError(err)
	}()
	require.NotNil(bc)
	// check hash<-->height mapping
	hash, err := bc.GetHashByHeight(0)
	require.Nil(err)
	height, err = bc.GetHeightByHash(hash)
	require.Nil(err)
	require.Equal(uint64(0), height)
	blk, err := bc.GetBlockByHash(hash)
	require.Nil(err)
	require.Equal(hash, blk.HashBlock())
	fmt.Printf("Genesis hash = %x\n", hash)
	hash1, err := bc.GetHashByHeight(1)
	require.Nil(err)
	height, err = bc.GetHeightByHash(hash1)
	require.Nil(err)
	require.Equal(uint64(1), height)
	blk, err = bc.GetBlockByHash(hash1)
	require.Nil(err)
	require.Equal(hash1, blk.HashBlock())
	fmt.Printf("block 1 hash = %x\n", hash1)
	hash2, err := bc.GetHashByHeight(2)
	require.Nil(err)
	height, err = bc.GetHeightByHash(hash2)
	require.Nil(err)
	require.Equal(uint64(2), height)
	blk, err = bc.GetBlockByHash(hash2)
	require.Nil(err)
	require.Equal(hash2, blk.HashBlock())
	fmt.Printf("block 2 hash = %x\n", hash2)
	hash3, err := bc.GetHashByHeight(3)
	require.Nil(err)
	height, err = bc.GetHeightByHash(hash3)
	require.Nil(err)
	require.Equal(uint64(3), height)
	blk, err = bc.GetBlockByHash(hash3)
	require.Nil(err)
	require.Equal(hash3, blk.HashBlock())
	fmt.Printf("block 3 hash = %x\n", hash3)
	hash4, err := bc.GetHashByHeight(4)
	require.Nil(err)
	height, err = bc.GetHeightByHash(hash4)
	require.Nil(err)
	require.Equal(uint64(4), height)
	blk, err = bc.GetBlockByHash(hash4)
	require.Nil(err)
	require.Equal(hash4, blk.HashBlock())
	fmt.Printf("block 4 hash = %x\n", hash4)
	empblk, err := bc.GetBlockByHash(_hash.ZeroHash32B)
	require.Nil(empblk)
	require.NotNil(err.Error())
	blk, err = bc.GetBlockByHeight(60000)
	require.Nil(blk)
	require.NotNil(err)
	// add wrong blocks
	h := bc.TipHeight()
	hash = bc.TipHash()
	blk, err = bc.GetBlockByHeight(h)
	require.Nil(err)
	require.Equal(hash, blk.HashBlock())
	fmt.Printf("Current tip = %d hash = %x\n", h, hash)
	// add block with wrong height
	cbTsf := action.NewCoinBaseTransfer(big.NewInt(50), ta.Addrinfo["bravo"].RawAddress)
	require.NotNil(cbTsf)
	blk = NewBlock(0, h+2, hash, clock.New(), []*action.Transfer{cbTsf}, nil, nil)
	err = bc.ValidateBlock(blk)
	require.NotNil(err)
	fmt.Printf("Cannot validate block %d: %v\n", blk.Height(), err)
	// add block with zero prev hash
	cbTsf2 := action.NewCoinBaseTransfer(big.NewInt(50), ta.Addrinfo["bravo"].RawAddress)
	require.NotNil(cbTsf2)
	blk = NewBlock(0, h+1, _hash.ZeroHash32B, clock.New(), []*action.Transfer{cbTsf2}, nil, nil)
	err = bc.ValidateBlock(blk)
	require.NotNil(err)
	fmt.Printf("Cannot validate block %d: %v\n", blk.Height(), err)
	// cannot add existing block again
	blk, err = bc.GetBlockByHeight(3)
	require.NotNil(blk)
	require.Nil(err)
	err = bc.(*blockchain).commitBlock(blk)
	require.NotNil(err)
	fmt.Printf("Cannot add block 3 again: %v\n", err)
	// check all Tx from block 4
	blk, err = bc.GetBlockByHeight(4)
	require.Nil(err)
	require.Equal(hash4, blk.HashBlock())
	for _, transfer := range blk.Transfers {
		transferHash := transfer.Hash()
		_, err := bc.GetBlockHashByTransferHash(transferHash)
		require.NotNil(err)
		_, err = bc.GetTransferByTransferHash(transferHash)
		require.NotNil(err)
	}
	for _, vote := range blk.Votes {
		voteHash := vote.Hash()
		_, err := bc.GetBlockHashByVoteHash(voteHash)
		require.NotNil(err)
		_, err = bc.GetVoteByVoteHash(voteHash)
		require.NotNil(err)
	}
	_, err = bc.GetTransfersFromAddress(ta.Addrinfo["charlie"].RawAddress)
	require.NotNil(err)
	_, err = bc.GetTransfersToAddress(ta.Addrinfo["charlie"].RawAddress)
	require.NotNil(err)
	_, err = bc.GetVotesFromAddress(ta.Addrinfo["charlie"].RawAddress)
	require.NotNil(err)
	_, err = bc.GetVotesFromAddress(ta.Addrinfo["alfa"].RawAddress)
	require.NotNil(err)
	_, err = bc.GetVotesToAddress(ta.Addrinfo["charlie"].RawAddress)
	require.NotNil(err)
	_, err = bc.GetVotesToAddress(ta.Addrinfo["alfa"].RawAddress)
	require.NotNil(err)
	_, err = bc.GetTotalTransfers()
	require.NotNil(err)
	_, err = bc.GetTotalVotes()
	require.NotNil(err)
	_, err = bc.GetTransferByTransferHash(_hash.ZeroHash32B)
	require.NotNil(err)
	_, err = bc.GetVoteByVoteHash(_hash.ZeroHash32B)
	require.NotNil(err)
	_, err = bc.StateByAddr("")
	require.NotNil(err)
}

func TestBlockchain_Validator(t *testing.T) {
	cfg := config.Default
	// disable account-based testing
	cfg.Chain.TrieDBPath = ""

	ctx := context.Background()
	bc := NewBlockchain(&cfg, InMemDaoOption(), InMemStateFactoryOption())
	require.NoError(t, bc.Start(ctx))
	defer func() {
		err := bc.Stop(ctx)
		assert.Nil(t, err)
	}()
	assert.NotNil(t, bc)

	val := bc.Validator()
	assert.NotNil(t, bc)
	bc.SetValidator(val)
	assert.NotNil(t, bc.Validator())
}

func TestBlockchain_MintNewDummyBlock(t *testing.T) {
	cfg := &config.Default
	testutil.CleanupPath(t, cfg.Chain.TrieDBPath)
	defer testutil.CleanupPath(t, cfg.Chain.TrieDBPath)
	testutil.CleanupPath(t, cfg.Chain.ChainDBPath)
	defer testutil.CleanupPath(t, cfg.Chain.ChainDBPath)
	require := require.New(t)
	sf, err := state.NewFactory(cfg, state.DefaultTrieOption())
	require.NoError(err)
	require.NoError(sf.Start(context.Background()))
	val := validator{sf}

	ctx := context.Background()
	bc := NewBlockchain(cfg, InMemDaoOption(), InMemStateFactoryOption())
	require.NoError(bc.Start(ctx))
	defer func() {
		err := bc.Stop(ctx)
		require.Nil(err)
	}()
	require.NotNil(bc)

	blk := bc.MintNewDummyBlock()
	require.Equal(uint64(1), blk.Height())
	tipHash := bc.TipHash()
	require.NoError(val.Validate(blk, 0, tipHash))
	tsf, _ := action.NewTransfer(1, big.NewInt(1), "", "", []byte{}, uint64(100000), big.NewInt(10))
	blk.Transfers = []*action.Transfer{tsf}
	err = val.Validate(blk, 0, tipHash)
	require.Error(err)
	require.True(
		strings.Contains(err.Error(), "failed to verify block's signature"),
	)
}

func TestBlockchainInitialCandidate(t *testing.T) {
	require := require.New(t)

	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)
	defer testutil.CleanupPath(t, testDBPath)

	cfg := config.Default
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Chain.NumCandidates = 2
	// Disable block reward to make bookkeeping easier
	Gen.BlockReward = uint64(0)

	sf, err := state.NewFactory(&cfg, state.DefaultTrieOption())
	require.Nil(err)
	require.NoError(sf.Start(context.Background()))

	height, candidate := sf.Candidates()
	require.True(height == 0)
	require.True(len(candidate) == 0)
	bc := NewBlockchain(&cfg, PrecreatedStateFactoryOption(sf), BoltDBDaoOption())
	require.NoError(bc.Start(context.Background()))
	require.NotNil(bc)
	// TODO: change the value when Candidates size is changed
	height, candidate = sf.Candidates()
	require.True(height == 0)
	require.True(len(candidate) == 2)
}

func TestCoinbaseTransfer(t *testing.T) {
	require := require.New(t)

	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)
	defer testutil.CleanupPath(t, testDBPath)

	cfg := config.Default
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath

	sf, err := state.NewFactory(&cfg, state.DefaultTrieOption())
	require.Nil(err)
	require.NoError(sf.Start(context.Background()))
	_, err = sf.LoadOrCreateState(ta.Addrinfo["producer"].RawAddress, Gen.TotalSupply)
	assert.NoError(t, err)

	Gen.BlockReward = uint64(10)

	bc := NewBlockchain(&cfg, PrecreatedStateFactoryOption(sf), BoltDBDaoOption())
	require.NoError(bc.Start(context.Background()))
	require.NotNil(bc)
	height := bc.TipHeight()
	require.Equal(0, int(height))

	transfers := []*action.Transfer{}
	blk, err := bc.MintNewBlock(transfers, nil, nil, ta.Addrinfo["producer"], "")
	require.Nil(err)
	s, err := bc.StateByAddr(ta.Addrinfo["producer"].RawAddress)
	require.Nil(err)
	b := s.Balance
	require.True(b.String() == strconv.Itoa(int(Gen.TotalSupply)))
	err = bc.CommitBlock(blk)
	require.Nil(err)
	height = bc.TipHeight()
	require.True(height == 1)
	require.True(len(blk.Transfers) == 1)
	s, err = bc.StateByAddr(ta.Addrinfo["producer"].RawAddress)
	require.Nil(err)
	b = s.Balance
	require.True(b.String() == strconv.Itoa(int(Gen.TotalSupply)+int(Gen.BlockReward)))
}

func TestBlockchain_StateByAddr(t *testing.T) {
	require := require.New(t)

	cfg := config.Default
	// disable account-based testing
	// create chain
	bc := NewBlockchain(&cfg, InMemDaoOption(), InMemStateFactoryOption())
	require.NoError(bc.Start(context.Background()))
	require.NotNil(bc)

	s, _ := bc.StateByAddr(Gen.CreatorAddr)
	require.Equal(uint64(0), s.Nonce)
	require.Equal(big.NewInt(7700000000), s.Balance)
	require.Equal(hash.ZeroHash32B, s.Root)
	require.Equal([]byte(nil), s.CodeHash)
	require.Equal(false, s.IsCandidate)
	require.Equal(big.NewInt(0), s.VotingWeight)
	require.Equal("", s.Votee)
	require.Equal(map[string]*big.Int(map[string]*big.Int(nil)), s.Voters)
}

func TestBlocks(t *testing.T) {
	// This test is used for committing block verify benchmark purpose
	t.Skip()
	require := require.New(t)
	cfg := config.Default

	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)
	defer testutil.CleanupPath(t, testDBPath)

	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath

	sf, _ := state.NewFactory(&cfg, state.InMemTrieOption())
	require.NoError(sf.Start(context.Background()))
	sf.LoadOrCreateState(ta.Addrinfo["producer"].RawAddress, Gen.TotalSupply)

	// Create a blockchain from scratch
	bc := NewBlockchain(&cfg, PrecreatedStateFactoryOption(sf), BoltDBDaoOption())
	require.NoError(bc.Start(context.Background()))
	a, _ := iotxaddress.NewAddress(iotxaddress.IsTestnet, iotxaddress.ChainID)
	c, _ := iotxaddress.NewAddress(iotxaddress.IsTestnet, iotxaddress.ChainID)
	sf.LoadOrCreateState(a.RawAddress, uint64(100000))
	sf.LoadOrCreateState(c.RawAddress, uint64(100000))

	for i := 0; i < 10; i++ {
		tsfs := []*action.Transfer{}
		for i := 0; i < 1000; i++ {
			tsf, err := action.NewTransfer(1, big.NewInt(2), a.RawAddress, c.RawAddress, []byte{}, uint64(100000), big.NewInt(10))
			require.NoError(err)
			tsf, _ = tsf.Sign(a)
			tsfs = append(tsfs, tsf)
		}
		blk, _ := bc.MintNewBlock(tsfs, nil, nil, ta.Addrinfo["producer"], "")
		err := bc.CommitBlock(blk)
		require.Nil(err)
	}
}

func TestActions(t *testing.T) {
	// This test is used for block verify benchmark purpose
	t.Skip()
	require := require.New(t)
	cfg := config.Default

	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)
	defer testutil.CleanupPath(t, testDBPath)

	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath

	sf, _ := state.NewFactory(&cfg, state.InMemTrieOption())
	require.NoError(sf.Start(context.Background()))
	sf.LoadOrCreateState(ta.Addrinfo["producer"].RawAddress, Gen.TotalSupply)

	// Create a blockchain from scratch
	bc := NewBlockchain(&cfg, PrecreatedStateFactoryOption(sf), BoltDBDaoOption())
	require.NoError(bc.Start(context.Background()))
	a, _ := iotxaddress.NewAddress(iotxaddress.IsTestnet, iotxaddress.ChainID)
	c, _ := iotxaddress.NewAddress(iotxaddress.IsTestnet, iotxaddress.ChainID)
	sf.LoadOrCreateState(a.RawAddress, uint64(100000))
	sf.LoadOrCreateState(c.RawAddress, uint64(100000))

	val := validator{sf}
	tsfs := []*action.Transfer{}
	votes := []*action.Vote{}
	for i := 0; i < 5000; i++ {
		tsf, err := action.NewTransfer(1, big.NewInt(2), a.RawAddress, c.RawAddress, []byte{}, uint64(100000), big.NewInt(10))
		require.NoError(err)
		tsf, _ = tsf.Sign(a)
		tsfs = append(tsfs, tsf)

		vote, err := action.NewVote(1, a.RawAddress, a.RawAddress, uint64(100000), big.NewInt(10))
		require.NoError(err)
		vote, _ = vote.Sign(a)
		votes = append(votes, vote)
	}
	blk, _ := bc.MintNewBlock(tsfs, votes, nil, ta.Addrinfo["producer"], "")
	require.Nil(val.Validate(blk, 0, blk.PrevHash()))
}

func TestDummyReplacement(t *testing.T) {
	require := require.New(t)
	cfg := config.Default

	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)
	defer testutil.CleanupPath(t, testDBPath)

	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath

	sf, _ := state.NewFactory(&cfg, state.InMemTrieOption())
	require.NoError(sf.Start(context.Background()))
	sf.LoadOrCreateState(ta.Addrinfo["producer"].RawAddress, Gen.TotalSupply)

	// Create a blockchain from scratch
	bc := NewBlockchain(&cfg, PrecreatedStateFactoryOption(sf), BoltDBDaoOption())
	require.NoError(bc.Start(context.Background()))
	dummy := bc.MintNewDummyBlock()
	require.NoError(bc.CommitBlock(dummy))
	actualDummyBlock, err := bc.GetBlockByHeight(1)
	require.NoError(err)
	require.Equal(dummy.HashBlock(), actualDummyBlock.HashBlock())

	chain := bc.(*blockchain)
	chain.tipHeight = 0
	realBlock, err := bc.MintNewBlock(nil, nil, nil, ta.Addrinfo["producer"], "")
	require.NotNil(realBlock)
	require.NoError(err)
	require.NoError(bc.CommitBlock(realBlock))
	actualRealBlock, err := bc.GetBlockByHeight(1)
	require.NoError(err)
	require.Equal(realBlock.HashBlock(), actualRealBlock.HashBlock())

	block2, err := bc.MintNewBlock(nil, nil, nil, ta.Addrinfo["producer"], "")
	require.NoError(err)
	require.NoError(bc.CommitBlock(block2))
	dummyBlock3 := bc.MintNewDummyBlock()
	require.NoError(bc.CommitBlock(dummyBlock3))
	block4, err := bc.MintNewBlock(nil, nil, nil, ta.Addrinfo["producer"], "")
	require.NoError(err)
	require.NoError(bc.CommitBlock(block4))
	actualDummyBlock3, err := bc.GetBlockByHeight(3)
	require.NoError(err)
	require.True(actualDummyBlock3.IsDummyBlock())
	chain.tipHeight = 2
	block3, err := bc.MintNewBlock(nil, nil, nil, ta.Addrinfo["producer"], "")
	require.NotNil(block3)
	require.NoError(err)
	require.NoError(bc.CommitBlock(block3))
	actualBlock3, err := bc.GetBlockByHeight(3)
	require.NoError(err)
	require.Equal(block3.HashBlock(), actualBlock3.HashBlock())
}

func TestMintNewBlock(t *testing.T) {
	t.Parallel()
	cfg := config.Default
	clk := clock.NewMock()
	chain := NewBlockchain(&cfg, InMemDaoOption(), InMemStateFactoryOption(), ClockOption(clk))
	require.NoError(t, chain.Start(context.Background()))
	blk1 := chain.MintNewDummyBlock()
	clk.Add(2 * time.Second)
	blk2 := chain.MintNewDummyBlock()
	require.Equal(t, uint64(2), blk2.Header.timestamp-blk1.Header.timestamp)
	require.Equal(t, blk1.HashBlock(), blk2.HashBlock())
}

func TestMintDKGBlock(t *testing.T) {
	require := require.New(t)
	lastSeed, _ := hex.DecodeString("9de6306b08158c423330f7a27243a1a5cbe39bfd764f07818437882d21241567")
	cfg := config.Default
	clk := clock.NewMock()
	chain := NewBlockchain(&cfg, InMemDaoOption(), InMemStateFactoryOption(), ClockOption(clk))
	require.NoError(chain.Start(context.Background()))

	var err error
	const numNodes = 21
	addresses := make([]*iotxaddress.Address, numNodes)
	skList := make([][]uint32, numNodes)
	idList := make([][]uint8, numNodes)
	coeffsList := make([][][]uint32, numNodes)
	sharesList := make([][][]uint32, numNodes)
	shares := make([][]uint32, numNodes)
	witnessesList := make([][][]byte, numNodes)
	sharestatusmatrix := make([][numNodes]bool, numNodes)
	qsList := make([][]byte, numNodes)
	pkList := make([][]byte, numNodes)
	askList := make([][]uint32, numNodes)

	// Generate 21 identifiers for the delegates
	for i := 0; i < numNodes; i++ {
		addresses[i], _ = iotxaddress.NewAddress(iotxaddress.IsTestnet, iotxaddress.ChainID)
		idList[i] = hash.Hash256b([]byte(addresses[i].RawAddress))
		skList[i] = crypto.DKG.SkGeneration()
	}

	// Initialize DKG and generate secret shares
	for i := 0; i < numNodes; i++ {
		coeffsList[i], sharesList[i], witnessesList[i], err = crypto.DKG.Init(skList[i], idList)
		require.NoError(err)
	}

	// Verify all the received secret shares
	for i := 0; i < numNodes; i++ {
		for j := 0; j < numNodes; j++ {
			shares[j] = sharesList[j][i]
		}
		sharestatusmatrix[i], err = crypto.DKG.SharesCollect(idList[i], shares, witnessesList)
		require.NoError(err)
		for _, b := range sharestatusmatrix[i] {
			require.True(b)
		}
	}

	// Generate private and public key shares of a group key
	for i := 0; i < numNodes; i++ {
		for j := 0; j < numNodes; j++ {
			shares[j] = sharesList[j][i]
		}
		qsList[i], pkList[i], askList[i], err = crypto.DKG.KeyPairGeneration(shares, sharestatusmatrix)
		require.NoError(err)
	}

	// Generate dkg signature for each block
	require.NoError(err)
	dummy := chain.MintNewDummyBlock()
	err = chain.CommitBlock(dummy)
	require.NoError(err)
	for i := 1; i < numNodes; i++ {
		blk, err := chain.MintNewDKGBlock(nil, nil, nil, addresses[i],
			&iotxaddress.DKGAddress{PrivateKey: askList[i], PublicKey: pkList[i], ID: idList[i]},
			lastSeed, "")
		require.NoError(err)
		err = chain.CommitBlock(blk)
		require.NoError(err)
		require.Equal(pkList[i], blk.Header.DKGPubkey)
		require.Equal(idList[i], blk.Header.DKGID)
		require.True(len(blk.Header.DKGBlockSig) > 0)
	}
	height, candidates := chain.Candidates()
	require.True(21 == height)
	require.True(21 == len(candidates))
}
