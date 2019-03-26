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
	"io/ioutil"
	"math/big"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/execution"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/action/protocol/vote"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/iotexproject/iotex-core/test/identityset"
	ta "github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/testutil"
)

func addTestingTsfBlocks(bc Blockchain) error {
	// Add block 0
	tsf0, _ := action.NewTransfer(
		1,
		big.NewInt(90000000),
		ta.Addrinfo["producer"].String(),
		[]byte{}, uint64(100000),
		big.NewInt(10),
	)
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetAction(tsf0).
		SetNonce(1).
		SetGasLimit(100000).
		SetGasPrice(big.NewInt(10)).Build()
	selp, err := action.Sign(elp, identityset.PrivateKey(0))
	if err != nil {
		return err
	}
	actionMap := make(map[string][]action.SealedEnvelope)
	actionMap[identityset.Address(0).String()] = []action.SealedEnvelope{selp}
	blk, err := bc.MintNewBlock(
		actionMap,
		testutil.TimestampNow(),
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
	tsf1, err := testutil.SignedTransfer(addr1, priKey0, 1, big.NewInt(20), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	tsf2, err := testutil.SignedTransfer(addr2, priKey0, 2, big.NewInt(30), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	tsf3, err := testutil.SignedTransfer(addr3, priKey0, 3, big.NewInt(50), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	tsf4, err := testutil.SignedTransfer(addr4, priKey0, 4, big.NewInt(70), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	tsf5, err := testutil.SignedTransfer(addr5, priKey0, 5, big.NewInt(110), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	tsf6, err := testutil.SignedTransfer(addr6, priKey0, 6, big.NewInt(50<<20), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	accMap := make(map[string][]action.SealedEnvelope)
	accMap[addr0] = []action.SealedEnvelope{tsf1, tsf2, tsf3, tsf4, tsf5, tsf6}

	blk, err = bc.MintNewBlock(
		accMap,
		testutil.TimestampNow(),
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
	tsf1, err = testutil.SignedTransfer(addr1, priKey3, 1, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	tsf2, err = testutil.SignedTransfer(addr2, priKey3, 2, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	tsf3, err = testutil.SignedTransfer(addr4, priKey3, 3, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	tsf4, err = testutil.SignedTransfer(addr5, priKey3, 4, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	tsf5, err = testutil.SignedTransfer(addr0, priKey3, 5, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	accMap = make(map[string][]action.SealedEnvelope)
	accMap[addr3] = []action.SealedEnvelope{tsf1, tsf2, tsf3, tsf4, tsf5}
	blk, err = bc.MintNewBlock(
		accMap,
		testutil.TimestampNow(),
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
	tsf1, err = testutil.SignedTransfer(addr2, priKey4, 1, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	tsf2, err = testutil.SignedTransfer(addr5, priKey4, 2, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	tsf3, err = testutil.SignedTransfer(addr6, priKey4, 3, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	tsf4, err = testutil.SignedTransfer(addr0, priKey4, 4, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}

	accMap = make(map[string][]action.SealedEnvelope)
	accMap[addr4] = []action.SealedEnvelope{tsf1, tsf2, tsf3, tsf4}
	blk, err = bc.MintNewBlock(
		accMap,
		testutil.TimestampNow(),
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
	tsf1, err = testutil.SignedTransfer(addr1, priKey5, 1, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	tsf2, err = testutil.SignedTransfer(addr2, priKey5, 2, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	tsf3, err = testutil.SignedTransfer(addr3, priKey5, 3, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	tsf4, err = testutil.SignedTransfer(addr4, priKey5, 4, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	tsf5, err = testutil.SignedTransfer(addr6, priKey5, 5, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	tsf6, err = testutil.SignedTransfer(addr0, priKey5, 6, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	vote1, err := testutil.SignedVote(addr3, priKey3, 6, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	vote2, err := testutil.SignedVote(addr1, priKey1, 1, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	accMap = make(map[string][]action.SealedEnvelope)
	accMap[addr5] = []action.SealedEnvelope{tsf1, tsf2, tsf3, tsf4, tsf5, tsf6}
	accMap[addr3] = []action.SealedEnvelope{vote1}
	accMap[addr1] = []action.SealedEnvelope{vote2}

	blk, err = bc.MintNewBlock(
		accMap,
		testutil.TimestampNow(),
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
	registry := protocol.Registry{}
	acc := account.NewProtocol()
	registry.Register(account.ProtocolID, acc)
	rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
	require.NoError(registry.Register(rolldpos.ProtocolID, rp))
	bc := NewBlockchain(cfg, InMemStateFactoryOption(), InMemDaoOption(), RegistryOption(&registry))
	bc.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(bc, genesis.Default.ActionGasLimit))
	v := vote.NewProtocol(bc)
	require.NoError(registry.Register(vote.ProtocolID, v))
	bc.Validator().AddActionValidators(acc, v)
	bc.GetFactory().AddActionHandlers(acc, v)
	require.NoError(bc.Start(ctx))
	require.NotNil(bc)
	height := bc.TipHeight()
	require.Equal(0, int(height))
	fmt.Printf("Create blockchain pass, height = %d\n", height)
	defer func() {
		err := bc.Stop(ctx)
		require.NoError(err)
	}()

	// add 4 sample blocks
	require.NoError(addTestingTsfBlocks(bc))
	height = bc.TipHeight()
	require.Equal(5, int(height))
}

func TestBlockchain_MintNewBlock(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default
	cfg.Genesis.BlockGasLimit = uint64(100000)

	registry := protocol.Registry{}
	acc := account.NewProtocol()
	require.NoError(t, registry.Register(account.ProtocolID, acc))
	rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
	require.NoError(t, registry.Register(rolldpos.ProtocolID, rp))
	bc := NewBlockchain(cfg, InMemStateFactoryOption(), InMemDaoOption(), RegistryOption(&registry))
	bc.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(bc, genesis.Default.ActionGasLimit))
	v := vote.NewProtocol(bc)
	require.NoError(t, registry.Register(vote.ProtocolID, v))
	exec := execution.NewProtocol(bc)
	require.NoError(t, registry.Register(execution.ProtocolID, exec))
	bc.Validator().AddActionValidators(acc, v, exec)
	bc.GetFactory().AddActionHandlers(acc, v, exec)
	require.NoError(t, bc.Start(ctx))
	defer require.NoError(t, bc.Stop(ctx))

	tsf, err := action.NewTransfer(
		1,
		big.NewInt(100000000),
		ta.Addrinfo["producer"].String(),
		[]byte{}, uint64(100000),
		big.NewInt(10),
	)
	require.NoError(t, err)

	data, _ := hex.DecodeString("608060405234801561001057600080fd5b5060df8061001f6000396000f3006080604052600436106049576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff16806360fe47b114604e5780636d4ce63c146078575b600080fd5b348015605957600080fd5b5060766004803603810190808035906020019092919050505060a0565b005b348015608357600080fd5b50608a60aa565b6040518082815260200191505060405180910390f35b8060008190555050565b600080549050905600a165627a7a7230582002faabbefbbda99b20217cf33cb8ab8100caf1542bf1f48117d72e2c59139aea0029")
	execution, err := action.NewExecution(action.EmptyAddress, 2, big.NewInt(0), uint64(100000), big.NewInt(0), data)
	require.NoError(t, err)

	bd := &action.EnvelopeBuilder{}
	elp1 := bd.SetAction(tsf).
		SetNonce(1).
		SetGasLimit(100000).
		SetGasPrice(big.NewInt(10)).Build()
	selp1, err := action.Sign(elp1, identityset.PrivateKey(0))
	require.NoError(t, err)
	// This execution should not be included in block because block is out of gas
	elp2 := bd.SetAction(execution).
		SetNonce(2).
		SetGasLimit(100000).
		SetGasPrice(big.NewInt(10)).Build()
	selp2, err := action.Sign(elp2, identityset.PrivateKey(0))
	require.NoError(t, err)

	actionMap := make(map[string][]action.SealedEnvelope)
	actionMap[identityset.Address(0).String()] = []action.SealedEnvelope{selp1, selp2}

	blk, err := bc.MintNewBlock(
		actionMap,
		testutil.TimestampNow(),
	)
	require.NoError(t, err)
	require.Equal(t, 2, len(blk.Actions))
	require.Equal(t, 1, len(blk.Receipts))
	var gasConsumed uint64
	for _, receipt := range blk.Receipts {
		gasConsumed += receipt.GasConsumed
	}
	require.True(t, gasConsumed <= cfg.Genesis.BlockGasLimit)
}

func TestBlockchain_MintNewBlock_PopAccount(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default
	registry := protocol.Registry{}
	acc := account.NewProtocol()
	require.NoError(t, registry.Register(account.ProtocolID, acc))
	bc := NewBlockchain(cfg, InMemStateFactoryOption(), InMemDaoOption(), RegistryOption(&registry))
	rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
	require.NoError(t, registry.Register(rolldpos.ProtocolID, rp))
	bc.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(bc, genesis.Default.ActionGasLimit))
	v := vote.NewProtocol(bc)
	require.NoError(t, registry.Register(vote.ProtocolID, v))
	bc.Validator().AddActionValidators(acc, v)
	bc.GetFactory().AddActionHandlers(acc, v)
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
			1000000, big.NewInt(testutil.TestGasPriceInt64))
		require.NoError(t, err)
		actions = append(actions, tsf)
	}
	actionMap[addr0] = actions
	transfer1, err := testutil.SignedTransfer(addr1, priKey3, 7, big.NewInt(2),
		[]byte{}, 100000, big.NewInt(testutil.TestGasPriceInt64))
	require.NoError(t, err)
	actionMap[addr3] = []action.SealedEnvelope{transfer1}

	blk, err := bc.MintNewBlock(
		actionMap,
		testutil.TimestampNow(),
	)
	require.NoError(t, err)
	require.NotNil(t, blk)
	require.Equal(t, 183, len(blk.Actions))
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

	testTrieFile, _ := ioutil.TempFile(os.TempDir(), "trie")
	testTriePath := testTrieFile.Name()
	testDBFile, _ := ioutil.TempFile(os.TempDir(), "db")
	testDBPath := testDBFile.Name()

	cfg := config.Default
	cfg.Plugins[config.GatewayPlugin] = true
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath

	sf, err := factory.NewFactory(cfg, factory.DefaultTrieOption())
	require.NoError(err)
	sf.AddActionHandlers(account.NewProtocol())

	// Create a blockchain from scratch
	registry := protocol.Registry{}
	acc := account.NewProtocol()
	require.NoError(registry.Register(account.ProtocolID, acc))
	bc := NewBlockchain(
		cfg,
		PrecreatedStateFactoryOption(sf),
		BoltDBDaoOption(),
		RegistryOption(&registry),
	)
	rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
	require.NoError(registry.Register(rolldpos.ProtocolID, rp))
	v := vote.NewProtocol(bc)
	require.NoError(registry.Register(vote.ProtocolID, v))
	bc.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(bc, genesis.Default.ActionGasLimit))
	bc.Validator().AddActionValidators(acc, v)
	sf.AddActionHandlers(acc, v)
	require.NoError(bc.Start(ctx))
	require.NoError(addCreatorToFactory(sf))

	ms := &MockSubscriber{counter: 0}
	err = bc.AddSubscriber(ms)
	require.NoError(err)
	require.Equal(0, ms.Counter())

	height := bc.TipHeight()
	fmt.Printf("Open blockchain pass, height = %d\n", height)
	require.Nil(addTestingTsfBlocks(bc))
	err = bc.Stop(ctx)
	require.NoError(err)
	require.Equal(22, ms.Counter())

	// Load a blockchain from DB
	sf, err = factory.NewFactory(cfg, factory.DefaultTrieOption())
	require.NoError(err)
	accountProtocol := account.NewProtocol()
	sf.AddActionHandlers(accountProtocol)
	registry = protocol.Registry{}
	require.NoError(registry.Register(account.ProtocolID, accountProtocol))
	bc = NewBlockchain(
		cfg,
		PrecreatedStateFactoryOption(sf),
		BoltDBDaoOption(),
	)
	voteProtocol := vote.NewProtocol(bc)
	require.NoError(registry.Register(vote.ProtocolID, voteProtocol))
	rolldposProtocol := rolldpos.NewProtocol(
		genesis.Default.NumCandidateDelegates,
		genesis.Default.NumDelegates,
		genesis.Default.NumSubEpochs,
	)
	require.NoError(registry.Register(rolldpos.ProtocolID, rolldposProtocol))
	rewardingProtocol := rewarding.NewProtocol(bc, rolldposProtocol)
	require.NoError(registry.Register(rewarding.ProtocolID, rewardingProtocol))
	bc.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(bc, 0))
	bc.Validator().AddActionValidators(accountProtocol, voteProtocol)
	require.NoError(bc.Start(ctx))
	defer func() {
		require.NoError(bc.Stop(ctx))
	}()

	hash1, err := bc.GetHashByHeight(1)
	require.NoError(err)
	height, err = bc.GetHeightByHash(hash1)
	require.NoError(err)
	require.Equal(uint64(1), height)
	blk, err := bc.GetBlockByHash(hash1)
	require.NoError(err)
	require.Equal(hash1, blk.HashBlock())
	fmt.Printf("block 1 hash = %x\n", hash1)

	hash2, err := bc.GetHashByHeight(2)
	require.NoError(err)
	height, err = bc.GetHeightByHash(hash2)
	require.NoError(err)
	require.Equal(uint64(2), height)
	blk, err = bc.GetBlockByHash(hash2)
	require.NoError(err)
	require.Equal(hash2, blk.HashBlock())
	fmt.Printf("block 2 hash = %x\n", hash2)

	hash3, err := bc.GetHashByHeight(3)
	require.NoError(err)
	height, err = bc.GetHeightByHash(hash3)
	require.NoError(err)
	require.Equal(uint64(3), height)
	blk, err = bc.GetBlockByHash(hash3)
	require.NoError(err)
	require.Equal(hash3, blk.HashBlock())
	fmt.Printf("block 3 hash = %x\n", hash3)

	hash4, err := bc.GetHashByHeight(4)
	require.NoError(err)
	height, err = bc.GetHeightByHash(hash4)
	require.NoError(err)
	require.Equal(uint64(4), height)
	blk, err = bc.GetBlockByHash(hash4)
	require.NoError(err)
	require.Equal(hash4, blk.HashBlock())
	fmt.Printf("block 4 hash = %x\n", hash4)

	hash5, err := bc.GetHashByHeight(5)
	require.NoError(err)
	height, err = bc.GetHeightByHash(hash5)
	require.NoError(err)
	require.Equal(uint64(5), height)
	blk, err = bc.GetBlockByHash(hash5)
	require.NoError(err)
	require.Equal(hash5, blk.HashBlock())
	fmt.Printf("block 5 hash = %x\n", hash5)

	empblk, err := bc.GetBlockByHash(hash.ZeroHash256)
	require.Nil(empblk)
	require.NotNil(err.Error())

	blk, err = bc.GetBlockByHeight(60000)
	require.Nil(blk)
	require.Error(err)

	// add wrong blocks
	h := bc.TipHeight()
	blkhash := bc.TipHash()
	blk, err = bc.GetBlockByHeight(h)
	require.NoError(err)
	require.Equal(blkhash, blk.HashBlock())
	fmt.Printf("Current tip = %d hash = %x\n", h, blkhash)

	// add block with wrong height
	selp, err := testutil.SignedTransfer(ta.Addrinfo["bravo"].String(), ta.Keyinfo["producer"].PriKey, 1, big.NewInt(50), nil, genesis.Default.ActionGasLimit, big.NewInt(0))
	require.NoError(err)

	nblk, err := block.NewTestingBuilder().
		SetHeight(h+2).
		SetPrevBlockHash(blkhash).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(selp).SignAndBuild(ta.Keyinfo["bravo"].PubKey, ta.Keyinfo["bravo"].PriKey)
	require.NoError(err)

	err = bc.ValidateBlock(&nblk)
	require.Error(err)
	fmt.Printf("Cannot validate block %d: %v\n", blk.Height(), err)

	// add block with zero prev hash
	selp2, err := testutil.SignedTransfer(ta.Addrinfo["bravo"].String(), ta.Keyinfo["producer"].PriKey, 1, big.NewInt(50), nil, genesis.Default.ActionGasLimit, big.NewInt(0))
	require.NoError(err)

	nblk, err = block.NewTestingBuilder().
		SetHeight(h+1).
		SetPrevBlockHash(hash.ZeroHash256).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(selp2).SignAndBuild(ta.Keyinfo["bravo"].PubKey, ta.Keyinfo["bravo"].PriKey)
	require.NoError(err)
	err = bc.ValidateBlock(&nblk)
	require.Error(err)
	fmt.Printf("Cannot validate block %d: %v\n", blk.Height(), err)

	// add existing block again will have no effect
	blk, err = bc.GetBlockByHeight(3)
	require.NotNil(blk)
	require.NoError(err)
	require.NoError(bc.(*blockchain).commitBlock(blk))
	fmt.Printf("Cannot add block 3 again: %v\n", err)

	// check all Tx from block 4
	blk, err = bc.GetBlockByHeight(5)
	require.NoError(err)
	require.Equal(hash5, blk.HashBlock())
	_, err = bc.StateByAddr("")
	require.Error(err)
}

func TestLoadBlockchainfromDBWithoutExplorer(t *testing.T) {
	require := require.New(t)

	testTrieFile, _ := ioutil.TempFile(os.TempDir(), "trie")
	testTriePath := testTrieFile.Name()
	testDBFile, _ := ioutil.TempFile(os.TempDir(), "db")
	testDBPath := testDBFile.Name()

	ctx := context.Background()
	cfg := config.Default
	cfg.DB.UseBadgerDB = false // test with boltDB
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath

	sf, err := factory.NewFactory(cfg, factory.DefaultTrieOption())
	require.NoError(err)
	sf.AddActionHandlers(account.NewProtocol())
	// Create a blockchain from scratch
	registry := protocol.Registry{}
	acc := account.NewProtocol()
	require.NoError(registry.Register(account.ProtocolID, acc))
	rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
	require.NoError(registry.Register(rolldpos.ProtocolID, rp))
	bc := NewBlockchain(
		cfg,
		PrecreatedStateFactoryOption(sf),
		BoltDBDaoOption(),
		RegistryOption(&registry),
	)
	v := vote.NewProtocol(bc)
	require.NoError(registry.Register(vote.ProtocolID, v))
	bc.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(bc, genesis.Default.ActionGasLimit))
	bc.Validator().AddActionValidators(acc, v)
	sf.AddActionHandlers(acc, v)
	require.NoError(bc.Start(ctx))

	require.NoError(addCreatorToFactory(sf))

	ms := &MockSubscriber{counter: 0}
	err = bc.AddSubscriber(ms)
	require.NoError(err)
	require.Equal(0, ms.counter)
	err = bc.RemoveSubscriber(ms)
	require.NoError(err)

	height := bc.TipHeight()
	fmt.Printf("Open blockchain pass, height = %d\n", height)
	require.Nil(addTestingTsfBlocks(bc))
	err = bc.Stop(ctx)
	require.NoError(err)
	require.Equal(0, ms.counter)

	// Load a blockchain from DB
	sf, err = factory.NewFactory(cfg, factory.DefaultTrieOption())
	require.NoError(err)
	sf.AddActionHandlers(account.NewProtocol())

	bc = NewBlockchain(cfg, PrecreatedStateFactoryOption(sf), BoltDBDaoOption())
	bc.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(bc, 0))
	bc.Validator().AddActionValidators(account.NewProtocol(), vote.NewProtocol(bc))
	require.NoError(bc.Start(ctx))
	defer func() {
		err := bc.Stop(ctx)
		require.NoError(err)
	}()
	require.NotNil(bc)
	// check hash<-->height mapping
	hash1, err := bc.GetHashByHeight(1)
	require.NoError(err)
	height, err = bc.GetHeightByHash(hash1)
	require.NoError(err)
	require.Equal(uint64(1), height)
	blk, err := bc.GetBlockByHash(hash1)
	require.NoError(err)
	require.Equal(hash1, blk.HashBlock())
	fmt.Printf("block 1 hash = %x\n", hash1)
	hash2, err := bc.GetHashByHeight(2)
	require.NoError(err)
	height, err = bc.GetHeightByHash(hash2)
	require.NoError(err)
	require.Equal(uint64(2), height)
	blk, err = bc.GetBlockByHash(hash2)
	require.NoError(err)
	require.Equal(hash2, blk.HashBlock())
	fmt.Printf("block 2 hash = %x\n", hash2)
	hash3, err := bc.GetHashByHeight(3)
	require.NoError(err)
	height, err = bc.GetHeightByHash(hash3)
	require.NoError(err)
	require.Equal(uint64(3), height)
	blk, err = bc.GetBlockByHash(hash3)
	require.NoError(err)
	require.Equal(hash3, blk.HashBlock())
	fmt.Printf("block 3 hash = %x\n", hash3)
	hash4, err := bc.GetHashByHeight(4)
	require.NoError(err)
	height, err = bc.GetHeightByHash(hash4)
	require.NoError(err)
	require.Equal(uint64(4), height)
	blk, err = bc.GetBlockByHash(hash4)
	require.NoError(err)
	require.Equal(hash4, blk.HashBlock())
	fmt.Printf("block 4 hash = %x\n", hash4)
	empblk, err := bc.GetBlockByHash(hash.ZeroHash256)
	require.Nil(empblk)
	require.NotNil(err.Error())
	blk, err = bc.GetBlockByHeight(60000)
	require.Nil(blk)
	require.Error(err)
	// add wrong blocks
	h := bc.TipHeight()
	blkhash := bc.TipHash()
	blk, err = bc.GetBlockByHeight(h)
	require.NoError(err)
	require.Equal(blkhash, blk.HashBlock())
	fmt.Printf("Current tip = %d hash = %x\n", h, blkhash)
	// add block with wrong height
	selp, err := testutil.SignedTransfer(ta.Addrinfo["bravo"].String(), ta.Keyinfo["producer"].PriKey, 1, big.NewInt(50), nil, genesis.Default.ActionGasLimit, big.NewInt(0))
	require.NoError(err)

	nblk, err := block.NewTestingBuilder().
		SetHeight(h+2).
		SetPrevBlockHash(blkhash).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(selp).SignAndBuild(ta.Keyinfo["bravo"].PubKey, ta.Keyinfo["bravo"].PriKey)
	require.NoError(err)

	err = bc.ValidateBlock(&nblk)
	require.Error(err)
	fmt.Printf("Cannot validate block %d: %v\n", blk.Height(), err)
	// add block with zero prev hash
	selp2, err := testutil.SignedTransfer(ta.Addrinfo["bravo"].String(), ta.Keyinfo["producer"].PriKey, 1, big.NewInt(50), nil, genesis.Default.ActionGasLimit, big.NewInt(0))
	require.NoError(err)

	nblk, err = block.NewTestingBuilder().
		SetHeight(h+1).
		SetPrevBlockHash(hash.ZeroHash256).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(selp2).SignAndBuild(ta.Keyinfo["bravo"].PubKey, ta.Keyinfo["bravo"].PriKey)
	require.NoError(err)
	err = bc.ValidateBlock(&nblk)
	require.Error(err)
	fmt.Printf("Cannot validate block %d: %v\n", blk.Height(), err)
	// add existing block again will have no effect
	blk, err = bc.GetBlockByHeight(3)
	require.NotNil(blk)
	require.NoError(err)
	require.NoError(bc.(*blockchain).commitBlock(blk))
	fmt.Printf("Cannot add block 3 again: %v\n", err)
	// check all Tx from block 4
	blk, err = bc.GetBlockByHeight(4)
	require.NoError(err)
	require.Equal(hash4, blk.HashBlock())
	_, err = bc.StateByAddr("")
	require.Error(err)
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

	testTrieFile, _ := ioutil.TempFile(os.TempDir(), "trie")
	testTriePath := testTrieFile.Name()
	testDBFile, _ := ioutil.TempFile(os.TempDir(), "db")
	testDBPath := testDBFile.Name()

	cfg := config.Default
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath

	sf, err := factory.NewFactory(cfg, factory.DefaultTrieOption())
	require.NoError(err)
	voteProtocol := vote.NewProtocol(nil)
	accountProtocol := account.NewProtocol()
	sf.AddActionHandlers(accountProtocol, voteProtocol)
	registry := protocol.Registry{}
	require.NoError(registry.Register(account.ProtocolID, accountProtocol))
	require.NoError(registry.Register(vote.ProtocolID, voteProtocol))
	bc := NewBlockchain(
		cfg,
		PrecreatedStateFactoryOption(sf),
		BoltDBDaoOption(),
		RegistryOption(&registry),
	)
	rolldposProtocol := rolldpos.NewProtocol(
		genesis.Default.NumCandidateDelegates,
		genesis.Default.NumDelegates,
		genesis.Default.NumSubEpochs,
	)
	require.NoError(registry.Register(rolldpos.ProtocolID, rolldposProtocol))
	rewardingProtocol := rewarding.NewProtocol(bc, rolldposProtocol)
	require.NoError(registry.Register(rewarding.ProtocolID, rewardingProtocol))
	require.NoError(bc.Start(context.Background()))
	defer func() {
		require.NoError(bc.Stop(context.Background()))
	}()
	candidate, err := sf.CandidatesByHeight(0)
	require.NoError(err)
	require.Equal(24, len(candidate))
}

func TestBlockchain_StateByAddr(t *testing.T) {
	require := require.New(t)

	cfg := config.Default
	// disable account-based testing
	// create chain

	bc := NewBlockchain(cfg, InMemDaoOption(), InMemStateFactoryOption())
	require.NoError(bc.Start(context.Background()))
	require.NotNil(bc)
	_, err := bc.CreateState(identityset.Address(0).String(), big.NewInt(100))
	require.NoError(err)
	s, err := bc.StateByAddr(identityset.Address(0).String())
	require.NoError(err)
	require.Equal(uint64(0), s.Nonce)
	require.Equal(big.NewInt(100), s.Balance)
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

	testTrieFile, _ := ioutil.TempFile(os.TempDir(), "trie")
	testTriePath := testTrieFile.Name()
	testDBFile, _ := ioutil.TempFile(os.TempDir(), "db")
	testDBPath := testDBFile.Name()

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
	_, err = accountutil.LoadOrCreateAccount(ws, a, big.NewInt(100000))
	require.NoError(err)
	_, err = accountutil.LoadOrCreateAccount(ws, c, big.NewInt(100000))
	require.NoError(err)
	gasLimit := testutil.TestGasLimit
	ctx := protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			Producer: ta.Addrinfo["producer"],
			GasLimit: gasLimit,
		})
	_, err = ws.RunActions(ctx, 0, nil)
	require.NoError(err)
	require.NoError(sf.Commit(ws))

	for i := 0; i < 10; i++ {
		acts := []action.SealedEnvelope{}
		actionMap := make(map[string][]action.SealedEnvelope)
		actionMap[a] = []action.SealedEnvelope{}
		for i := 0; i < 1000; i++ {
			tsf, err := testutil.SignedTransfer(c, priKeyA, 1, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
			require.NoError(err)
			acts = append(acts, tsf)
			actionMap[a] = append(actionMap[a], tsf)
		}
		blk, _ := bc.MintNewBlock(
			actionMap,
			testutil.TimestampNow(),
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

	testTrieFile, _ := ioutil.TempFile(os.TempDir(), "trie")
	testTriePath := testTrieFile.Name()
	testDBFile, _ := ioutil.TempFile(os.TempDir(), "db")
	testDBPath := testDBFile.Name()

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
	_, err = accountutil.LoadOrCreateAccount(ws, a, big.NewInt(100000))
	require.NoError(err)
	_, err = accountutil.LoadOrCreateAccount(ws, c, big.NewInt(100000))
	require.NoError(err)
	gasLimit := testutil.TestGasLimit
	ctx := protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			Producer: ta.Addrinfo["producer"],
			GasLimit: gasLimit,
		})
	_, err = ws.RunActions(ctx, 0, nil)
	require.NoError(err)
	require.NoError(sf.Commit(ws))

	val := &validator{sf: sf, validatorAddr: ""}
	bc.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(bc, 0))
	bc.Validator().AddActionValidators(account.NewProtocol(), vote.NewProtocol(bc))
	actionMap := make(map[string][]action.SealedEnvelope)
	for i := 0; i < 5000; i++ {
		tsf, err := testutil.SignedTransfer(c, priKeyA, 1, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
		require.NoError(err)
		actionMap[a] = append(actionMap[a], tsf)

		vote, err := testutil.SignedVote(a, priKeyA, 1, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
		require.NoError(err)
		actionMap[a] = append(actionMap[a], vote)
	}
	blk, _ := bc.MintNewBlock(
		actionMap,
		testutil.TimestampNow(),
	)
	require.Nil(val.Validate(blk, 0, blk.PrevHash()))
}

func addCreatorToFactory(sf factory.Factory) error {
	ws, err := sf.NewWorkingSet()
	if err != nil {
		return err
	}
	if _, err = accountutil.LoadOrCreateAccount(
		ws,
		ta.Addrinfo["producer"].String(),
		unit.ConvertIotxToRau(10000000000),
	); err != nil {
		return err
	}
	gasLimit := testutil.TestGasLimit
	ctx := protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			Producer: ta.Addrinfo["producer"],
			GasLimit: gasLimit,
		})
	if _, err = ws.RunActions(ctx, 0, nil); err != nil {
		return err
	}
	return sf.Commit(ws)
}
