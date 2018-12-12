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
	"testing"

	"github.com/facebookgo/clock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	"github.com/iotexproject/iotex-core/action/protocol/vote"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/pkg/hash"
	_hash "github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/state/factory"
	ta "github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/testutil"
)

const (
	testDBPath   = "db.test"
	testTriePath = "trie.test"
)

func addTestingTsfBlocks(bc Blockchain) error {
	// Add block 0
	tsf0, _ := action.NewTransfer(
		1,
		big.NewInt(3000000000),
		Gen.CreatorAddr(config.Default.Chain.ID),
		ta.Addrinfo["producer"].RawAddress,
		[]byte{}, uint64(100000),
		big.NewInt(10),
	)
	pk, _ := hex.DecodeString(Gen.CreatorPubKey)
	pubk, _ := keypair.BytesToPublicKey(pk)
	sig, _ := hex.DecodeString("ea18385718db2a8b92fa2dcdc158a05aae17fc7e800cc735850bdc99ab7b081f32fce70080888742d7b6bee892034e38298b69ce5fff671c18f0bed827e1e21f9da2ba5d8c434e00")
	tsf0.SetSrcPubkey(pubk)
	tsf0.SetSignature(sig)
	blk, err := bc.MintNewBlock([]action.Action{tsf0}, ta.Addrinfo["producer"],
		nil, nil, "")
	if err != nil {
		return err
	}
	if err := bc.ValidateBlock(blk, true); err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}
	// Add block 1
	// test --> A, B, C, D, E, F
	tsf1, _ := action.NewTransfer(1, big.NewInt(20), ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["alfa"].RawAddress, []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	_ = action.Sign(tsf1, ta.Addrinfo["producer"].PrivateKey)
	tsf2, _ := action.NewTransfer(2, big.NewInt(30), ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["bravo"].RawAddress, []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	_ = action.Sign(tsf2, ta.Addrinfo["producer"].PrivateKey)
	tsf3, _ := action.NewTransfer(3, big.NewInt(50), ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["charlie"].RawAddress, []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	_ = action.Sign(tsf3, ta.Addrinfo["producer"].PrivateKey)
	tsf4, _ := action.NewTransfer(4, big.NewInt(70), ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["delta"].RawAddress, []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	_ = action.Sign(tsf4, ta.Addrinfo["producer"].PrivateKey)
	tsf5, _ := action.NewTransfer(5, big.NewInt(110), ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["echo"].RawAddress, []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	_ = action.Sign(tsf5, ta.Addrinfo["producer"].PrivateKey)
	tsf6, _ := action.NewTransfer(6, big.NewInt(50<<20), ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["foxtrot"].RawAddress, []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	_ = action.Sign(tsf6, ta.Addrinfo["producer"].PrivateKey)

	blk, err = bc.MintNewBlock([]action.Action{tsf1, tsf2, tsf3, tsf4, tsf5, tsf6}, ta.Addrinfo["producer"], nil, nil, "")
	if err != nil {
		return err
	}
	if err := bc.ValidateBlock(blk, true); err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	// Add block 2
	// Charlie --> A, B, D, E, test
	tsf1, _ = action.NewTransfer(1, big.NewInt(1), ta.Addrinfo["charlie"].RawAddress, ta.Addrinfo["alfa"].RawAddress, []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	_ = action.Sign(tsf1, ta.Addrinfo["charlie"].PrivateKey)
	tsf2, _ = action.NewTransfer(2, big.NewInt(1), ta.Addrinfo["charlie"].RawAddress, ta.Addrinfo["bravo"].RawAddress, []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	_ = action.Sign(tsf2, ta.Addrinfo["charlie"].PrivateKey)
	tsf3, _ = action.NewTransfer(3, big.NewInt(1), ta.Addrinfo["charlie"].RawAddress, ta.Addrinfo["delta"].RawAddress, []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	_ = action.Sign(tsf3, ta.Addrinfo["charlie"].PrivateKey)
	tsf4, _ = action.NewTransfer(4, big.NewInt(1), ta.Addrinfo["charlie"].RawAddress, ta.Addrinfo["echo"].RawAddress, []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	_ = action.Sign(tsf4, ta.Addrinfo["charlie"].PrivateKey)
	tsf5, _ = action.NewTransfer(5, big.NewInt(1), ta.Addrinfo["charlie"].RawAddress, ta.Addrinfo["producer"].RawAddress, []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	_ = action.Sign(tsf5, ta.Addrinfo["charlie"].PrivateKey)
	blk, err = bc.MintNewBlock([]action.Action{tsf1, tsf2, tsf3, tsf4, tsf5}, ta.Addrinfo["producer"], nil, nil, "")
	if err != nil {
		return err
	}
	if err := bc.ValidateBlock(blk, true); err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	// Add block 3
	// Delta --> B, E, F, test
	tsf1, _ = action.NewTransfer(1, big.NewInt(1), ta.Addrinfo["delta"].RawAddress, ta.Addrinfo["bravo"].RawAddress, []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	_ = action.Sign(tsf1, ta.Addrinfo["delta"].PrivateKey)
	tsf2, _ = action.NewTransfer(2, big.NewInt(1), ta.Addrinfo["delta"].RawAddress, ta.Addrinfo["echo"].RawAddress, []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	_ = action.Sign(tsf2, ta.Addrinfo["delta"].PrivateKey)
	tsf3, _ = action.NewTransfer(3, big.NewInt(1), ta.Addrinfo["delta"].RawAddress, ta.Addrinfo["foxtrot"].RawAddress, []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	_ = action.Sign(tsf3, ta.Addrinfo["delta"].PrivateKey)
	tsf4, _ = action.NewTransfer(4, big.NewInt(1), ta.Addrinfo["delta"].RawAddress, ta.Addrinfo["producer"].RawAddress, []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	_ = action.Sign(tsf4, ta.Addrinfo["delta"].PrivateKey)
	blk, err = bc.MintNewBlock([]action.Action{tsf1, tsf2, tsf3, tsf4}, ta.Addrinfo["producer"], nil, nil, "")
	if err != nil {
		return err
	}
	if err := bc.ValidateBlock(blk, true); err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	// Add block 4
	// Delta --> A, B, C, D, F, test
	tsf1, _ = action.NewTransfer(1, big.NewInt(2), ta.Addrinfo["echo"].RawAddress, ta.Addrinfo["alfa"].RawAddress, []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	_ = action.Sign(tsf1, ta.Addrinfo["echo"].PrivateKey)
	tsf2, _ = action.NewTransfer(2, big.NewInt(2), ta.Addrinfo["echo"].RawAddress, ta.Addrinfo["bravo"].RawAddress, []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	_ = action.Sign(tsf2, ta.Addrinfo["echo"].PrivateKey)
	tsf3, _ = action.NewTransfer(3, big.NewInt(2), ta.Addrinfo["echo"].RawAddress, ta.Addrinfo["charlie"].RawAddress, []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	_ = action.Sign(tsf3, ta.Addrinfo["echo"].PrivateKey)
	tsf4, _ = action.NewTransfer(4, big.NewInt(2), ta.Addrinfo["echo"].RawAddress, ta.Addrinfo["delta"].RawAddress, []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	_ = action.Sign(tsf4, ta.Addrinfo["echo"].PrivateKey)
	tsf5, _ = action.NewTransfer(5, big.NewInt(2), ta.Addrinfo["echo"].RawAddress, ta.Addrinfo["foxtrot"].RawAddress, []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	_ = action.Sign(tsf5, ta.Addrinfo["echo"].PrivateKey)
	tsf6, _ = action.NewTransfer(6, big.NewInt(2), ta.Addrinfo["echo"].RawAddress, ta.Addrinfo["producer"].RawAddress, []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	_ = action.Sign(tsf6, ta.Addrinfo["echo"].PrivateKey)
	vote1, _ := action.NewVote(6, ta.Addrinfo["charlie"].RawAddress, ta.Addrinfo["charlie"].RawAddress,
		testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	vote2, _ := action.NewVote(1, ta.Addrinfo["alfa"].RawAddress, ta.Addrinfo["alfa"].RawAddress,
		testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err := action.Sign(vote1, ta.Addrinfo["charlie"].PrivateKey); err != nil {
		return err
	}
	if err := action.Sign(vote2, ta.Addrinfo["alfa"].PrivateKey); err != nil {
		return err
	}

	blk, err = bc.MintNewBlock([]action.Action{tsf1, tsf2, tsf3, tsf4, tsf5, tsf6, vote1, vote2},
		ta.Addrinfo["producer"], nil, nil, "")
	if err != nil {
		return err
	}
	if err := bc.ValidateBlock(blk, true); err != nil {
		return err
	}
	return bc.CommitBlock(blk)
}

func TestCreateBlockchain(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	cfg := config.Default
	// disable account-based testing
	cfg.Chain.TrieDBPath = ""
	// Disable block reward to make bookkeeping easier
	Gen.BlockReward = big.NewInt(0)

	// create chain
	bc := NewBlockchain(cfg, InMemStateFactoryOption(), InMemDaoOption())
	bc.Validator().AddActionValidators(protocol.NewGenericValidator(bc), account.NewProtocol(), vote.NewProtocol(bc))
	bc.GetFactory().AddActionHandlers(account.NewProtocol(), vote.NewProtocol(bc))
	require.NoError(bc.Start(ctx))
	require.NotNil(bc)
	height := bc.TipHeight()
	require.Equal(0, int(height))
	fmt.Printf("Create blockchain pass, height = %d\n", height)
	defer func() {
		err := bc.Stop(ctx)
		require.NoError(err)
	}()

	// verify Genesis block
	genesis, _ := bc.GetBlockByHeight(0)
	require.NotNil(genesis)
	// serialize
	data, err := genesis.Serialize()
	require.Nil(err)

	transfers, votes, _ := action.ClassifyActions(genesis.Actions)
	require.Equal(0, len(transfers))
	require.Equal(21, len(votes))

	fmt.Printf("Block size match pass\n")
	fmt.Printf("Marshaling Block pass\n")

	// deserialize
	deserialize := Block{}
	err = deserialize.Deserialize(data)
	require.Nil(err)
	fmt.Printf("Unmarshaling Block pass\n")

	hash := genesis.HashBlock()
	require.Equal(hash, deserialize.HashBlock())
	fmt.Printf("Serialize/Deserialize Block hash = %x match\n", hash)

	hash = genesis.CalculateTxRoot()
	require.Equal(hash, deserialize.CalculateTxRoot())
	fmt.Printf("Serialize/Deserialize Block merkle = %x match\n", hash)

	// add 4 sample blocks
	require.Nil(addTestingTsfBlocks(bc))
	height = bc.TipHeight()
	require.Equal(5, int(height))
}

type MockSubscriber struct {
	counter int
}

func (ms *MockSubscriber) HandleBlock(blk *Block) error {
	tsfs, _, _ := action.ClassifyActions(blk.Actions)
	ms.counter += len(tsfs)
	return nil
}

func TestLoadBlockchainfromDB(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)
	defer testutil.CleanupPath(t, testDBPath)

	// Disable block reward to make bookkeeping easier
	Gen.BlockReward = big.NewInt(0)

	cfg := config.Default
	cfg.DB.UseBadgerDB = true // test with badgerDB
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Explorer.Enabled = true

	sf, err := factory.NewFactory(cfg, factory.DefaultTrieOption())
	require.Nil(err)
	sf.AddActionHandlers(account.NewProtocol())
	require.NoError(sf.Start(context.Background()))
	require.NoError(addCreatorToFactory(sf))

	// Create a blockchain from scratch
	bc := NewBlockchain(cfg, PrecreatedStateFactoryOption(sf), BoltDBDaoOption())
	bc.Validator().AddActionValidators(protocol.NewGenericValidator(bc), account.NewProtocol(), vote.NewProtocol(bc))
	sf.AddActionHandlers(vote.NewProtocol(bc))
	require.NoError(bc.Start(ctx))
	require.NotNil(bc)

	ms := &MockSubscriber{counter: 0}
	err = bc.AddSubscriber(ms)
	require.Nil(err)
	require.Equal(0, ms.counter)

	height := bc.TipHeight()
	fmt.Printf("Open blockchain pass, height = %d\n", height)
	require.Nil(addTestingTsfBlocks(bc))
	err = bc.Stop(ctx)
	require.NoError(err)
	require.Equal(27, ms.counter)

	// Load a blockchain from DB
	bc = NewBlockchain(cfg, PrecreatedStateFactoryOption(sf), BoltDBDaoOption())
	bc.Validator().AddActionValidators(protocol.NewGenericValidator(bc), account.NewProtocol(), vote.NewProtocol(bc))
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

	hash5, err := bc.GetHashByHeight(5)
	require.Nil(err)
	height, err = bc.GetHeightByHash(hash5)
	require.Nil(err)
	require.Equal(uint64(5), height)
	blk, err = bc.GetBlockByHash(hash5)
	require.Nil(err)
	require.Equal(hash5, blk.HashBlock())
	fmt.Printf("block 5 hash = %x\n", hash5)

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
	cbTsf := action.NewCoinBaseTransfer(1, big.NewInt(50), ta.Addrinfo["bravo"].RawAddress)
	require.NotNil(cbTsf)
	blk = NewBlock(0, h+2, hash, testutil.TimestampNow(), ta.Addrinfo["bravo"].PublicKey, []action.Action{cbTsf})
	err = bc.ValidateBlock(blk, true)
	require.NotNil(err)
	fmt.Printf("Cannot validate block %d: %v\n", blk.Height(), err)

	// add block with zero prev hash
	cbTsf2 := action.NewCoinBaseTransfer(1, big.NewInt(50), ta.Addrinfo["bravo"].RawAddress)
	require.NotNil(cbTsf2)
	blk = NewBlock(
		0,
		h+1,
		_hash.ZeroHash32B,
		testutil.TimestampNow(),
		ta.Addrinfo["bravo"].PublicKey,
		[]action.Action{cbTsf2},
	)
	err = bc.ValidateBlock(blk, true)
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
	blk, err = bc.GetBlockByHeight(5)
	require.Nil(err)
	require.Equal(hash5, blk.HashBlock())
	tsfs, votes, _ := action.ClassifyActions(blk.Actions)
	for _, transfer := range tsfs {
		transferHash := transfer.Hash()
		hash, err := bc.GetBlockHashByTransferHash(transferHash)
		require.Nil(err)
		require.Equal(hash, hash5)
		transfer1, err := bc.GetTransferByTransferHash(transferHash)
		require.Nil(err)
		require.Equal(transfer1.Hash(), transferHash)
	}

	for _, vote := range votes {
		voteHash := vote.Hash()
		hash, err := bc.GetBlockHashByVoteHash(voteHash)
		require.Nil(err)
		require.Equal(hash, hash5)
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
	require.Equal(totalTransfers, uint64(27))

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
	Gen.BlockReward = big.NewInt(0)
	cfg := config.Default
	cfg.DB.UseBadgerDB = false // test with boltDB
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	sf, err := factory.NewFactory(cfg, factory.DefaultTrieOption())
	require.Nil(err)
	sf.AddActionHandlers(account.NewProtocol())
	require.NoError(sf.Start(context.Background()))
	require.NoError(addCreatorToFactory(sf))
	// Create a blockchain from scratch
	bc := NewBlockchain(cfg, PrecreatedStateFactoryOption(sf), BoltDBDaoOption())
	bc.Validator().AddActionValidators(protocol.NewGenericValidator(bc), account.NewProtocol(), vote.NewProtocol(bc))
	sf.AddActionHandlers(vote.NewProtocol(bc))
	require.NoError(bc.Start(ctx))
	require.NotNil(bc)

	ms := &MockSubscriber{counter: 0}
	err = bc.AddSubscriber(ms)
	require.Nil(err)
	require.Equal(0, ms.counter)
	err = bc.RemoveSubscriber(ms)
	require.Nil(err)

	height := bc.TipHeight()
	fmt.Printf("Open blockchain pass, height = %d\n", height)
	require.Nil(addTestingTsfBlocks(bc))
	err = bc.Stop(ctx)
	require.NoError(err)
	require.Equal(0, ms.counter)

	// Load a blockchain from DB
	bc = NewBlockchain(cfg, PrecreatedStateFactoryOption(sf), BoltDBDaoOption())
	bc.Validator().AddActionValidators(protocol.NewGenericValidator(bc), account.NewProtocol(), vote.NewProtocol(bc))
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
	cbTsf := action.NewCoinBaseTransfer(1, big.NewInt(50), ta.Addrinfo["bravo"].RawAddress)
	require.NotNil(cbTsf)
	blk = NewBlock(0, h+2, hash, testutil.TimestampNow(), ta.Addrinfo["bravo"].PublicKey, []action.Action{cbTsf})
	err = bc.ValidateBlock(blk, true)
	require.NotNil(err)
	fmt.Printf("Cannot validate block %d: %v\n", blk.Height(), err)
	// add block with zero prev hash
	cbTsf2 := action.NewCoinBaseTransfer(1, big.NewInt(50), ta.Addrinfo["bravo"].RawAddress)
	require.NotNil(cbTsf2)
	blk = NewBlock(
		0,
		h+1,
		_hash.ZeroHash32B,
		testutil.TimestampNow(),
		ta.Addrinfo["bravo"].PublicKey,
		[]action.Action{cbTsf2},
	)
	err = bc.ValidateBlock(blk, true)
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
	tsfs, votes, _ := action.ClassifyActions(blk.Actions)
	for _, transfer := range tsfs {
		transferHash := transfer.Hash()
		_, err := bc.GetBlockHashByTransferHash(transferHash)
		require.NotNil(err)
		_, err = bc.GetTransferByTransferHash(transferHash)
		require.NotNil(err)
	}
	for _, vote := range votes {
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
	bc := NewBlockchain(cfg, InMemDaoOption(), InMemStateFactoryOption())
	require.NoError(t, bc.Start(ctx))
	defer func() {
		err := bc.Stop(ctx)
		require.Nil(t, err)
	}()
	require.NotNil(t, bc)

	val := bc.Validator()
	require.NotNil(t, bc)
	bc.SetValidator(val)
	require.NotNil(t, bc.Validator())
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
	Gen.BlockReward = big.NewInt(0)

	sf, err := factory.NewFactory(cfg, factory.DefaultTrieOption())
	require.Nil(err)
	sf.AddActionHandlers(account.NewProtocol(), vote.NewProtocol(nil))
	require.NoError(sf.Start(context.Background()))
	bc := NewBlockchain(cfg, PrecreatedStateFactoryOption(sf), BoltDBDaoOption())
	require.NoError(bc.Start(context.Background()))
	require.NotNil(bc)
	// TODO: change the value when Candidates size is changed
	height, err := sf.Height()
	require.NoError(err)
	require.Equal(uint64(0), height)
	candidate, err := sf.CandidatesByHeight(height)
	require.NoError(err)
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

	sf, err := factory.NewFactory(cfg, factory.DefaultTrieOption())
	require.Nil(err)
	require.NoError(sf.Start(context.Background()))
	require.NoError(addCreatorToFactory(sf))

	Gen.BlockReward = big.NewInt(0)
	bc := NewBlockchain(cfg, PrecreatedStateFactoryOption(sf), BoltDBDaoOption())
	bc.Validator().AddActionValidators(protocol.NewGenericValidator(bc), account.NewProtocol())
	require.NoError(bc.Start(context.Background()))
	require.NotNil(bc)
	height := bc.TipHeight()
	require.Equal(0, int(height))

	actions := []action.Action{}
	blk, err := bc.MintNewBlock(actions, ta.Addrinfo["producer"], nil, nil, "")
	require.Nil(err)
	s, err := bc.StateByAddr(ta.Addrinfo["producer"].RawAddress)
	require.Nil(err)
	b := s.Balance
	require.True(b.String() == Gen.TotalSupply.String())
	require.Nil(bc.ValidateBlock(blk, true))
	require.Nil(bc.CommitBlock(blk))
	height = bc.TipHeight()
	require.True(height == 1)
	require.True(len(blk.Actions) == 1)
	s, err = bc.StateByAddr(ta.Addrinfo["producer"].RawAddress)
	require.Nil(err)
	b = s.Balance
	expectedBalance := big.NewInt(0)
	expectedBalance.Add(Gen.TotalSupply, Gen.BlockReward)
	require.True(b.String() == expectedBalance.String())
}

func TestBlockchain_StateByAddr(t *testing.T) {
	require := require.New(t)

	cfg := config.Default
	// disable account-based testing
	// create chain
	bc := NewBlockchain(cfg, InMemDaoOption(), InMemStateFactoryOption())
	require.NoError(bc.Start(context.Background()))
	require.NotNil(bc)

	s, err := bc.StateByAddr(Gen.CreatorAddr(cfg.Chain.ID))
	require.NoError(err)
	require.Equal(uint64(0), s.Nonce)
	bal := big.NewInt(7700000000)
	require.Equal(bal.Mul(bal, big.NewInt(1e18)).String(), s.Balance.String())
	require.Equal(hash.ZeroHash32B, s.Root)
	require.Equal([]byte(nil), s.CodeHash)
	require.Equal(false, s.IsCandidate)
	require.Equal(big.NewInt(0), s.VotingWeight)
	require.Equal("", s.Votee)
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

	sf, _ := factory.NewFactory(cfg, factory.InMemTrieOption())
	require.NoError(sf.Start(context.Background()))
	require.NoError(addCreatorToFactory(sf))

	// Create a blockchain from scratch
	bc := NewBlockchain(cfg, PrecreatedStateFactoryOption(sf), BoltDBDaoOption())
	require.NoError(bc.Start(context.Background()))
	a := ta.Addrinfo["alfa"]
	c := ta.Addrinfo["bravo"]
	ws, err := sf.NewWorkingSet()
	require.NoError(err)
	_, err = account.LoadOrCreateAccount(ws, a.RawAddress, big.NewInt(100000))
	require.NoError(err)
	_, err = account.LoadOrCreateAccount(ws, c.RawAddress, big.NewInt(100000))
	require.NoError(err)
	gasLimit := testutil.TestGasLimit
	ctx := protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			ProducerAddr:    ta.Addrinfo["producer"].RawAddress,
			GasLimit:        &gasLimit,
			EnableGasCharge: testutil.EnableGasCharge,
		})
	_, _, err = ws.RunActions(ctx, 0, nil)
	require.NoError(err)
	require.NoError(sf.Commit(ws))

	for i := 0; i < 10; i++ {
		acts := []action.Action{}
		for i := 0; i < 1000; i++ {
			tsf, err := action.NewTransfer(1, big.NewInt(2), a.RawAddress, c.RawAddress, []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
			require.NoError(err)
			_ = action.Sign(tsf, a.PrivateKey)
			acts = append(acts, tsf)
		}
		blk, _ := bc.MintNewBlock(acts, ta.Addrinfo["producer"], nil, nil, "")
		require.Nil(bc.ValidateBlock(blk, true))
		require.Nil(bc.CommitBlock(blk))
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

	sf, _ := factory.NewFactory(cfg, factory.InMemTrieOption())
	require.NoError(sf.Start(context.Background()))
	require.NoError(addCreatorToFactory(sf))

	// Create a blockchain from scratch
	bc := NewBlockchain(cfg, PrecreatedStateFactoryOption(sf), BoltDBDaoOption())
	require.NoError(bc.Start(context.Background()))
	a := ta.Addrinfo["alfa"]
	c := ta.Addrinfo["bravo"]
	ws, err := sf.NewWorkingSet()
	require.NoError(err)
	_, err = account.LoadOrCreateAccount(ws, a.RawAddress, big.NewInt(100000))
	require.NoError(err)
	_, err = account.LoadOrCreateAccount(ws, c.RawAddress, big.NewInt(100000))
	require.NoError(err)
	gasLimit := testutil.TestGasLimit
	ctx := protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			ProducerAddr:    ta.Addrinfo["producer"].RawAddress,
			GasLimit:        &gasLimit,
			EnableGasCharge: testutil.EnableGasCharge,
		})
	_, _, err = ws.RunActions(ctx, 0, nil)
	require.NoError(err)
	require.NoError(sf.Commit(ws))

	val := validator{sf: sf, validatorAddr: ""}
	val.AddActionValidators(protocol.NewGenericValidator(bc), account.NewProtocol(), vote.NewProtocol(bc))
	acts := []action.Action{}
	for i := 0; i < 5000; i++ {
		tsf, err := action.NewTransfer(1, big.NewInt(2), a.RawAddress, c.RawAddress, []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
		require.NoError(err)
		_ = action.Sign(tsf, a.PrivateKey)
		acts = append(acts, tsf)

		vote, err := action.NewVote(1, a.RawAddress, a.RawAddress, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
		require.NoError(err)
		_ = action.Sign(vote, a.PrivateKey)
		acts = append(acts, vote)
	}
	blk, _ := bc.MintNewBlock(acts, ta.Addrinfo["producer"], nil,
		nil, "")
	require.Nil(val.Validate(blk, 0, blk.PrevHash(), true))
}

func TestMintDKGBlock(t *testing.T) {
	require := require.New(t)
	lastSeed, _ := hex.DecodeString("9de6306b08158c423330f7a27243a1a5cbe39bfd764f07818437882d21241567")
	cfg := config.Default
	clk := clock.NewMock()
	chain := NewBlockchain(cfg, InMemDaoOption(), InMemStateFactoryOption(), ClockOption(clk))
	chain.Validator().AddActionValidators(protocol.NewGenericValidator(chain), account.NewProtocol())
	chain.GetFactory().AddActionHandlers(account.NewProtocol(), vote.NewProtocol(chain))
	require.NoError(chain.Start(context.Background()))

	var err error
	const numNodes = 21
	addresses := make([]string, numNodes)
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
	ec283PKList := make([]keypair.PublicKey, numNodes)
	ec283SKList := make([]keypair.PrivateKey, numNodes)

	// Generate 21 identifiers for the delegates
	for i := 0; i < numNodes; i++ {
		var err error
		ec283PKList[i], ec283SKList[i], err = crypto.EC283.NewKeyPair()
		require.NoError(err)
		pkHash := keypair.HashPubKey(ec283PKList[i])
		addresses[i] = address.New(cfg.Chain.ID, pkHash[:]).IotxAddress()
		idList[i] = hash.Hash256b([]byte(addresses[i]))
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
			result, err := crypto.DKG.ShareVerify(idList[i], sharesList[j][i], witnessesList[j])
			require.NoError(err)
			require.True(result)
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
	for i := 1; i < numNodes; i++ {
		iotxAddr := iotxaddress.Address{
			PublicKey:  ec283PKList[i],
			PrivateKey: ec283SKList[i],
			RawAddress: addresses[i],
		}
		blk, err := chain.MintNewBlock(nil, &iotxAddr,
			&iotxaddress.DKGAddress{PrivateKey: askList[i], PublicKey: pkList[i], ID: idList[i]}, lastSeed, "")
		require.NoError(err)
		require.NoError(chain.ValidateBlock(blk, true))
		require.NoError(chain.CommitBlock(blk))
		require.Equal(pkList[i], blk.Header.DKGPubkey)
		require.Equal(idList[i], blk.Header.DKGID)
		require.True(len(blk.Header.DKGBlockSig) > 0)
	}
	height, err := chain.GetFactory().Height()
	require.NoError(err)
	require.Equal(uint64(20), height)
	candidates, err := chain.CandidatesByHeight(height)
	require.NoError(err)
	require.Equal(21, len(candidates))
}
