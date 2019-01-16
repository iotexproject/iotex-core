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
	"sync"
	"testing"

	"github.com/facebookgo/clock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	"github.com/iotexproject/iotex-core/action/protocol/vote"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/pkg/hash"
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
		ta.Addrinfo["producer"].Bech32(),
		[]byte{}, uint64(100000),
		big.NewInt(10),
	)
	pubk, _ := keypair.DecodePublicKey(Gen.CreatorPubKey)
	sig, _ := hex.DecodeString("b14516e888e78e8dcba9af59607c8deea404f52c3df706a0bb6e1fadfda58113983b9201400ff75202fe486363a2e99052b3866595be932542bc0b860278b0753ab3fd2c66764a01")
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetAction(tsf0).
		SetDestinationAddress(ta.Addrinfo["producer"].Bech32()).
		SetNonce(1).
		SetGasLimit(100000).
		SetGasPrice(big.NewInt(10)).Build()

	selp := action.AssembleSealedEnvelope(elp, Gen.CreatorAddr(config.Default.Chain.ID), pubk, sig)

	blk, err := bc.MintNewBlock([]action.SealedEnvelope{selp}, ta.Keyinfo["producer"].PubKey,
		ta.Keyinfo["producer"].PriKey, ta.Addrinfo["producer"].Bech32(), nil, nil, "")
	if err != nil {
		return err
	}
	if err := bc.ValidateBlock(blk, true); err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	addr0 := ta.Addrinfo["producer"].Bech32()
	priKey0 := ta.Keyinfo["producer"].PriKey
	addr1 := ta.Addrinfo["alfa"].Bech32()
	priKey1 := ta.Keyinfo["alfa"].PriKey
	addr2 := ta.Addrinfo["bravo"].Bech32()
	addr3 := ta.Addrinfo["charlie"].Bech32()
	priKey3 := ta.Keyinfo["charlie"].PriKey
	addr4 := ta.Addrinfo["delta"].Bech32()
	priKey4 := ta.Keyinfo["delta"].PriKey
	addr5 := ta.Addrinfo["echo"].Bech32()
	priKey5 := ta.Keyinfo["echo"].PriKey
	addr6 := ta.Addrinfo["foxtrot"].Bech32()
	// Add block 1
	// test --> A, B, C, D, E, F
	tsf1, err := testutil.SignedTransfer(addr0, addr1, priKey0, 1, big.NewInt(20), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf2, err := testutil.SignedTransfer(addr0, addr2, priKey0, 2, big.NewInt(30), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf3, err := testutil.SignedTransfer(addr0, addr3, priKey0, 3, big.NewInt(50), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf4, err := testutil.SignedTransfer(addr0, addr4, priKey0, 4, big.NewInt(70), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf5, err := testutil.SignedTransfer(addr0, addr5, priKey0, 5, big.NewInt(110), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf6, err := testutil.SignedTransfer(addr0, addr6, priKey0, 6, big.NewInt(50<<20), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}

	blk, err = bc.MintNewBlock([]action.SealedEnvelope{tsf1, tsf2, tsf3, tsf4, tsf5, tsf6},
		ta.Keyinfo["producer"].PubKey, ta.Keyinfo["producer"].PriKey,
		ta.Addrinfo["producer"].Bech32(), nil, nil, "")
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
	tsf1, err = testutil.SignedTransfer(addr3, addr1, priKey3, 1, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf2, err = testutil.SignedTransfer(addr3, addr2, priKey3, 2, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf3, err = testutil.SignedTransfer(addr3, addr4, priKey3, 3, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf4, err = testutil.SignedTransfer(addr3, addr5, priKey3, 4, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf5, err = testutil.SignedTransfer(addr3, addr0, priKey3, 5, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	blk, err = bc.MintNewBlock([]action.SealedEnvelope{tsf1, tsf2, tsf3, tsf4, tsf5},
		ta.Keyinfo["producer"].PubKey, ta.Keyinfo["producer"].PriKey,
		ta.Addrinfo["producer"].Bech32(), nil, nil, "")
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
	tsf1, err = testutil.SignedTransfer(addr4, addr2, priKey4, 1, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf2, err = testutil.SignedTransfer(addr4, addr5, priKey4, 2, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf3, err = testutil.SignedTransfer(addr4, addr6, priKey4, 3, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf4, err = testutil.SignedTransfer(addr4, addr0, priKey4, 4, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	blk, err = bc.MintNewBlock([]action.SealedEnvelope{tsf1, tsf2, tsf3, tsf4}, ta.Keyinfo["producer"].PubKey,
		ta.Keyinfo["producer"].PriKey, ta.Addrinfo["producer"].Bech32(), nil, nil, "")
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
	tsf1, err = testutil.SignedTransfer(addr5, addr1, priKey5, 1, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf2, err = testutil.SignedTransfer(addr5, addr2, priKey5, 2, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf3, err = testutil.SignedTransfer(addr5, addr3, priKey5, 3, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf4, err = testutil.SignedTransfer(addr5, addr4, priKey5, 4, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf5, err = testutil.SignedTransfer(addr5, addr6, priKey5, 5, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf6, err = testutil.SignedTransfer(addr5, addr0, priKey5, 6, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	vote1, err := testutil.SignedVote(addr3, addr3, priKey3, 6, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	vote2, err := testutil.SignedVote(addr1, addr1, priKey1, 1, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}

	blk, err = bc.MintNewBlock([]action.SealedEnvelope{tsf1, tsf2, tsf3, tsf4, tsf5, tsf6, vote1, vote2},
		ta.Keyinfo["producer"].PubKey, ta.Keyinfo["producer"].PriKey,
		ta.Addrinfo["producer"].Bech32(), nil, nil, "")
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

	// create chain
	bc := NewBlockchain(cfg, InMemStateFactoryOption(), InMemDaoOption())
	bc.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(bc))
	bc.Validator().AddActionValidators(account.NewProtocol(), vote.NewProtocol(bc))
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
	deserialize := block.Block{}
	err = deserialize.Deserialize(data)
	require.Nil(err)
	fmt.Printf("Unmarshaling Block pass\n")

	blkhash := genesis.HashBlock()
	require.Equal(blkhash, deserialize.HashBlock())
	fmt.Printf("Serialize/Deserialize Block hash = %x match\n", blkhash)

	blkhash = genesis.CalculateTxRoot()
	require.Equal(blkhash, deserialize.CalculateTxRoot())
	fmt.Printf("Serialize/Deserialize Block merkle = %x match\n", blkhash)

	// add 4 sample blocks
	require.Nil(addTestingTsfBlocks(bc))
	height = bc.TipHeight()
	require.Equal(5, int(height))
}

func TestBlockchain_MintNewBlock(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default
	bc := NewBlockchain(cfg, InMemStateFactoryOption(), InMemDaoOption())
	bc.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(bc))
	bc.Validator().AddActionValidators(account.NewProtocol(), vote.NewProtocol(bc))
	bc.GetFactory().AddActionHandlers(account.NewProtocol(), vote.NewProtocol(bc))
	require.NoError(t, bc.Start(ctx))
	defer require.NoError(t, bc.Stop(ctx))

	pk, _ := keypair.DecodePublicKey(Gen.CreatorPubKey)
	// The signature should only matches the transfer amount 3000000000
	sig, err := hex.DecodeString("0ea4152e8bef2049319d945ac55d21093f9a6e9e4793abf8906e1b9a9f59de1123db31006b3574cb080b7efd623101cb6b175bb881d7361e2341a740cfb8c3db1be1dd3aa4b67901")
	require.NoError(t, err)

	cases := make(map[int64]bool)
	cases[0] = true
	cases[1] = false
	for k, v := range cases {
		tsf, err := action.NewTransfer(
			1,
			big.NewInt(3000000000+k),
			Gen.CreatorAddr(config.Default.Chain.ID),
			ta.Addrinfo["producer"].Bech32(),
			[]byte{}, uint64(100000),
			big.NewInt(10),
		)
		require.NoError(t, err)
		bd := &action.EnvelopeBuilder{}
		elp := bd.SetAction(tsf).
			SetDestinationAddress(ta.Addrinfo["producer"].Bech32()).
			SetNonce(1).
			SetGasLimit(100000).
			SetGasPrice(big.NewInt(10)).Build()

		selp := action.AssembleSealedEnvelope(elp, Gen.CreatorAddr(config.Default.Chain.ID), pk, sig)
		_, err = bc.MintNewBlock([]action.SealedEnvelope{selp}, ta.Keyinfo["producer"].PubKey,
			ta.Keyinfo["producer"].PriKey, ta.Addrinfo["producer"].Bech32(), nil,
			nil, "")
		if v {
			require.NoError(t, err)
		} else {
			require.Error(t, err)
		}
	}

}

type MockSubscriber struct {
	counter int
	mu      sync.RWMutex
}

func (ms *MockSubscriber) HandleBlock(blk *block.Block) error {
	ms.mu.Lock()
	tsfs, _, _ := action.ClassifyActions(blk.Actions)
	ms.counter += len(tsfs)
	ms.mu.Unlock()
	return nil
}

func (ms *MockSubscriber) Counter() int {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	return ms.counter
}

func TestLoadBlockchainfromDB(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)
	defer testutil.CleanupPath(t, testDBPath)

	cfg := config.Default
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
	bc.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(bc))
	bc.Validator().AddActionValidators(account.NewProtocol(), vote.NewProtocol(bc))
	sf.AddActionHandlers(vote.NewProtocol(bc))
	require.NoError(bc.Start(ctx))
	require.NotNil(bc)

	ms := &MockSubscriber{counter: 0}
	err = bc.AddSubscriber(ms)
	require.Nil(err)
	require.Equal(0, ms.Counter())

	height := bc.TipHeight()
	fmt.Printf("Open blockchain pass, height = %d\n", height)
	require.Nil(addTestingTsfBlocks(bc))
	err = bc.Stop(ctx)
	require.NoError(err)
	require.Equal(27, ms.Counter())

	// Load a blockchain from DB
	bc = NewBlockchain(cfg, PrecreatedStateFactoryOption(sf), BoltDBDaoOption())
	bc.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(bc))
	bc.Validator().AddActionValidators(account.NewProtocol(), vote.NewProtocol(bc))
	require.NoError(bc.Start(ctx))
	defer func() {
		err := bc.Stop(ctx)
		require.NoError(err)
	}()
	require.NotNil(bc)

	// check hash<-->height mapping
	blkhash, err := bc.GetHashByHeight(0)
	require.Nil(err)
	height, err = bc.GetHeightByHash(blkhash)
	require.Nil(err)
	require.Equal(uint64(0), height)
	blk, err := bc.GetBlockByHash(blkhash)
	require.Nil(err)
	require.Equal(blkhash, blk.HashBlock())
	fmt.Printf("Genesis hash = %x\n", blkhash)

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

	empblk, err := bc.GetBlockByHash(hash.ZeroHash32B)
	require.Nil(empblk)
	require.NotNil(err.Error())

	blk, err = bc.GetBlockByHeight(60000)
	require.Nil(blk)
	require.NotNil(err)

	// add wrong blocks
	h := bc.TipHeight()
	blkhash = bc.TipHash()
	blk, err = bc.GetBlockByHeight(h)
	require.Nil(err)
	require.Equal(blkhash, blk.HashBlock())
	fmt.Printf("Current tip = %d hash = %x\n", h, blkhash)

	// add block with wrong height
	cbTsf := action.NewCoinBaseTransfer(1, big.NewInt(50), ta.Addrinfo["bravo"].Bech32())
	require.NotNil(cbTsf)
	bd := action.EnvelopeBuilder{}
	elp := bd.SetNonce(1).
		SetDestinationAddress(ta.Addrinfo["bravo"].Bech32()).
		SetGasLimit(genesis.ActionGasLimit).
		SetAction(cbTsf).Build()
	selp, err := action.Sign(elp, ta.Addrinfo["bravo"].Bech32(), ta.Keyinfo["bravo"].PriKey)
	require.NoError(err)

	nblk, err := block.NewTestingBuilder().
		SetChainID(0).
		SetHeight(h+2).
		SetPrevBlockHash(blkhash).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(selp).SignAndBuild(ta.Keyinfo["bravo"].PubKey, ta.Keyinfo["bravo"].PriKey)
	require.NoError(err)

	err = bc.ValidateBlock(&nblk, true)
	require.NotNil(err)
	fmt.Printf("Cannot validate block %d: %v\n", blk.Height(), err)

	// add block with zero prev hash
	cbTsf2 := action.NewCoinBaseTransfer(1, big.NewInt(50), ta.Addrinfo["bravo"].Bech32())
	require.NotNil(cbTsf2)
	bd = action.EnvelopeBuilder{}
	elp = bd.SetNonce(1).
		SetDestinationAddress(ta.Addrinfo["bravo"].Bech32()).
		SetGasLimit(genesis.ActionGasLimit).
		SetAction(cbTsf2).Build()
	selp2, err := action.Sign(elp, ta.Addrinfo["bravo"].Bech32(), ta.Keyinfo["bravo"].PriKey)
	require.NoError(err)

	nblk, err = block.NewTestingBuilder().
		SetChainID(0).
		SetHeight(h+1).
		SetPrevBlockHash(hash.ZeroHash32B).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(selp2).SignAndBuild(ta.Keyinfo["bravo"].PubKey, ta.Keyinfo["bravo"].PriKey)
	require.NoError(err)
	err = bc.ValidateBlock(&nblk, true)
	require.NotNil(err)
	fmt.Printf("Cannot validate block %d: %v\n", blk.Height(), err)

	// add existing block again will have no effect
	blk, err = bc.GetBlockByHeight(3)
	require.NotNil(blk)
	require.Nil(err)
	require.NoError(bc.(*blockchain).commitBlock(blk))
	fmt.Printf("Cannot add block 3 again: %v\n", err)

	// check all Tx from block 4
	blk, err = bc.GetBlockByHeight(5)
	require.Nil(err)
	require.Equal(hash5, blk.HashBlock())
	tsfs, votes, _ := action.ClassifyActions(blk.Actions)
	for _, transfer := range tsfs {
		transferHash := transfer.Hash()
		blkhash, err := bc.GetBlockHashByTransferHash(transferHash)
		require.Nil(err)
		require.Equal(blkhash, hash5)
		transfer1, err := bc.GetTransferByTransferHash(transferHash)
		require.Nil(err)
		require.Equal(transfer1.Hash(), transferHash)
	}

	for _, vote := range votes {
		voteHash := vote.Hash()
		blkhash, err := bc.GetBlockHashByVoteHash(voteHash)
		require.Nil(err)
		require.Equal(blkhash, hash5)
		vote1, err := bc.GetVoteByVoteHash(voteHash)
		require.Nil(err)
		require.Equal(vote1.Hash(), voteHash)
	}

	fromTransfers, err := bc.GetTransfersFromAddress(ta.Addrinfo["charlie"].Bech32())
	require.Nil(err)
	require.Equal(len(fromTransfers), 5)

	toTransfers, err := bc.GetTransfersToAddress(ta.Addrinfo["charlie"].Bech32())
	require.Nil(err)
	require.Equal(len(toTransfers), 2)

	fromVotes, err := bc.GetVotesFromAddress(ta.Addrinfo["charlie"].Bech32())
	require.Nil(err)
	require.Equal(len(fromVotes), 1)

	fromVotes, err = bc.GetVotesFromAddress(ta.Addrinfo["alfa"].Bech32())
	require.Nil(err)
	require.Equal(len(fromVotes), 1)

	toVotes, err := bc.GetVotesToAddress(ta.Addrinfo["charlie"].Bech32())
	require.Nil(err)
	require.Equal(len(toVotes), 1)

	toVotes, err = bc.GetVotesToAddress(ta.Addrinfo["alfa"].Bech32())
	require.Nil(err)
	require.Equal(len(toVotes), 1)

	totalTransfers, err := bc.GetTotalTransfers()
	require.Nil(err)
	require.Equal(totalTransfers, uint64(27))

	totalVotes, err := bc.GetTotalVotes()
	require.Nil(err)
	require.Equal(totalVotes, uint64(23))

	_, err = bc.GetTransferByTransferHash(hash.ZeroHash32B)
	require.NotNil(err)
	_, err = bc.GetVoteByVoteHash(hash.ZeroHash32B)
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
	bc.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(bc))
	bc.Validator().AddActionValidators(account.NewProtocol(), vote.NewProtocol(bc))
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
	bc.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(bc))
	bc.Validator().AddActionValidators(account.NewProtocol(), vote.NewProtocol(bc))
	require.NoError(bc.Start(ctx))
	defer func() {
		err := bc.Stop(ctx)
		require.NoError(err)
	}()
	require.NotNil(bc)
	// check hash<-->height mapping
	blkhash, err := bc.GetHashByHeight(0)
	require.Nil(err)
	height, err = bc.GetHeightByHash(blkhash)
	require.Nil(err)
	require.Equal(uint64(0), height)
	blk, err := bc.GetBlockByHash(blkhash)
	require.Nil(err)
	require.Equal(blkhash, blk.HashBlock())
	fmt.Printf("Genesis hash = %x\n", blkhash)
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
	empblk, err := bc.GetBlockByHash(hash.ZeroHash32B)
	require.Nil(empblk)
	require.NotNil(err.Error())
	blk, err = bc.GetBlockByHeight(60000)
	require.Nil(blk)
	require.NotNil(err)
	// add wrong blocks
	h := bc.TipHeight()
	blkhash = bc.TipHash()
	blk, err = bc.GetBlockByHeight(h)
	require.Nil(err)
	require.Equal(blkhash, blk.HashBlock())
	fmt.Printf("Current tip = %d hash = %x\n", h, blkhash)
	// add block with wrong height
	cbTsf := action.NewCoinBaseTransfer(1, big.NewInt(50), ta.Addrinfo["bravo"].Bech32())
	require.NotNil(cbTsf)
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(1).
		SetDestinationAddress(ta.Addrinfo["bravo"].Bech32()).
		SetGasLimit(genesis.ActionGasLimit).
		SetAction(cbTsf).Build()
	selp, err := action.Sign(elp, ta.Addrinfo["bravo"].Bech32(), ta.Keyinfo["bravo"].PriKey)
	require.NoError(err)

	nblk, err := block.NewTestingBuilder().
		SetChainID(0).
		SetHeight(h+2).
		SetPrevBlockHash(blkhash).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(selp).SignAndBuild(ta.Keyinfo["bravo"].PubKey, ta.Keyinfo["bravo"].PriKey)
	require.NoError(err)

	err = bc.ValidateBlock(&nblk, true)
	require.NotNil(err)
	fmt.Printf("Cannot validate block %d: %v\n", blk.Height(), err)
	// add block with zero prev hash
	cbTsf2 := action.NewCoinBaseTransfer(1, big.NewInt(50), ta.Addrinfo["bravo"].Bech32())
	require.NotNil(cbTsf2)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetNonce(1).
		SetDestinationAddress(ta.Addrinfo["bravo"].Bech32()).
		SetGasLimit(genesis.ActionGasLimit).
		SetAction(cbTsf2).Build()
	selp2, err := action.Sign(elp, ta.Addrinfo["bravo"].Bech32(), ta.Keyinfo["bravo"].PriKey)
	require.NoError(err)

	nblk, err = block.NewTestingBuilder().
		SetChainID(0).
		SetHeight(h+1).
		SetPrevBlockHash(hash.ZeroHash32B).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(selp2).SignAndBuild(ta.Keyinfo["bravo"].PubKey, ta.Keyinfo["bravo"].PriKey)
	require.NoError(err)
	err = bc.ValidateBlock(&nblk, true)
	require.NotNil(err)
	fmt.Printf("Cannot validate block %d: %v\n", blk.Height(), err)
	// add existing block again will have no effect
	blk, err = bc.GetBlockByHeight(3)
	require.NotNil(blk)
	require.Nil(err)
	require.NoError(bc.(*blockchain).commitBlock(blk))
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
	_, err = bc.GetTransfersFromAddress(ta.Addrinfo["charlie"].Bech32())
	require.NotNil(err)
	_, err = bc.GetTransfersToAddress(ta.Addrinfo["charlie"].Bech32())
	require.NotNil(err)
	_, err = bc.GetVotesFromAddress(ta.Addrinfo["charlie"].Bech32())
	require.NotNil(err)
	_, err = bc.GetVotesFromAddress(ta.Addrinfo["alfa"].Bech32())
	require.NotNil(err)
	_, err = bc.GetVotesToAddress(ta.Addrinfo["charlie"].Bech32())
	require.NotNil(err)
	_, err = bc.GetVotesToAddress(ta.Addrinfo["alfa"].Bech32())
	require.NotNil(err)
	_, err = bc.GetTotalTransfers()
	require.NotNil(err)
	_, err = bc.GetTotalVotes()
	require.NotNil(err)
	_, err = bc.GetTransferByTransferHash(hash.ZeroHash32B)
	require.NotNil(err)
	_, err = bc.GetVoteByVoteHash(hash.ZeroHash32B)
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
	sf.AddActionHandlers(account.NewProtocol())
	require.Nil(err)
	require.NoError(sf.Start(context.Background()))
	require.NoError(addCreatorToFactory(sf))

	bc := NewBlockchain(cfg, PrecreatedStateFactoryOption(sf), BoltDBDaoOption())
	bc.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(bc))
	bc.Validator().AddActionValidators(account.NewProtocol(), vote.NewProtocol(bc))
	require.NoError(bc.Start(context.Background()))
	require.NotNil(bc)
	height := bc.TipHeight()
	require.Equal(0, int(height))

	blk, err := bc.MintNewBlock([]action.SealedEnvelope{}, ta.Keyinfo["alfa"].PubKey,
		ta.Keyinfo["alfa"].PriKey, ta.Addrinfo["alfa"].Bech32(), nil, nil, "")
	require.Nil(err)
	s, err := bc.StateByAddr(ta.Addrinfo["alfa"].Bech32())
	require.Nil(err)
	require.Equal(big.NewInt(0), s.Balance)
	require.Nil(bc.ValidateBlock(blk, true))
	require.Nil(bc.CommitBlock(blk))
	height = bc.TipHeight()
	require.True(height == 1)
	require.True(len(blk.Actions) == 1)
	s, err = bc.StateByAddr(ta.Addrinfo["alfa"].Bech32())
	require.Nil(err)
	require.Equal(Gen.BlockReward, s.Balance)
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
	a := ta.Addrinfo["alfa"].Bech32()
	priKeyA := ta.Keyinfo["alfa"].PriKey
	c := ta.Addrinfo["bravo"].Bech32()
	ws, err := sf.NewWorkingSet()
	require.NoError(err)
	_, err = account.LoadOrCreateAccount(ws, a, big.NewInt(100000))
	require.NoError(err)
	_, err = account.LoadOrCreateAccount(ws, c, big.NewInt(100000))
	require.NoError(err)
	gasLimit := testutil.TestGasLimit
	ctx := protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			ProducerAddr:    ta.Addrinfo["producer"].Bech32(),
			GasLimit:        &gasLimit,
			EnableGasCharge: testutil.EnableGasCharge,
		})
	_, _, err = ws.RunActions(ctx, 0, nil)
	require.NoError(err)
	require.NoError(sf.Commit(ws))

	for i := 0; i < 10; i++ {
		acts := []action.SealedEnvelope{}
		for i := 0; i < 1000; i++ {
			tsf, err := testutil.SignedTransfer(a, c, priKeyA, 1, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
			require.NoError(err)
			acts = append(acts, tsf)
		}
		blk, _ := bc.MintNewBlock(acts, ta.Keyinfo["producer"].PubKey, ta.Keyinfo["producer"].PriKey,
			ta.Addrinfo["producer"].Bech32(), nil, nil, "")
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
	a := ta.Addrinfo["alfa"].Bech32()
	priKeyA := ta.Keyinfo["alfa"].PriKey
	c := ta.Addrinfo["bravo"].Bech32()
	ws, err := sf.NewWorkingSet()
	require.NoError(err)
	_, err = account.LoadOrCreateAccount(ws, a, big.NewInt(100000))
	require.NoError(err)
	_, err = account.LoadOrCreateAccount(ws, c, big.NewInt(100000))
	require.NoError(err)
	gasLimit := testutil.TestGasLimit
	ctx := protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			ProducerAddr:    ta.Addrinfo["producer"].Bech32(),
			GasLimit:        &gasLimit,
			EnableGasCharge: testutil.EnableGasCharge,
		})
	_, _, err = ws.RunActions(ctx, 0, nil)
	require.NoError(err)
	require.NoError(sf.Commit(ws))

	val := &validator{sf: sf, validatorAddr: ""}
	bc.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(bc))
	bc.Validator().AddActionValidators(account.NewProtocol(), vote.NewProtocol(bc))
	acts := []action.SealedEnvelope{}
	for i := 0; i < 5000; i++ {
		tsf, err := testutil.SignedTransfer(a, c, priKeyA, 1, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
		require.NoError(err)
		acts = append(acts, tsf)

		vote, err := testutil.SignedVote(a, a, priKeyA, 1, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
		require.NoError(err)
		acts = append(acts, vote)
	}
	blk, _ := bc.MintNewBlock(acts, ta.Keyinfo["producer"].PubKey, ta.Keyinfo["producer"].PriKey,
		ta.Addrinfo["producer"].Bech32(), nil, nil, "")
	require.Nil(val.Validate(blk, 0, blk.PrevHash(), true))
}

func TestMintDKGBlock(t *testing.T) {
	require := require.New(t)
	lastSeed, _ := hex.DecodeString("9de6306b08158c423330f7a27243a1a5cbe39bfd764f07818437882d21241567")
	cfg := config.Default
	clk := clock.NewMock()
	chain := NewBlockchain(cfg, InMemDaoOption(), InMemStateFactoryOption(), ClockOption(clk))
	chain.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(chain))
	chain.Validator().AddActionValidators(account.NewProtocol(), vote.NewProtocol(chain))
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
		addresses[i] = address.New(cfg.Chain.ID, pkHash[:]).Bech32()
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
		blk, err := chain.MintNewBlock(nil, ec283PKList[i], ec283SKList[i], addresses[i],
			&address.DKGAddress{PrivateKey: askList[i], PublicKey: pkList[i], ID: idList[i]}, lastSeed, "")
		require.NoError(err)
		require.NoError(chain.ValidateBlock(blk, true))
		require.NoError(chain.CommitBlock(blk))
		require.Equal(pkList[i], blk.DKGPubkey())
		require.Equal(idList[i], blk.DKGID())
		require.True(len(blk.DKGSignature()) > 0)
	}
	height, err := chain.GetFactory().Height()
	require.NoError(err)
	require.Equal(uint64(20), height)
	candidates, err := chain.CandidatesByHeight(height)
	require.NoError(err)
	require.Equal(21, len(candidates))
}

func TestStartExistingBlockchain(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)

	// Disable block reward to make bookkeeping easier
	Gen.BlockReward = big.NewInt(0)
	cfg := config.Default
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath

	sf, err := factory.NewFactory(cfg, factory.DefaultTrieOption())
	require.NoError(err)
	require.NoError(sf.Start(context.Background()))
	sf.AddActionHandlers(account.NewProtocol())

	// Create a blockchain from scratch
	bc := NewBlockchain(cfg, PrecreatedStateFactoryOption(sf), BoltDBDaoOption())
	require.NotNil(bc)
	bc.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(bc))
	bc.Validator().AddActionValidators(account.NewProtocol(), vote.NewProtocol(bc))
	sf.AddActionHandlers(vote.NewProtocol(bc))
	require.NoError(bc.Start(ctx))

	defer func() {
		require.NoError(sf.Stop(ctx))
		require.NoError(bc.Stop(ctx))
		testutil.CleanupPath(t, testTriePath)
		testutil.CleanupPath(t, testDBPath)
	}()

	require.NoError(addTestingTsfBlocks(bc))
	require.True(5 == bc.TipHeight())

	// delete state db and recover to tip
	testutil.CleanupPath(t, testTriePath)
	sf, err = factory.NewFactory(cfg, factory.DefaultTrieOption())
	require.NoError(err)
	require.NoError(sf.Start(context.Background()))
	sf.AddActionHandlers(account.NewProtocol())
	sf.AddActionHandlers(vote.NewProtocol(bc))
	chain, ok := bc.(*blockchain)
	require.True(ok)
	chain.sf = sf
	require.NoError(chain.startExistingBlockchain(0))
	height, _ := chain.sf.Height()
	require.Equal(bc.TipHeight(), height)

	// recover to height 3
	testutil.CleanupPath(t, testTriePath)
	sf, err = factory.NewFactory(cfg, factory.DefaultTrieOption())
	require.NoError(err)
	require.NoError(sf.Start(context.Background()))
	sf.AddActionHandlers(account.NewProtocol())
	sf.AddActionHandlers(vote.NewProtocol(bc))
	chain.sf = sf
	require.NoError(chain.startExistingBlockchain(3))
	height, _ = chain.sf.Height()
	require.Equal(bc.TipHeight(), height)
	require.True(3 == height)
}

func addCreatorToFactory(sf factory.Factory) error {
	ws, err := sf.NewWorkingSet()
	if err != nil {
		return err
	}
	if _, err = account.LoadOrCreateAccount(ws, ta.Addrinfo["producer"].Bech32(), Gen.TotalSupply); err != nil {
		return err
	}
	gasLimit := testutil.TestGasLimit
	ctx := protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			ProducerAddr:    ta.Addrinfo["producer"].Bech32(),
			GasLimit:        &gasLimit,
			EnableGasCharge: testutil.EnableGasCharge,
		})
	if _, _, err = ws.RunActions(ctx, 0, nil); err != nil {
		return err
	}
	return sf.Commit(ws)
}
