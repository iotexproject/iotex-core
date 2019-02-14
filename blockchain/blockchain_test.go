// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	"github.com/iotexproject/iotex-core/action/protocol/vote"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/config"
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
		ta.Addrinfo["producer"].String(),
		[]byte{}, uint64(100000),
		big.NewInt(10),
	)
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetAction(tsf0).
		SetDestinationAddress(ta.Addrinfo["producer"].String()).
		SetNonce(1).
		SetGasLimit(100000).
		SetGasPrice(big.NewInt(10)).Build()
	genSK, err := keypair.DecodePrivateKey(GenesisProducerPrivateKey)
	if err != nil {
		return err
	}
	selp, err := action.Sign(elp, genSK)
	if err != nil {
		return err
	}
	actionMap := make(map[string][]action.SealedEnvelope)
	actionMap[Gen.CreatorAddr()] = []action.SealedEnvelope{selp}
	blk, err := bc.MintNewBlock(
		actionMap,
		ta.Keyinfo["producer"].PubKey,
		ta.Keyinfo["producer"].PriKey,
		ta.Addrinfo["producer"].String(),
		0,
	)
	if err != nil {
		return err
	}
	if err := bc.ValidateBlock(blk); err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	addr0 := ta.Addrinfo["producer"].String()
	priKey0 := ta.Keyinfo["producer"].PriKey
	addr1 := ta.Addrinfo["alfa"].String()
	priKey1 := ta.Keyinfo["alfa"].PriKey
	addr2 := ta.Addrinfo["bravo"].String()
	addr3 := ta.Addrinfo["charlie"].String()
	priKey3 := ta.Keyinfo["charlie"].PriKey
	addr4 := ta.Addrinfo["delta"].String()
	priKey4 := ta.Keyinfo["delta"].PriKey
	addr5 := ta.Addrinfo["echo"].String()
	priKey5 := ta.Keyinfo["echo"].PriKey
	addr6 := ta.Addrinfo["foxtrot"].String()
	// Add block 1
	// test --> A, B, C, D, E, F
	tsf1, err := testutil.SignedTransfer(addr1, priKey0, 1, big.NewInt(20), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf2, err := testutil.SignedTransfer(addr2, priKey0, 2, big.NewInt(30), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf3, err := testutil.SignedTransfer(addr3, priKey0, 3, big.NewInt(50), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf4, err := testutil.SignedTransfer(addr4, priKey0, 4, big.NewInt(70), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf5, err := testutil.SignedTransfer(addr5, priKey0, 5, big.NewInt(110), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf6, err := testutil.SignedTransfer(addr6, priKey0, 6, big.NewInt(50<<20), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	accMap := make(map[string][]action.SealedEnvelope)
	accMap[addr0] = []action.SealedEnvelope{tsf1, tsf2, tsf3, tsf4, tsf5, tsf6}

	blk, err = bc.MintNewBlock(
		accMap,
		ta.Keyinfo["producer"].PubKey,
		ta.Keyinfo["producer"].PriKey,
		ta.Addrinfo["producer"].String(),
		0,
	)
	if err != nil {
		return err
	}
	if err := bc.ValidateBlock(blk); err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	// Add block 2
	// Charlie --> A, B, D, E, test
	tsf1, err = testutil.SignedTransfer(addr1, priKey3, 1, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf2, err = testutil.SignedTransfer(addr2, priKey3, 2, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf3, err = testutil.SignedTransfer(addr4, priKey3, 3, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf4, err = testutil.SignedTransfer(addr5, priKey3, 4, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf5, err = testutil.SignedTransfer(addr0, priKey3, 5, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	accMap = make(map[string][]action.SealedEnvelope)
	accMap[addr3] = []action.SealedEnvelope{tsf1, tsf2, tsf3, tsf4, tsf5}
	blk, err = bc.MintNewBlock(
		accMap,
		ta.Keyinfo["producer"].PubKey,
		ta.Keyinfo["producer"].PriKey,
		ta.Addrinfo["producer"].String(),
		0,
	)
	if err != nil {
		return err
	}

	if err := bc.ValidateBlock(blk); err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	// Add block 3
	// Delta --> B, E, F, test
	tsf1, err = testutil.SignedTransfer(addr2, priKey4, 1, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf2, err = testutil.SignedTransfer(addr5, priKey4, 2, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf3, err = testutil.SignedTransfer(addr6, priKey4, 3, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf4, err = testutil.SignedTransfer(addr0, priKey4, 4, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}

	accMap = make(map[string][]action.SealedEnvelope)
	accMap[addr4] = []action.SealedEnvelope{tsf1, tsf2, tsf3, tsf4}
	blk, err = bc.MintNewBlock(
		accMap,
		ta.Keyinfo["producer"].PubKey,
		ta.Keyinfo["producer"].PriKey,
		ta.Addrinfo["producer"].String(),
		0,
	)

	if err != nil {
		return err
	}
	if err := bc.ValidateBlock(blk); err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	// Add block 4
	// Delta --> A, B, C, D, F, test
	tsf1, err = testutil.SignedTransfer(addr1, priKey5, 1, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf2, err = testutil.SignedTransfer(addr2, priKey5, 2, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf3, err = testutil.SignedTransfer(addr3, priKey5, 3, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf4, err = testutil.SignedTransfer(addr4, priKey5, 4, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf5, err = testutil.SignedTransfer(addr6, priKey5, 5, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf6, err = testutil.SignedTransfer(addr0, priKey5, 6, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	vote1, err := testutil.SignedVote(addr3, priKey3, 6, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	vote2, err := testutil.SignedVote(addr1, priKey1, 1, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	accMap = make(map[string][]action.SealedEnvelope)
	accMap[addr5] = []action.SealedEnvelope{tsf1, tsf2, tsf3, tsf4, tsf5, tsf6}
	accMap[addr3] = []action.SealedEnvelope{vote1}
	accMap[addr1] = []action.SealedEnvelope{vote2}

	blk, err = bc.MintNewBlock(
		accMap,
		ta.Keyinfo["producer"].PubKey,
		ta.Keyinfo["producer"].PriKey,
		ta.Addrinfo["producer"].String(),
		0,
	)
	if err != nil {
		return err
	}
	if err := bc.ValidateBlock(blk); err != nil {
		return err
	}
	if blk.TxRoot() != blk.CalculateTxRoot() {
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
	require.NoError(addTestingTsfBlocks(bc))
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

	tsf, err := action.NewTransfer(
		1,
		big.NewInt(3000000000),
		ta.Addrinfo["producer"].String(),
		[]byte{}, uint64(100000),
		big.NewInt(10),
	)
	require.NoError(t, err)
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetAction(tsf).
		SetDestinationAddress(ta.Addrinfo["producer"].String()).
		SetNonce(1).
		SetGasLimit(100000).
		SetGasPrice(big.NewInt(10)).Build()
	genSK, err := keypair.DecodePrivateKey(GenesisProducerPrivateKey)
	require.NoError(t, err)
	selp, err := action.Sign(elp, genSK)
	require.NoError(t, err)
	actionMap := make(map[string][]action.SealedEnvelope)
	actionMap[Gen.CreatorAddr()] = []action.SealedEnvelope{selp}
	_, err = bc.MintNewBlock(
		actionMap,
		ta.Keyinfo["producer"].PubKey,
		ta.Keyinfo["producer"].PriKey,
		ta.Addrinfo["producer"].String(),
		0,
	)
	require.NoError(t, err)
}

func TestBlockchain_MintNewBlock_PopAccount(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default
	cfg.Chain.EnableGasCharge = true
	bc := NewBlockchain(cfg, InMemStateFactoryOption(), InMemDaoOption())
	bc.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(bc))
	bc.Validator().AddActionValidators(account.NewProtocol(), vote.NewProtocol(bc))
	bc.GetFactory().AddActionHandlers(account.NewProtocol(), vote.NewProtocol(bc))
	require.NoError(t, bc.Start(ctx))
	defer require.NoError(t, bc.Stop(ctx))

	addr0 := ta.Addrinfo["producer"].String()
	priKey0 := ta.Keyinfo["producer"].PriKey
	addr1 := ta.Addrinfo["alfa"].String()
	addr3 := ta.Addrinfo["charlie"].String()
	priKey3 := ta.Keyinfo["charlie"].PriKey
	addTestingTsfBlocks(bc)

	// test third block
	bytes := []byte{}
	for i := 0; i < 1000; i++ {
		bytes = append(bytes, 1)
	}
	actionMap := make(map[string][]action.SealedEnvelope)
	actions := make([]action.SealedEnvelope, 0)
	for i := uint64(0); i < 300; i++ {
		tsf, err := testutil.SignedTransfer(addr1, priKey0, i+7, big.NewInt(2), bytes,
			1000000, big.NewInt(testutil.TestGasPrice))
		require.NoError(t, err)
		actions = append(actions, tsf)
	}
	actionMap[addr0] = actions
	transfer1, err := testutil.SignedTransfer(addr1, priKey3, 7, big.NewInt(2),
		[]byte{}, 100000, big.NewInt(testutil.TestGasPrice))
	require.NoError(t, err)
	actionMap[addr3] = []action.SealedEnvelope{transfer1}

	blk, err := bc.MintNewBlock(
		actionMap,
		ta.Keyinfo["producer"].PubKey,
		ta.Keyinfo["producer"].PriKey,
		ta.Addrinfo["producer"].String(),
		0,
	)
	require.NoError(t, err)
	require.NotNil(t, blk)
	require.Equal(t, 182, len(blk.Actions))
	whetherInclude := false
	for _, action := range blk.Actions {
		if transfer1.Hash() == action.Hash() {
			whetherInclude = true
			break
		}
	}
	require.True(t, whetherInclude)
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
	cfg.Chain.EnableIndex = true

	sf, err := factory.NewFactory(cfg, factory.DefaultTrieOption())
	require.Nil(err)
	sf.AddActionHandlers(account.NewProtocol())

	// Create a blockchain from scratch
	bc := NewBlockchain(cfg, PrecreatedStateFactoryOption(sf), BoltDBDaoOption())
	bc.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(bc))
	bc.Validator().AddActionValidators(account.NewProtocol(), vote.NewProtocol(bc))
	sf.AddActionHandlers(vote.NewProtocol(bc))
	require.NoError(bc.Start(ctx))
	require.NoError(addCreatorToFactory(sf))

	ms := &MockSubscriber{counter: 0}
	err = bc.AddSubscriber(ms)
	require.Nil(err)
	require.Equal(0, ms.Counter())

	height := bc.TipHeight()
	fmt.Printf("Open blockchain pass, height = %d\n", height)
	require.Nil(addTestingTsfBlocks(bc))
	err = bc.Stop(ctx)
	require.NoError(err)
	require.Equal(22, ms.Counter())

	// Load a blockchain from DB
	sf, err = factory.NewFactory(cfg, factory.DefaultTrieOption())
	require.Nil(err)
	sf.AddActionHandlers(account.NewProtocol())

	bc = NewBlockchain(cfg, PrecreatedStateFactoryOption(sf), BoltDBDaoOption())
	bc.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(bc))
	bc.Validator().AddActionValidators(account.NewProtocol(), vote.NewProtocol(bc))
	require.NoError(bc.Start(ctx))
	defer func() {
		require.NoError(bc.Stop(ctx))
	}()

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

	empblk, err := bc.GetBlockByHash(hash.ZeroHash256)
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
	selp, err := testutil.SignedTransfer(ta.Addrinfo["bravo"].String(), ta.Keyinfo["producer"].PriKey, 1, big.NewInt(50), nil, genesis.ActionGasLimit, big.NewInt(0))
	require.NoError(err)

	nblk, err := block.NewTestingBuilder().
		SetChainID(0).
		SetHeight(h+2).
		SetPrevBlockHash(blkhash).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(selp).SignAndBuild(ta.Keyinfo["bravo"].PubKey, ta.Keyinfo["bravo"].PriKey)
	require.NoError(err)

	err = bc.ValidateBlock(&nblk)
	require.NotNil(err)
	fmt.Printf("Cannot validate block %d: %v\n", blk.Height(), err)

	// add block with zero prev hash
	selp2, err := testutil.SignedTransfer(ta.Addrinfo["bravo"].String(), ta.Keyinfo["producer"].PriKey, 1, big.NewInt(50), nil, genesis.ActionGasLimit, big.NewInt(0))
	require.NoError(err)

	nblk, err = block.NewTestingBuilder().
		SetChainID(0).
		SetHeight(h+1).
		SetPrevBlockHash(hash.ZeroHash256).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(selp2).SignAndBuild(ta.Keyinfo["bravo"].PubKey, ta.Keyinfo["bravo"].PriKey)
	require.NoError(err)
	err = bc.ValidateBlock(&nblk)
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

	fromTransfers, err := bc.GetTransfersFromAddress(ta.Addrinfo["charlie"].String())
	require.Nil(err)
	require.Equal(len(fromTransfers), 5)

	toTransfers, err := bc.GetTransfersToAddress(ta.Addrinfo["charlie"].String())
	require.Nil(err)
	require.Equal(len(toTransfers), 2)

	fromVotes, err := bc.GetVotesFromAddress(ta.Addrinfo["charlie"].String())
	require.Nil(err)
	require.Equal(len(fromVotes), 1)

	fromVotes, err = bc.GetVotesFromAddress(ta.Addrinfo["alfa"].String())
	require.Nil(err)
	require.Equal(len(fromVotes), 1)

	toVotes, err := bc.GetVotesToAddress(ta.Addrinfo["charlie"].String())
	require.Nil(err)
	require.Equal(len(toVotes), 1)

	toVotes, err = bc.GetVotesToAddress(ta.Addrinfo["alfa"].String())
	require.Nil(err)
	require.Equal(len(toVotes), 1)

	totalTransfers, err := bc.GetTotalTransfers()
	require.Nil(err)
	require.Equal(totalTransfers, uint64(22))

	totalVotes, err := bc.GetTotalVotes()
	require.Nil(err)
	require.Equal(totalVotes, uint64(23))

	_, err = bc.GetTransferByTransferHash(hash.ZeroHash256)
	require.NotNil(err)
	_, err = bc.GetVoteByVoteHash(hash.ZeroHash256)
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
	// Create a blockchain from scratch
	bc := NewBlockchain(cfg, PrecreatedStateFactoryOption(sf), BoltDBDaoOption())
	bc.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(bc))
	bc.Validator().AddActionValidators(account.NewProtocol(), vote.NewProtocol(bc))
	sf.AddActionHandlers(vote.NewProtocol(bc))
	require.NoError(bc.Start(ctx))

	require.NoError(addCreatorToFactory(sf))

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
	sf, err = factory.NewFactory(cfg, factory.DefaultTrieOption())
	require.Nil(err)
	sf.AddActionHandlers(account.NewProtocol())

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
	empblk, err := bc.GetBlockByHash(hash.ZeroHash256)
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
	selp, err := testutil.SignedTransfer(ta.Addrinfo["bravo"].String(), ta.Keyinfo["producer"].PriKey, 1, big.NewInt(50), nil, genesis.ActionGasLimit, big.NewInt(0))
	require.NoError(err)

	nblk, err := block.NewTestingBuilder().
		SetChainID(0).
		SetHeight(h+2).
		SetPrevBlockHash(blkhash).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(selp).SignAndBuild(ta.Keyinfo["bravo"].PubKey, ta.Keyinfo["bravo"].PriKey)
	require.NoError(err)

	err = bc.ValidateBlock(&nblk)
	require.NotNil(err)
	fmt.Printf("Cannot validate block %d: %v\n", blk.Height(), err)
	// add block with zero prev hash
	selp2, err := testutil.SignedTransfer(ta.Addrinfo["bravo"].String(), ta.Keyinfo["producer"].PriKey, 1, big.NewInt(50), nil, genesis.ActionGasLimit, big.NewInt(0))
	require.NoError(err)

	nblk, err = block.NewTestingBuilder().
		SetChainID(0).
		SetHeight(h+1).
		SetPrevBlockHash(hash.ZeroHash256).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(selp2).SignAndBuild(ta.Keyinfo["bravo"].PubKey, ta.Keyinfo["bravo"].PriKey)
	require.NoError(err)
	err = bc.ValidateBlock(&nblk)
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
	_, err = bc.GetTransfersFromAddress(ta.Addrinfo["charlie"].String())
	require.NotNil(err)
	_, err = bc.GetTransfersToAddress(ta.Addrinfo["charlie"].String())
	require.NotNil(err)
	_, err = bc.GetVotesFromAddress(ta.Addrinfo["charlie"].String())
	require.NotNil(err)
	_, err = bc.GetVotesFromAddress(ta.Addrinfo["alfa"].String())
	require.NotNil(err)
	_, err = bc.GetVotesToAddress(ta.Addrinfo["charlie"].String())
	require.NotNil(err)
	_, err = bc.GetVotesToAddress(ta.Addrinfo["alfa"].String())
	require.NotNil(err)
	_, err = bc.GetTotalTransfers()
	require.NotNil(err)
	_, err = bc.GetTotalVotes()
	require.NotNil(err)
	_, err = bc.GetTransferByTransferHash(hash.ZeroHash256)
	require.NotNil(err)
	_, err = bc.GetVoteByVoteHash(hash.ZeroHash256)
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
	bc := NewBlockchain(cfg, PrecreatedStateFactoryOption(sf), BoltDBDaoOption())
	require.NoError(bc.Start(context.Background()))
	defer func() {
		require.NoError(bc.Stop(context.Background()))
	}()
	// TODO: change the value when Candidates size is changed
	height, err := sf.Height()
	require.NoError(err)
	require.Equal(uint64(0), height)
	candidate, err := sf.CandidatesByHeight(height)
	require.NoError(err)
	require.True(len(candidate) == 2)
}

func TestBlockchain_StateByAddr(t *testing.T) {
	require := require.New(t)

	cfg := config.Default
	// disable account-based testing
	// create chain
	bc := NewBlockchain(cfg, InMemDaoOption(), InMemStateFactoryOption())
	require.NoError(bc.Start(context.Background()))
	require.NotNil(bc)

	s, err := bc.StateByAddr(Gen.CreatorAddr())
	require.NoError(err)
	require.Equal(uint64(0), s.Nonce)
	bal := big.NewInt(7700000000)
	require.Equal(bal.Mul(bal, big.NewInt(1e18)).String(), s.Balance.String())
	require.Equal(hash.ZeroHash256, s.Root)
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

	// Create a blockchain from scratch
	bc := NewBlockchain(cfg, PrecreatedStateFactoryOption(sf), BoltDBDaoOption())
	require.NoError(bc.Start(context.Background()))
	defer func() {
		require.NoError(bc.Stop(context.Background()))
	}()

	require.NoError(addCreatorToFactory(sf))

	a := ta.Addrinfo["alfa"].String()
	priKeyA := ta.Keyinfo["alfa"].PriKey
	c := ta.Addrinfo["bravo"].String()
	ws, err := sf.NewWorkingSet()
	require.NoError(err)
	_, err = account.LoadOrCreateAccount(ws, a, big.NewInt(100000))
	require.NoError(err)
	_, err = account.LoadOrCreateAccount(ws, c, big.NewInt(100000))
	require.NoError(err)
	gasLimit := testutil.TestGasLimit
	ctx := protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			Producer:        ta.Addrinfo["producer"],
			GasLimit:        &gasLimit,
			EnableGasCharge: testutil.EnableGasCharge,
		})
	_, _, err = ws.RunActions(ctx, 0, nil)
	require.NoError(err)
	require.NoError(sf.Commit(ws))

	for i := 0; i < 10; i++ {
		acts := []action.SealedEnvelope{}
		actionMap := make(map[string][]action.SealedEnvelope)
		actionMap[a] = []action.SealedEnvelope{}
		for i := 0; i < 1000; i++ {
			tsf, err := testutil.SignedTransfer(c, priKeyA, 1, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
			require.NoError(err)
			acts = append(acts, tsf)
			actionMap[a] = append(actionMap[a], tsf)
		}
		blk, _ := bc.MintNewBlock(
			actionMap,
			ta.Keyinfo["producer"].PubKey,
			ta.Keyinfo["producer"].PriKey,
			ta.Addrinfo["producer"].String(),
			0,
		)
		require.Nil(bc.ValidateBlock(blk))
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

	// Create a blockchain from scratch
	bc := NewBlockchain(cfg, PrecreatedStateFactoryOption(sf), BoltDBDaoOption())
	require.NoError(bc.Start(context.Background()))
	defer func() {
		require.NoError(bc.Stop(context.Background()))
	}()

	require.NoError(addCreatorToFactory(sf))
	a := ta.Addrinfo["alfa"].String()
	priKeyA := ta.Keyinfo["alfa"].PriKey
	c := ta.Addrinfo["bravo"].String()
	ws, err := sf.NewWorkingSet()
	require.NoError(err)
	_, err = account.LoadOrCreateAccount(ws, a, big.NewInt(100000))
	require.NoError(err)
	_, err = account.LoadOrCreateAccount(ws, c, big.NewInt(100000))
	require.NoError(err)
	gasLimit := testutil.TestGasLimit
	ctx := protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			Producer:        ta.Addrinfo["producer"],
			GasLimit:        &gasLimit,
			EnableGasCharge: testutil.EnableGasCharge,
		})
	_, _, err = ws.RunActions(ctx, 0, nil)
	require.NoError(err)
	require.NoError(sf.Commit(ws))

	val := &validator{sf: sf, validatorAddr: ""}
	bc.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(bc))
	bc.Validator().AddActionValidators(account.NewProtocol(), vote.NewProtocol(bc))
	actionMap := make(map[string][]action.SealedEnvelope)
	for i := 0; i < 5000; i++ {
		tsf, err := testutil.SignedTransfer(c, priKeyA, 1, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
		require.NoError(err)
		actionMap[a] = append(actionMap[a], tsf)

		vote, err := testutil.SignedVote(a, priKeyA, 1, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
		require.NoError(err)
		actionMap[a] = append(actionMap[a], vote)
	}
	blk, _ := bc.MintNewBlock(
		actionMap,
		ta.Keyinfo["producer"].PubKey,
		ta.Keyinfo["producer"].PriKey,
		ta.Addrinfo["producer"].String(),
		0,
	)
	require.Nil(val.Validate(blk, 0, blk.PrevHash()))
}

func addCreatorToFactory(sf factory.Factory) error {
	ws, err := sf.NewWorkingSet()
	if err != nil {
		return err
	}
	if _, err = account.LoadOrCreateAccount(ws, ta.Addrinfo["producer"].String(), Gen.TotalSupply); err != nil {
		return err
	}
	gasLimit := testutil.TestGasLimit
	ctx := protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			Producer:        ta.Addrinfo["producer"],
			GasLimit:        &gasLimit,
			EnableGasCharge: testutil.EnableGasCharge,
		})
	if _, _, err = ws.RunActions(ctx, 0, nil); err != nil {
		return err
	}
	return sf.Commit(ws)
}
