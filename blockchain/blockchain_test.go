// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"encoding/hex"
	"errors"
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

func addTestingBlocks(bc Blockchain) error {
	// Add block 1
	// test --> A, B, C, D, E, F
	payee := []*Payee{}
	payee = append(payee, &Payee{ta.Addrinfo["alfa"].RawAddress, 20})
	payee = append(payee, &Payee{ta.Addrinfo["bravo"].RawAddress, 30})
	payee = append(payee, &Payee{ta.Addrinfo["charlie"].RawAddress, 50})
	payee = append(payee, &Payee{ta.Addrinfo["delta"].RawAddress, 70})
	payee = append(payee, &Payee{ta.Addrinfo["echo"].RawAddress, 110})
	payee = append(payee, &Payee{ta.Addrinfo["foxtrot"].RawAddress, 50 << 20})
	transfers := []*action.Transfer{}
	transfers = append(transfers, action.NewTransfer(0, big.NewInt(1), ta.Addrinfo["miner"].RawAddress, ta.Addrinfo["charlie"].RawAddress))
	tx := bc.CreateTransaction(ta.Addrinfo["miner"], 280+(50<<20), payee)
	if tx == nil {
		return errors.New("empty tx for block 1")
	}
	blk, err := bc.MintNewBlock([]*trx.Tx{tx}, transfers, nil, ta.Addrinfo["miner"], "")
	if err != nil {
		return err
	}
	if err := bc.AddBlockCommit(blk); err != nil {
		return err
	}
	bc.ResetUTXO()

	// Add block 2
	// Charlie --> A, B, D, E, test
	payee = nil
	payee = append(payee, &Payee{ta.Addrinfo["alfa"].RawAddress, 1})
	payee = append(payee, &Payee{ta.Addrinfo["bravo"].RawAddress, 1})
	payee = append(payee, &Payee{ta.Addrinfo["charlie"].RawAddress, 1})
	payee = append(payee, &Payee{ta.Addrinfo["delta"].RawAddress, 1})
	payee = append(payee, &Payee{ta.Addrinfo["miner"].RawAddress, 1})
	tx = bc.CreateTransaction(ta.Addrinfo["charlie"], 5, payee)
	transfers = nil
	transfers = append(transfers, action.NewTransfer(0, big.NewInt(1), ta.Addrinfo["charlie"].RawAddress, ta.Addrinfo["alfa"].RawAddress))
	transfers = append(transfers, action.NewTransfer(0, big.NewInt(1), ta.Addrinfo["charlie"].RawAddress, ta.Addrinfo["bravo"].RawAddress))
	transfers = append(transfers, action.NewTransfer(0, big.NewInt(1), ta.Addrinfo["charlie"].RawAddress, ta.Addrinfo["delta"].RawAddress))
	transfers = append(transfers, action.NewTransfer(0, big.NewInt(1), ta.Addrinfo["charlie"].RawAddress, ta.Addrinfo["miner"].RawAddress))
	blk, err = bc.MintNewBlock([]*trx.Tx{tx}, transfers, nil, ta.Addrinfo["miner"], "")
	if err != nil {
		return err
	}
	if err := bc.AddBlockCommit(blk); err != nil {
		return err
	}
	bc.ResetUTXO()

	// Add block 3
	// Delta --> B, E, F, test
	payee = payee[1:]
	payee[1] = &Payee{ta.Addrinfo["echo"].RawAddress, 1}
	payee[2] = &Payee{ta.Addrinfo["foxtrot"].RawAddress, 1}
	tx = bc.CreateTransaction(ta.Addrinfo["delta"], 4, payee)
	blk, err = bc.MintNewBlock([]*trx.Tx{tx}, nil, nil, ta.Addrinfo["miner"], "")
	if err != nil {
		return err
	}
	if err := bc.AddBlockCommit(blk); err != nil {
		return err
	}
	bc.ResetUTXO()

	// Add block 4
	// Delta --> A, B, C, D, F, test
	payee = nil
	payee = append(payee, &Payee{ta.Addrinfo["alfa"].RawAddress, 2})
	payee = append(payee, &Payee{ta.Addrinfo["bravo"].RawAddress, 2})
	payee = append(payee, &Payee{ta.Addrinfo["charlie"].RawAddress, 2})
	payee = append(payee, &Payee{ta.Addrinfo["delta"].RawAddress, 2})
	payee = append(payee, &Payee{ta.Addrinfo["foxtrot"].RawAddress, 2})
	payee = append(payee, &Payee{ta.Addrinfo["miner"].RawAddress, 2})
	tx = bc.CreateTransaction(ta.Addrinfo["echo"], 12, payee)
	blk, err = bc.MintNewBlock([]*trx.Tx{tx}, nil, nil, ta.Addrinfo["miner"], "")
	if err != nil {
		return err
	}
	if err := bc.AddBlockCommit(blk); err != nil {
		return err
	}
	bc.ResetUTXO()

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

	assert.Equal(1, len(genesis.Tranxs))
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
	assert.Nil(addTestingBlocks(bc))
	height, err = bc.TipHeight()
	assert.Nil(err)
	assert.Equal(4, int(height))
}

func TestLoadBlockchainfromDB(t *testing.T) {
	assert := assert.New(t)
	util.CleanupPath(t, testDBPath)
	defer util.CleanupPath(t, testDBPath)
	config, err := config.LoadConfigWithPathWithoutValidation(testingConfigPath)
	assert.Nil(err)
	config.Chain.ChainDBPath = testDBPath
	// disable account-based testing
	config.Chain.TrieDBPath = ""
	// Disable block reward to make bookkeeping easier
	Gen.BlockReward = uint64(0)
	// Create a blockchain from scratch
	bc := CreateBlockchain(config, nil)
	assert.NotNil(bc)
	height, err := bc.TipHeight()
	assert.Nil(err)
	fmt.Printf("Open blockchain pass, height = %d\n", height)
	assert.Nil(addTestingBlocks(bc))
	bc.Stop()

	// Load a blockchain from DB
	bc = CreateBlockchain(config, nil)
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
	cbTx := trx.NewCoinbaseTx(ta.Addrinfo["bravo"].RawAddress, 50, testCoinbaseData)
	assert.NotNil(cbTx)
	blk = NewBlock(0, h+2, hash, []*trx.Tx{cbTx}, nil, nil)
	err = bc.ValidateBlock(blk)
	assert.NotNil(err)
	fmt.Printf("Cannot validate block %d: %v\n", blk.Height(), err)

	// add block with zero prev hash
	cbTx = trx.NewCoinbaseTx(ta.Addrinfo["bravo"].RawAddress, 50, testCoinbaseData)
	assert.NotNil(cbTx)
	blk = NewBlock(0, h+1, common.ZeroHash32B, []*trx.Tx{cbTx}, nil, nil)
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
	assert.Equal(len(fromTransfers), 4)

	toTransfers, err := bc.GetTransfersToAddress(ta.Addrinfo["charlie"].RawAddress)
	assert.Nil(err)
	assert.Equal(len(toTransfers), 1)
}

func TestEmptyBlockOnlyHasCoinbaseTx(t *testing.T) {
	config, err := config.LoadConfigWithPathWithoutValidation(testingConfigPath)
	assert.Nil(t, err)
	// disable account-based testing
	config.Chain.TrieDBPath = ""
	config.Chain.InMemTest = true
	Gen.BlockReward = uint64(7777)

	bc := CreateBlockchain(config, nil)
	defer bc.Stop()
	assert.NotNil(t, bc)

	blk, err := bc.MintNewBlock([]*trx.Tx{}, nil, nil, ta.Addrinfo["miner"], "")
	assert.Nil(t, err)
	assert.Equal(t, uint64(1), blk.Height())
	assert.Equal(t, 1, len(blk.Tranxs))
	assert.True(t, blk.Tranxs[0].IsCoinbase())
	assert.Equal(t, 1, len(blk.Tranxs[0].TxIn))
	assert.Equal(t, 1, len(blk.Tranxs[0].TxOut))
	assert.Equal(t, uint64(7777), blk.Tranxs[0].TxOut[0].Value)
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

	Gen.BlockReward = uint64(10)

	bc := CreateBlockchain(config, sf)
	assert.NotNil(t, bc)
	height, err := bc.TipHeight()
	assert.Nil(t, err)
	assert.Equal(t, 0, int(height))

	transfers := []*action.Transfer{}
	blk, err := bc.MintNewBlock([]*trx.Tx{}, transfers, nil, ta.Addrinfo["miner"], "")
	assert.Nil(t, err)
	bal, _ := bc.BalanceNonceOf(ta.Addrinfo["miner"].RawAddress)
	require.True(bal.String() == "10000000000")
	err = bc.AddBlockCommit(blk)
	assert.Nil(t, err)
	bc.ResetUTXO()
	bal, _ = bc.BalanceNonceOf(ta.Addrinfo["miner"].RawAddress)
	require.True(bal.String() == strconv.Itoa(10000000000+int(Gen.BlockReward)))
}
