// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package api

import (
	"context"
	"encoding/hex"
	"math/big"
	"testing"
	"time"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/execution"
	"github.com/iotexproject/iotex-core/action/protocol/poll"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/blockindex"
	"github.com/iotexproject/iotex-core/consensus"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
)

type testConfig struct {
	api       Config
	genesis   genesis.Genesis
	actPoll   actpool.Config
	chain     blockchain.Config
	consensus consensus.Config
	db        db.Config
	indexer   blockindex.Config
}

var (
	_testTransfer1, _ = action.SignedTransfer(identityset.Address(30).String(), identityset.PrivateKey(27), 1,
		big.NewInt(10), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	_transferHash1, _ = _testTransfer1.Hash()
	_testTransfer2, _ = action.SignedTransfer(identityset.Address(30).String(), identityset.PrivateKey(30), 5,
		big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	_transferHash2, _ = _testTransfer2.Hash()

	_testExecution1, _ = action.SignedExecution(identityset.Address(31).String(), identityset.PrivateKey(30), 6,
		big.NewInt(1), testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64), []byte{1})
	_executionHash1, _ = _testExecution1.Hash()
	_testExecution3, _ = action.SignedExecution(identityset.Address(31).String(), identityset.PrivateKey(28), 2,
		big.NewInt(1), testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64), []byte{1})
	_executionHash3, _ = _testExecution3.Hash()

	_blkHash      = map[uint64]string{}
	_implicitLogs = map[hash.Hash256]*block.TransactionLog{}
)

func addTestingBlocks(bc blockchain.Blockchain, ap actpool.ActPool) error {
	ctx := context.Background()
	addr0 := identityset.Address(27).String()
	addr1 := identityset.Address(28).String()
	addr2 := identityset.Address(29).String()
	addr3 := identityset.Address(30).String()
	priKey3 := identityset.PrivateKey(30)
	addr4 := identityset.Address(31).String()
	// Add block 1
	// Producer transfer--> C
	_implicitLogs[_transferHash1] = block.NewTransactionLog(_transferHash1,
		[]*block.TokenTxRecord{block.NewTokenTxRecord(iotextypes.TransactionLogType_NATIVE_TRANSFER, "10", addr0, addr3)},
	)

	blk1Time := testutil.TimestampNow()
	if err := ap.Add(ctx, _testTransfer1); err != nil {
		return err
	}
	blk, err := bc.MintNewBlock(blk1Time)
	if err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}
	ap.Reset()
	h := blk.HashBlock()
	_blkHash[1] = hex.EncodeToString(h[:])

	// Add block 2
	// Charlie transfer--> A, B, D, P
	// Charlie transfer--> C
	// Charlie exec--> D
	recipients := []string{addr1, addr2, addr4, addr0}
	for i, recipient := range recipients {
		selp, err := action.SignedTransfer(recipient, priKey3, uint64(i+1), big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
		if err != nil {
			return err
		}
		if err := ap.Add(ctx, selp); err != nil {
			return err
		}
		selpHash, err := selp.Hash()
		if err != nil {
			return err
		}
		_implicitLogs[selpHash] = block.NewTransactionLog(selpHash,
			[]*block.TokenTxRecord{block.NewTokenTxRecord(iotextypes.TransactionLogType_NATIVE_TRANSFER, "1", addr3, recipient)},
		)
	}
	_implicitLogs[_transferHash2] = block.NewTransactionLog(_transferHash2,
		[]*block.TokenTxRecord{block.NewTokenTxRecord(iotextypes.TransactionLogType_NATIVE_TRANSFER, "2", addr3, addr3)},
	)
	if err := ap.Add(ctx, _testTransfer2); err != nil {
		return err
	}
	_implicitLogs[_executionHash1] = block.NewTransactionLog(
		_executionHash1,
		[]*block.TokenTxRecord{block.NewTokenTxRecord(iotextypes.TransactionLogType_IN_CONTRACT_TRANSFER, "1", addr3, addr4)},
	)
	if err := ap.Add(ctx, _testExecution1); err != nil {
		return err
	}
	if blk, err = bc.MintNewBlock(blk1Time.Add(time.Second)); err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}
	ap.Reset()
	h = blk.HashBlock()
	_blkHash[2] = hex.EncodeToString(h[:])

	// Add block 3
	// Empty actions
	if blk, err = bc.MintNewBlock(blk1Time.Add(time.Second * 2)); err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}
	ap.Reset()
	h = blk.HashBlock()
	_blkHash[3] = hex.EncodeToString(h[:])

	// Add block 4
	// Charlie transfer--> C
	// Alfa transfer--> A
	// Charlie exec--> D
	// Alfa exec--> D
	tsf1, err := action.SignedTransfer(addr3, priKey3, uint64(7), big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	tsf1Hash, err := tsf1.Hash()
	if err != nil {
		return err
	}
	_implicitLogs[tsf1Hash] = block.NewTransactionLog(tsf1Hash,
		[]*block.TokenTxRecord{block.NewTokenTxRecord(iotextypes.TransactionLogType_NATIVE_TRANSFER, "1", addr3, addr3)},
	)
	if err := ap.Add(ctx, tsf1); err != nil {
		return err
	}
	tsf2, err := action.SignedTransfer(addr1, identityset.PrivateKey(28), uint64(1), big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	tsf2Hash, err := tsf2.Hash()
	if err != nil {
		return err
	}
	_implicitLogs[tsf2Hash] = block.NewTransactionLog(tsf2Hash,
		[]*block.TokenTxRecord{block.NewTokenTxRecord(iotextypes.TransactionLogType_NATIVE_TRANSFER, "1", addr1, addr1)},
	)
	if err := ap.Add(ctx, tsf2); err != nil {
		return err
	}
	execution1, err := action.SignedExecution(addr4, priKey3, 8,
		big.NewInt(2), testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64), []byte{1})
	if err != nil {
		return err
	}
	execution1Hash, err := execution1.Hash()
	if err != nil {
		return err
	}
	_implicitLogs[execution1Hash] = block.NewTransactionLog(
		execution1Hash,
		[]*block.TokenTxRecord{block.NewTokenTxRecord(iotextypes.TransactionLogType_IN_CONTRACT_TRANSFER, "2", addr3, addr4)},
	)
	if err := ap.Add(ctx, execution1); err != nil {
		return err
	}
	_implicitLogs[_executionHash3] = block.NewTransactionLog(
		_executionHash3,
		[]*block.TokenTxRecord{block.NewTokenTxRecord(iotextypes.TransactionLogType_IN_CONTRACT_TRANSFER, "1", addr1, addr4)},
	)
	if err := ap.Add(ctx, _testExecution3); err != nil {
		return err
	}
	if blk, err = bc.MintNewBlock(blk1Time.Add(time.Second * 3)); err != nil {
		return err
	}
	h = blk.HashBlock()
	_blkHash[4] = hex.EncodeToString(h[:])
	return bc.CommitBlock(blk)
}

func deployContractV2(bc blockchain.Blockchain, dao blockdao.BlockDAO, actPool actpool.ActPool, key crypto.PrivateKey, nonce, height uint64, code string) (string, error) {
	data, _ := hex.DecodeString(code)
	ex1, err := action.SignedExecution(action.EmptyAddress, key, nonce, big.NewInt(0), 500000, big.NewInt(testutil.TestGasPriceInt64), data)
	if err != nil {
		return "", err
	}
	if err := actPool.Add(context.Background(), ex1); err != nil {
		return "", err
	}
	blk, err := bc.MintNewBlock(testutil.TimestampNow())
	if err != nil {
		return "", err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return "", err
	}
	actPool.Reset()
	// get deployed contract address
	var contract string
	if dao != nil {
		ex1Hash, err := ex1.Hash()
		if err != nil {
			return "", err
		}
		r, err := dao.GetReceiptByActionHash(ex1Hash, height+1)
		if err != nil {
			return "", err
		}
		contract = r.ContractAddress
	}
	return contract, nil
}

func addActsToActPool(ctx context.Context, ap actpool.ActPool) error {
	// Producer transfer--> A
	tsf1, err := action.SignedTransfer(identityset.Address(28).String(), identityset.PrivateKey(27), 2, big.NewInt(20), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	// Producer transfer--> P
	tsf2, err := action.SignedTransfer(identityset.Address(27).String(), identityset.PrivateKey(27), 3, big.NewInt(20), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	// Producer transfer--> B
	tsf3, err := action.SignedTransfer(identityset.Address(29).String(), identityset.PrivateKey(27), 4, big.NewInt(20), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	// Producer exec--> D
	execution1, err := action.SignedExecution(identityset.Address(31).String(), identityset.PrivateKey(27), 5,
		big.NewInt(1), testutil.TestGasLimit, big.NewInt(10), []byte{1})
	if err != nil {
		return err
	}

	if err := ap.Add(ctx, tsf1); err != nil {
		return err
	}
	if err := ap.Add(ctx, tsf2); err != nil {
		return err
	}
	if err := ap.Add(ctx, tsf3); err != nil {
		return err
	}
	return ap.Add(ctx, execution1)
}

func setupChain(cfg testConfig) (blockchain.Blockchain, blockdao.BlockDAO, blockindex.Indexer, blockindex.BloomFilterIndexer, factory.Factory, actpool.ActPool, *protocol.Registry, string, error) {
	cfg.chain.ProducerPrivKey = hex.EncodeToString(identityset.PrivateKey(0).Bytes())
	registry := protocol.NewRegistry()
	factoryCfg := factory.GenerateConfig(cfg.chain, cfg.genesis)
	sf, err := factory.NewFactory(factoryCfg, db.NewMemKVStore(), factory.RegistryOption(registry))
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, "", err
	}
	ap, err := setupActPool(cfg.genesis, sf, cfg.actPoll)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, "", err
	}
	cfg.genesis.InitBalanceMap[identityset.Address(27).String()] = unit.ConvertIotxToRau(10000000000).String()
	cfg.genesis.InitBalanceMap[identityset.Address(28).String()] = unit.ConvertIotxToRau(10000000000).String()
	// create indexer
	indexer, err := blockindex.NewIndexer(db.NewMemKVStore(), cfg.genesis.Hash())
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, "", errors.New("failed to create indexer")
	}
	testPath, _ := testutil.PathOfTempFile("bloomfilter")
	cfg.db.DbPath = testPath
	bfIndexer, err := blockindex.NewBloomfilterIndexer(db.NewBoltDB(cfg.db), cfg.indexer)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, "", errors.New("failed to create bloomfilter indexer")
	}
	// create BlockDAO
	dao := blockdao.NewBlockDAOInMemForTest([]blockdao.BlockIndexer{sf, indexer, bfIndexer})
	if dao == nil {
		return nil, nil, nil, nil, nil, nil, nil, "", errors.New("failed to create blockdao")
	}
	// create chain
	bc := blockchain.NewBlockchain(
		cfg.chain,
		cfg.genesis,
		dao,
		factory.NewMinter(sf, ap),
		blockchain.BlockValidatorOption(block.NewValidator(
			sf,
			protocol.NewGenericValidator(sf, accountutil.AccountState),
		)),
	)
	if bc == nil {
		return nil, nil, nil, nil, nil, nil, nil, "", errors.New("failed to create blockchain")
	}
	defer func() {
		testutil.CleanupPath(testPath)
	}()

	acc := account.NewProtocol(rewarding.DepositGas)
	evm := execution.NewProtocol(dao.GetBlockHash, rewarding.DepositGas)
	p := poll.NewLifeLongDelegatesProtocol(cfg.genesis.Delegates)
	rolldposProtocol := rolldpos.NewProtocol(
		genesis.Default.NumCandidateDelegates,
		genesis.Default.NumDelegates,
		genesis.Default.NumSubEpochs,
		rolldpos.EnableDardanellesSubEpoch(cfg.genesis.DardanellesBlockHeight, cfg.genesis.DardanellesNumSubEpochs),
	)
	r := rewarding.NewProtocol(cfg.genesis.Rewarding)

	if err := rolldposProtocol.Register(registry); err != nil {
		return nil, nil, nil, nil, nil, nil, nil, "", err
	}
	if err := acc.Register(registry); err != nil {
		return nil, nil, nil, nil, nil, nil, nil, "", err
	}
	if err := evm.Register(registry); err != nil {
		return nil, nil, nil, nil, nil, nil, nil, "", err
	}
	if err := r.Register(registry); err != nil {
		return nil, nil, nil, nil, nil, nil, nil, "", err
	}
	if err := p.Register(registry); err != nil {
		return nil, nil, nil, nil, nil, nil, nil, "", err
	}

	return bc, dao, indexer, bfIndexer, sf, ap, registry, testPath, nil
}

func setupActPool(g genesis.Genesis, sf factory.Factory, cfg actpool.Config) (actpool.ActPool, error) {
	ap, err := actpool.NewActPool(g, sf, cfg)
	if err != nil {
		return nil, err
	}

	ap.AddActionEnvelopeValidators(protocol.NewGenericValidator(sf, accountutil.AccountState))

	return ap, nil
}

func newConfig() testConfig {
	cfg := testConfig{
		api:       DefaultConfig,
		genesis:   genesis.Default,
		actPoll:   actpool.DefaultConfig,
		chain:     blockchain.DefaultConfig,
		consensus: consensus.DefaultConfig,
		db:        db.DefaultConfig,
		indexer:   blockindex.DefaultConfig,
	}

	testTriePath, err := testutil.PathOfTempFile("trie")
	if err != nil {
		panic(err)
	}
	testDBPath, err := testutil.PathOfTempFile("db")
	if err != nil {
		panic(err)
	}
	testIndexPath, err := testutil.PathOfTempFile("index")
	if err != nil {
		panic(err)
	}
	testSystemLogPath, err := testutil.PathOfTempFile("systemlog")
	if err != nil {
		panic(err)
	}
	defer func() {
		testutil.CleanupPath(testTriePath)
		testutil.CleanupPath(testDBPath)
		testutil.CleanupPath(testIndexPath)
		testutil.CleanupPath(testSystemLogPath)
	}()

	cfg.chain.TrieDBPath = testTriePath
	cfg.chain.ChainDBPath = testDBPath
	cfg.chain.IndexDBPath = testIndexPath
	cfg.chain.EVMNetworkID = _evmNetworkID
	cfg.chain.EnableAsyncIndexWrite = false
	cfg.genesis.EnableGravityChainVoting = true
	cfg.actPoll.MinGasPriceStr = "0"
	cfg.api.RangeQueryLimit = 100
	cfg.api.GRPCPort = 0
	cfg.api.HTTPPort = 0
	cfg.api.WebSocketPort = 0
	return cfg
}

func createServerV2(cfg testConfig, needActPool bool) (*ServerV2, blockchain.Blockchain, blockdao.BlockDAO, blockindex.Indexer, *protocol.Registry, actpool.ActPool, string, error) {
	// TODO (zhi): revise
	bc, dao, indexer, bfIndexer, sf, ap, registry, bfIndexFile, err := setupChain(cfg)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, "", err
	}

	ctx := context.Background()

	// Start blockchain
	if err := bc.Start(ctx); err != nil {
		return nil, nil, nil, nil, nil, nil, "", err
	}
	// Add testing blocks
	if err := addTestingBlocks(bc, ap); err != nil {
		return nil, nil, nil, nil, nil, nil, "", err
	}

	if needActPool {
		// Add actions to actpool
		ctx = protocol.WithRegistry(ctx, registry)
		if err := addActsToActPool(ctx, ap); err != nil {
			return nil, nil, nil, nil, nil, nil, "", err
		}
	}
	opts := []Option{WithBroadcastOutbound(func(ctx context.Context, chainID uint32, msg proto.Message) error {
		return nil
	})}
	svr, err := NewServerV2(cfg.api, bc, nil, sf, dao, indexer, bfIndexer, ap, registry, opts...)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, "", err
	}
	return svr, bc, dao, indexer, registry, ap, bfIndexFile, nil
}

func TestServerV2Integrity(t *testing.T) {
	require := require.New(t)
	cfg := newConfig()
	cfg.api.GRPCPort = testutil.RandomPort()
	svr, _, _, _, _, _, bfIndexFile, err := createServerV2(cfg, false)
	require.NoError(err)
	defer func() {
		testutil.CleanupPath(bfIndexFile)
	}()
	ctx := context.Background()

	err = svr.Start(ctx)
	require.NoError(err)

	err = testutil.WaitUntil(100*time.Millisecond, 3*time.Second, func() (bool, error) {
		err = svr.Stop(ctx)
		return err == nil, err
	})
	require.NoError(err)
}
