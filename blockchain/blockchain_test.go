// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/blockchain/action"
	"github.com/iotexproject/iotex-core/blockchain/trx"
	"github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/state"
	ta "github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/test/util"
	"github.com/iotexproject/iotex-core/trie"
)

const (
	testingConfigPath = "../config.yaml"
	testDBPath        = "db.test"
	testCoinbaseData  = "The Times 03/Jan/2009 Chancellor on brink of second bailout for banks"
	testTriePath      = "trie.test"
)

func addTestingTsfBlocks(bc Blockchain) error {
	// Add block 1
	// test --> A, B, C, D, E, F
	tsf1 := action.NewTransfer(1, big.NewInt(20), ta.Addrinfo["miner"].RawAddress, ta.Addrinfo["alfa"].RawAddress)
	tsf1, err := tsf1.Sign(ta.Addrinfo["miner"])
	tsf2 := action.NewTransfer(1, big.NewInt(30), ta.Addrinfo["miner"].RawAddress, ta.Addrinfo["bravo"].RawAddress)
	tsf2, err = tsf2.Sign(ta.Addrinfo["miner"])
	tsf3 := action.NewTransfer(1, big.NewInt(50), ta.Addrinfo["miner"].RawAddress, ta.Addrinfo["charlie"].RawAddress)
	tsf3, err = tsf3.Sign(ta.Addrinfo["miner"])
	tsf4 := action.NewTransfer(1, big.NewInt(70), ta.Addrinfo["miner"].RawAddress, ta.Addrinfo["delta"].RawAddress)
	tsf4, err = tsf4.Sign(ta.Addrinfo["miner"])
	tsf5 := action.NewTransfer(1, big.NewInt(110), ta.Addrinfo["miner"].RawAddress, ta.Addrinfo["echo"].RawAddress)
	tsf5, err = tsf5.Sign(ta.Addrinfo["miner"])
	tsf6 := action.NewTransfer(1, big.NewInt(50<<20), ta.Addrinfo["miner"].RawAddress, ta.Addrinfo["foxtrot"].RawAddress)
	tsf6, err = tsf6.Sign(ta.Addrinfo["miner"])

	blk, err := bc.MintNewBlock(nil, []*action.Transfer{tsf1, tsf2, tsf3, tsf4, tsf5, tsf6}, nil, ta.Addrinfo["miner"], "")
	if err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	// Add block 2
	// Charlie --> A, B, D, E, test
	tsf1 = action.NewTransfer(1, big.NewInt(1), ta.Addrinfo["charlie"].RawAddress, ta.Addrinfo["alfa"].RawAddress)
	tsf1, err = tsf1.Sign(ta.Addrinfo["charlie"])
	tsf2 = action.NewTransfer(1, big.NewInt(1), ta.Addrinfo["charlie"].RawAddress, ta.Addrinfo["bravo"].RawAddress)
	tsf2, err = tsf2.Sign(ta.Addrinfo["charlie"])
	tsf3 = action.NewTransfer(1, big.NewInt(1), ta.Addrinfo["charlie"].RawAddress, ta.Addrinfo["delta"].RawAddress)
	tsf3, err = tsf3.Sign(ta.Addrinfo["charlie"])
	tsf4 = action.NewTransfer(1, big.NewInt(1), ta.Addrinfo["charlie"].RawAddress, ta.Addrinfo["echo"].RawAddress)
	tsf4, err = tsf4.Sign(ta.Addrinfo["charlie"])
	tsf5 = action.NewTransfer(1, big.NewInt(1), ta.Addrinfo["charlie"].RawAddress, ta.Addrinfo["miner"].RawAddress)
	tsf5, err = tsf5.Sign(ta.Addrinfo["charlie"])
	blk, err = bc.MintNewBlock(nil, []*action.Transfer{tsf1, tsf2, tsf3, tsf4, tsf5}, nil, ta.Addrinfo["miner"], "")
	if err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	// Add block 3
	// Delta --> B, E, F, test
	tsf1 = action.NewTransfer(1, big.NewInt(1), ta.Addrinfo["delta"].RawAddress, ta.Addrinfo["bravo"].RawAddress)
	tsf1, err = tsf1.Sign(ta.Addrinfo["delta"])
	tsf2 = action.NewTransfer(1, big.NewInt(1), ta.Addrinfo["delta"].RawAddress, ta.Addrinfo["echo"].RawAddress)
	tsf2, err = tsf2.Sign(ta.Addrinfo["delta"])
	tsf3 = action.NewTransfer(1, big.NewInt(1), ta.Addrinfo["delta"].RawAddress, ta.Addrinfo["foxtrot"].RawAddress)
	tsf3, err = tsf3.Sign(ta.Addrinfo["delta"])
	tsf4 = action.NewTransfer(1, big.NewInt(1), ta.Addrinfo["delta"].RawAddress, ta.Addrinfo["miner"].RawAddress)
	tsf4, err = tsf4.Sign(ta.Addrinfo["delta"])
	blk, err = bc.MintNewBlock(nil, []*action.Transfer{tsf1, tsf2, tsf3, tsf4}, nil, ta.Addrinfo["miner"], "")
	if err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	// Add block 4
	// Delta --> A, B, C, D, F, test
	tsf1 = action.NewTransfer(1, big.NewInt(2), ta.Addrinfo["echo"].RawAddress, ta.Addrinfo["alfa"].RawAddress)
	tsf1, err = tsf1.Sign(ta.Addrinfo["echo"])
	tsf2 = action.NewTransfer(1, big.NewInt(2), ta.Addrinfo["echo"].RawAddress, ta.Addrinfo["bravo"].RawAddress)
	tsf2, err = tsf2.Sign(ta.Addrinfo["echo"])
	tsf3 = action.NewTransfer(1, big.NewInt(2), ta.Addrinfo["echo"].RawAddress, ta.Addrinfo["charlie"].RawAddress)
	tsf3, err = tsf3.Sign(ta.Addrinfo["echo"])
	tsf4 = action.NewTransfer(1, big.NewInt(2), ta.Addrinfo["echo"].RawAddress, ta.Addrinfo["delta"].RawAddress)
	tsf4, err = tsf4.Sign(ta.Addrinfo["echo"])
	tsf5 = action.NewTransfer(1, big.NewInt(2), ta.Addrinfo["echo"].RawAddress, ta.Addrinfo["foxtrot"].RawAddress)
	tsf5, err = tsf5.Sign(ta.Addrinfo["echo"])
	tsf6 = action.NewTransfer(1, big.NewInt(2), ta.Addrinfo["echo"].RawAddress, ta.Addrinfo["miner"].RawAddress)
	tsf6, err = tsf6.Sign(ta.Addrinfo["echo"])
	blk, err = bc.MintNewBlock(nil, []*action.Transfer{tsf1, tsf2, tsf3, tsf4, tsf5, tsf6}, nil, ta.Addrinfo["miner"], "")
	if err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	return nil
}

func TestCreateBlockchain(t *testing.T) {
	assert := assert.New(t)

	config, err := config.LoadConfigWithPathWithoutValidation(testingConfigPath)
	assert.Nil(err)
	// disable account-based testing
	config.Chain.TrieDBPath = ""
	config.Chain.InMemTest = true
	// Disable block reward to make bookkeeping easier
	Gen.BlockReward = uint64(0)

	// create chain
	bc := CreateBlockchain(config, nil)
	assert.NotNil(bc)
	height, err := bc.TipHeight()
	assert.Nil(err)
	assert.Equal(0, int(height))
	fmt.Printf("Create blockchain pass, height = %d\n", height)
	defer bc.Stop()

	// verify Genesis block
	genesis, _ := bc.GetBlockByHeight(0)
	assert.NotNil(genesis)
	// serialize
	data, err := genesis.Serialize()
	assert.Nil(err)

	assert.Equal(0, len(genesis.Tranxs))
	assert.Equal(0, len(genesis.Transfers))
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
	height, err = bc.TipHeight()
	assert.Nil(err)
	assert.Equal(4, int(height))
}

func TestLoadBlockchainfromDB(t *testing.T) {
	assert := assert.New(t)

	config, err := config.LoadConfigWithPathWithoutValidation(testingConfigPath)
	assert.Nil(err)
	util.CleanupPath(t, testTriePath)
	defer util.CleanupPath(t, testTriePath)
	util.CleanupPath(t, testDBPath)
	defer util.CleanupPath(t, testDBPath)

	config.Chain.TrieDBPath = testTriePath
	config.Chain.InMemTest = false
	config.Chain.ChainDBPath = testDBPath

	tr, _ := trie.NewTrie(testTriePath, false)
	sf := state.NewFactory(tr)
	sf.CreateState(ta.Addrinfo["miner"].RawAddress, Gen.TotalSupply)
	// Disable block reward to make bookkeeping easier
	Gen.BlockReward = uint64(0)
	// Create a blockchain from scratch
	bc := CreateBlockchain(config, sf)
	assert.NotNil(bc)
	height, err := bc.TipHeight()
	assert.Nil(err)
	fmt.Printf("Open blockchain pass, height = %d\n", height)
	assert.Nil(addTestingTsfBlocks(bc))
	bc.Stop()

	// Load a blockchain from DB
	bc = CreateBlockchain(config, sf)
	defer bc.Stop()
	assert.NotNil(bc)

	// check hash<-->height mapping
	hash, err := bc.GetHashByHeight(0)
	assert.Nil(err)
	height, err = bc.GetHeightByHash(hash)
	assert.Nil(err)
	assert.Equal(uint64(0), height)
	blk, err := bc.GetBlockByHash(hash)
	assert.Nil(err)
	assert.Equal(hash, blk.HashBlock())
	fmt.Printf("Genesis hash = %x\n", hash)

	hash1, err := bc.GetHashByHeight(1)
	assert.Nil(err)
	height, err = bc.GetHeightByHash(hash1)
	assert.Nil(err)
	assert.Equal(uint64(1), height)
	blk, err = bc.GetBlockByHash(hash1)
	assert.Nil(err)
	assert.Equal(hash1, blk.HashBlock())
	fmt.Printf("block 1 hash = %x\n", hash1)

	hash2, err := bc.GetHashByHeight(2)
	assert.Nil(err)
	height, err = bc.GetHeightByHash(hash2)
	assert.Nil(err)
	assert.Equal(uint64(2), height)
	blk, err = bc.GetBlockByHash(hash2)
	assert.Nil(err)
	assert.Equal(hash2, blk.HashBlock())
	fmt.Printf("block 2 hash = %x\n", hash2)

	hash3, err := bc.GetHashByHeight(3)
	assert.Nil(err)
	height, err = bc.GetHeightByHash(hash3)
	assert.Nil(err)
	assert.Equal(uint64(3), height)
	blk, err = bc.GetBlockByHash(hash3)
	assert.Nil(err)
	assert.Equal(hash3, blk.HashBlock())
	fmt.Printf("block 3 hash = %x\n", hash3)

	hash4, err := bc.GetHashByHeight(4)
	assert.Nil(err)
	height, err = bc.GetHeightByHash(hash4)
	assert.Nil(err)
	assert.Equal(uint64(4), height)
	blk, err = bc.GetBlockByHash(hash4)
	assert.Nil(err)
	assert.Equal(hash4, blk.HashBlock())
	fmt.Printf("block 4 hash = %x\n", hash4)

	empblk, err := bc.GetBlockByHash(common.ZeroHash32B)
	assert.Nil(empblk)
	assert.NotNil(err.Error())

	blk, err = bc.GetBlockByHeight(60000)
	assert.Nil(blk)
	assert.NotNil(err)

	// add wrong blocks
	h, err := bc.TipHeight()
	assert.Nil(err)
	hash, err = bc.TipHash()
	assert.Nil(err)
	blk, err = bc.GetBlockByHeight(h)
	assert.Nil(err)
	assert.Equal(hash, blk.HashBlock())
	fmt.Printf("Current tip = %d hash = %x\n", h, hash)

	// add block with wrong height
	cbTsf := action.NewCoinBaseTransfer(big.NewInt(50), ta.Addrinfo["bravo"].RawAddress)
	assert.NotNil(cbTsf)
	blk = NewBlock(0, h+2, hash, nil, []*action.Transfer{cbTsf}, nil)
	err = bc.ValidateBlock(blk)
	assert.NotNil(err)
	fmt.Printf("Cannot validate block %d: %v\n", blk.Height(), err)

	// add block with zero prev hash
	cbTsf2 := action.NewCoinBaseTransfer(big.NewInt(50), ta.Addrinfo["bravo"].RawAddress)
	assert.NotNil(cbTsf2)
	blk = NewBlock(0, h+1, common.ZeroHash32B, nil, []*action.Transfer{cbTsf2}, nil)
	err = bc.ValidateBlock(blk)
	assert.NotNil(err)
	fmt.Printf("Cannot validate block %d: %v\n", blk.Height(), err)

	// cannot add existing block again
	blk, err = bc.GetBlockByHeight(3)
	assert.NotNil(blk)
	assert.Nil(err)
	err = bc.(*blockchain).commitBlock(blk)
	assert.NotNil(err)
	fmt.Printf("Cannot add block 3 again: %v\n", err)

	// check all Tx from block 4
	blk, err = bc.GetBlockByHeight(4)
	assert.Nil(err)
	assert.Equal(hash4, blk.HashBlock())
	for _, transfer := range blk.Transfers {
		transferHash := transfer.Hash()
		hash, err := bc.GetBlockHashByTransferHash(transferHash)
		assert.Nil(err)
		assert.Equal(hash, hash4)
		transfer1, err := bc.GetTransferByTransferHash(transferHash)
		assert.Nil(err)
		assert.Equal(transfer1.Hash(), transferHash)
	}

	fromTransfers, err := bc.GetTransfersFromAddress(ta.Addrinfo["charlie"].RawAddress)
	assert.Nil(err)
	assert.Equal(len(fromTransfers), 5)

	toTransfers, err := bc.GetTransfersToAddress(ta.Addrinfo["charlie"].RawAddress)
	assert.Nil(err)
	assert.Equal(len(toTransfers), 2)

	totalTransfers, err := bc.GetTotalTransfers()
	assert.Nil(err)
	assert.Equal(totalTransfers, uint64(25))
}

func TestBlockchain_Validator(t *testing.T) {
	config, err := config.LoadConfigWithPathWithoutValidation(testingConfigPath)
	assert.Nil(t, err)
	// disable account-based testing
	config.Chain.TrieDBPath = ""
	config.Chain.InMemTest = true

	bc := CreateBlockchain(config, nil)
	defer bc.Stop()
	assert.NotNil(t, bc)

	val := bc.Validator()
	assert.NotNil(t, bc)
	bc.SetValidator(val)
	assert.NotNil(t, bc.Validator())
}

func TestBlockchain_MintNewDummyBlock(t *testing.T) {
	config, err := config.LoadConfigWithPathWithoutValidation(testingConfigPath)
	assert.Nil(t, err)
	// disable account-based testing
	config.Chain.TrieDBPath = ""
	config.Chain.InMemTest = true

	bc := CreateBlockchain(config, nil)
	defer bc.Stop()
	assert.NotNil(t, bc)

	blk, err := bc.MintNewDummyBlock()
	assert.Nil(t, err)
	assert.Equal(t, uint64(1), blk.Height())
	assert.Equal(t, 0, len(blk.Tranxs))
}

func TestBlockchainInitialCandidate(t *testing.T) {
	require := require.New(t)

	config, err := config.LoadConfigWithPathWithoutValidation(testingConfigPath)
	require.Nil(err)
	util.CleanupPath(t, testTriePath)
	defer util.CleanupPath(t, testTriePath)
	util.CleanupPath(t, testDBPath)
	defer util.CleanupPath(t, testDBPath)

	config.Chain.TrieDBPath = testTriePath
	config.Chain.InMemTest = false
	config.Chain.ChainDBPath = testDBPath
	// Disable block reward to make bookkeeping easier
	Gen.BlockReward = uint64(0)

	tr, _ := trie.NewTrie(testTriePath, false)
	sf := state.NewFactory(tr)

	for _, pk := range initDelegatePK {
		pubk, err := hex.DecodeString(pk)
		require.Nil(err)
		address, err := iotxaddress.GetAddress(pubk, iotxaddress.IsTestnet, iotxaddress.ChainID)
		require.Nil(err)
		_, err = sf.CreateState(address.RawAddress, uint64(0))
		require.Nil(err)
	}

	height, candidate := sf.Candidates()
	require.True(height == 0)
	require.True(len(candidate) == 0)
	bc := CreateBlockchain(config, sf)
	require.NotNil(t, bc)
	// TODO: change the value when Candidates size is changed
	height, candidate = sf.Candidates()
	require.True(height == 0)
	require.True(len(candidate) == 2)
}

func TestCoinbaseTransfer(t *testing.T) {
	require := require.New(t)
	config, err := config.LoadConfigWithPathWithoutValidation(testingConfigPath)
	require.Nil(err)
	util.CleanupPath(t, testTriePath)
	defer util.CleanupPath(t, testTriePath)
	util.CleanupPath(t, testDBPath)
	defer util.CleanupPath(t, testDBPath)

	config.Chain.TrieDBPath = testTriePath
	config.Chain.InMemTest = false
	config.Chain.ChainDBPath = testDBPath

	tr, _ := trie.NewTrie(testTriePath, false)
	sf := state.NewFactory(tr)
	sf.CreateState(ta.Addrinfo["miner"].RawAddress, Gen.TotalSupply)

	Gen.BlockReward = uint64(10)

	bc := CreateBlockchain(config, sf)
	require.NotNil(bc)
	height, err := bc.TipHeight()
	require.Nil(err)
	require.Equal(0, int(height))

	transfers := []*action.Transfer{}
	blk, err := bc.MintNewBlock([]*trx.Tx{}, transfers, nil, ta.Addrinfo["miner"], "")
	require.Nil(err)
	b := bc.BalanceOf(ta.Addrinfo["miner"].RawAddress)
	require.True(b.String() == strconv.Itoa(int(Gen.TotalSupply)))
	err = bc.CommitBlock(blk)
	require.Nil(err)
	height, err = bc.TipHeight()
	require.Nil(err)
	require.True(height == 1)
	require.True(len(blk.Transfers) == 1)
	b = bc.BalanceOf(ta.Addrinfo["miner"].RawAddress)
	require.True(b.String() == strconv.Itoa(int(Gen.TotalSupply)+int(Gen.BlockReward)))
}

func TestBlockchain_StateByAddr(t *testing.T) {
	require := require.New(t)

	config, err := config.LoadConfigWithPathWithoutValidation(testingConfigPath)
	require.Nil(err)
	// disable account-based testing
	config.Chain.InMemTest = true
	// create chain
	bc := CreateBlockchain(config, nil)
	require.NotNil(bc)

	s, _ := bc.StateByAddr(Gen.Creator)
	require.Equal(uint64(0), s.Nonce)
	require.Equal(big.NewInt(int64(Gen.TotalSupply)), s.Balance)
	require.Equal(Gen.Creator, s.Address)
	require.Equal(false, s.IsCandidate)
	require.Equal(big.NewInt(0), s.VotingWeight)
	require.Equal("", s.Votee)
	require.Equal(map[string]*big.Int(map[string]*big.Int(nil)), s.Voters)
}
