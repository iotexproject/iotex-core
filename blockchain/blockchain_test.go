// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/config"
	ta "github.com/iotexproject/iotex-core/test/testaddress"
)

const (
	testingConfigPath = "../config.yaml"
	testDBPath        = "db.test"
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
	tx := bc.CreateTransaction(ta.Addrinfo["miner"], 280+(50<<20), payee)
	if tx == nil {
		return errors.New("empty tx for block 1")
	}
	blk := bc.MintNewBlock([]*Tx{tx}, ta.Addrinfo["miner"], "")
	if err := bc.AddBlockCommit(blk); err != nil {
		return err
	}
	bc.Reset()

	// Add block 2
	// Charlie --> A, B, D, E, test
	payee = nil
	payee = append(payee, &Payee{ta.Addrinfo["alfa"].RawAddress, 1})
	payee = append(payee, &Payee{ta.Addrinfo["bravo"].RawAddress, 1})
	payee = append(payee, &Payee{ta.Addrinfo["charlie"].RawAddress, 1})
	payee = append(payee, &Payee{ta.Addrinfo["delta"].RawAddress, 1})
	payee = append(payee, &Payee{ta.Addrinfo["miner"].RawAddress, 1})
	tx = bc.CreateTransaction(ta.Addrinfo["charlie"], 5, payee)
	blk = bc.MintNewBlock([]*Tx{tx}, ta.Addrinfo["miner"], "")
	if err := bc.AddBlockCommit(blk); err != nil {
		return err
	}
	bc.Reset()

	// Add block 3
	// Delta --> B, E, F, test
	payee = payee[1:]
	payee[1] = &Payee{ta.Addrinfo["echo"].RawAddress, 1}
	payee[2] = &Payee{ta.Addrinfo["foxtrot"].RawAddress, 1}
	tx = bc.CreateTransaction(ta.Addrinfo["delta"], 4, payee)
	blk = bc.MintNewBlock([]*Tx{tx}, ta.Addrinfo["miner"], "")
	if err := bc.AddBlockCommit(blk); err != nil {
		return err
	}
	bc.Reset()

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
	blk = bc.MintNewBlock([]*Tx{tx}, ta.Addrinfo["miner"], "")
	if err := bc.AddBlockCommit(blk); err != nil {
		return err
	}
	bc.Reset()

	return nil
}

func TestCreateBlockchain(t *testing.T) {
	defer os.Remove(testDBPath)
	assert := assert.New(t)

	config, err := config.LoadConfigWithPathWithoutValidation(testingConfigPath)
	assert.Nil(err)
	config.Chain.ChainDBPath = testDBPath
	// Disable block reward to make bookkeeping easier
	Gen.BlockReward = uint64(0)

	// create chain
	bc := CreateBlockchain(ta.Addrinfo["miner"].RawAddress, config, Gen)
	assert.NotNil(bc)
	assert.Equal(0, int(bc.height))
	fmt.Printf("Create blockchain pass, height = %d\n", bc.height)
	defer bc.Close()

	// verify Genesis block
	genesis, _ := bc.GetBlockByHeight(0)
	assert.NotNil(genesis)
	// serialize
	data, err := genesis.Serialize()
	assert.Nil(err)

	stream := genesis.ByteStream()
	assert.Equal(uint32(len(stream)), genesis.TranxsSize()+128)
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
	assert.Equal(4, int(bc.height))
}

func TestLoadBlockchainfromDB(t *testing.T) {
	defer os.Remove(testDBPath)
	assert := assert.New(t)

	config, err := config.LoadConfigWithPathWithoutValidation(testingConfigPath)
	assert.Nil(err)
	config.Chain.ChainDBPath = testDBPath
	// Disable block reward to make bookkeeping easier
	Gen.BlockReward = uint64(0)
	// Create a blockchain from scratch
	bc := CreateBlockchain(ta.Addrinfo["miner"].RawAddress, config, Gen)
	assert.NotNil(bc)
	fmt.Printf("Open blockchain pass, height = %d\n", bc.height)
	assert.Nil(addTestingBlocks(bc))
	bc.Close()

	// Load a blockchain from DB
	bc = CreateBlockchain(ta.Addrinfo["miner"].RawAddress, config, Gen)
	defer bc.Close()
	assert.NotNil(bc)

	// check hash<-->height mapping
	hash, err := bc.GetHashByHeight(0)
	assert.Nil(err)
	height, err := bc.GetHeightByHash(hash)
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

	blk, err = bc.GetBlockByHeight(60000)
	assert.Nil(blk)

	// add wrong blocks
	h := bc.TipHeight()
	hash = bc.TipHash()
	blk, err = bc.GetBlockByHeight(h)
	assert.Nil(err)
	assert.Equal(hash, blk.HashBlock())
	fmt.Printf("Current tip = %d hash = %x\n", h, hash)

	// add block with wrong height
	cbTx := NewCoinbaseTx(ta.Addrinfo["bravo"].RawAddress, 50, GenesisCoinbaseData)
	assert.NotNil(cbTx)
	blk = NewBlock(0, h+2, hash, []*Tx{cbTx})
	err = bc.ValidateBlock(blk)
	assert.NotNil(err)
	fmt.Printf("Cannot validate block %d: %v\n", blk.Height(), err)

	// add block with zero prev hash
	cbTx = NewCoinbaseTx(ta.Addrinfo["bravo"].RawAddress, 50, GenesisCoinbaseData)
	assert.NotNil(cbTx)
	blk = NewBlock(0, h+1, common.ZeroHash32B, []*Tx{cbTx})
	err = bc.ValidateBlock(blk)
	assert.NotNil(err)
	fmt.Printf("Cannot validate block %d: %v\n", blk.Height(), err)

	// cannot add existing block again
	blk, err = bc.GetBlockByHeight(3)
	assert.NotNil(blk)
	err = bc.commitBlock(blk)
	assert.NotNil(err)
	fmt.Printf("Cannot add block 3 again: %v\n", err)

	// read/write blocks from/to storage
	err = bc.StoreBlock(1, 4)
	assert.Nil(err)
	blk = bc.ReadBlock(1)
	assert.Equal(hash1, blk.HashBlock())
	fmt.Printf("Read block 1 hash match\n")
	blk = bc.ReadBlock(2)
	assert.Equal(hash2, blk.HashBlock())
	fmt.Printf("Read block 2 hash match\n")
	blk = bc.ReadBlock(3)
	assert.Equal(hash3, blk.HashBlock())
	fmt.Printf("Read block 3 hash match\n")
	blk = bc.ReadBlock(4)
	assert.Equal(hash4, blk.HashBlock())
	fmt.Printf("Read block 4 hash match\n")
}

func TestEmptyBlockOnlyHasCoinbaseTx(t *testing.T) {
	defer os.Remove(testDBPath)

	config, err := config.LoadConfigWithPathWithoutValidation(testingConfigPath)
	assert.Nil(t, err)
	config.Chain.ChainDBPath = testDBPath
	Gen.BlockReward = uint64(7777)

	bc := CreateBlockchain(ta.Addrinfo["miner"].RawAddress, config, Gen)
	defer bc.Close()
	assert.NotNil(t, bc)

	blk := bc.MintNewBlock([]*Tx{}, ta.Addrinfo["miner"], "")
	assert.Equal(t, uint64(1), blk.Height())
	assert.Equal(t, 1, len(blk.Tranxs))
	assert.True(t, blk.Tranxs[0].IsCoinbase())
	assert.Equal(t, uint32(1), blk.Tranxs[0].NumTxIn)
	assert.Equal(t, uint32(1), blk.Tranxs[0].NumTxOut)
	assert.Equal(t, uint64(7777), blk.Tranxs[0].TxOut[0].Value)
}
