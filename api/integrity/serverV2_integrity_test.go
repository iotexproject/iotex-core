// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package integrity

import (
	"context"
	"encoding/hex"
	"math/big"
	"testing"
	"time"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/api"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/chainservice"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/p2p"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
)

var (
	testTransfer1, _ = action.SignedTransfer(identityset.Address(30).String(), identityset.PrivateKey(27), 1,
		big.NewInt(10), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	transferHash1, _ = testTransfer1.Hash()
	testTransfer2, _ = action.SignedTransfer(identityset.Address(30).String(), identityset.PrivateKey(30), 5,
		big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	transferHash2, _ = testTransfer2.Hash()

	testExecution1, _ = action.SignedExecution(identityset.Address(31).String(), identityset.PrivateKey(30), 6,
		big.NewInt(1), testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64), []byte{1})
	executionHash1, _ = testExecution1.Hash()
	testExecution3, _ = action.SignedExecution(identityset.Address(31).String(), identityset.PrivateKey(28), 2,
		big.NewInt(1), testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64), []byte{1})
	executionHash3, _ = testExecution3.Hash()

	blkHash      = map[uint64]string{}
	implicitLogs = map[hash.Hash256]*block.TransactionLog{}
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
	implicitLogs[transferHash1] = block.NewTransactionLog(transferHash1,
		[]*block.TokenTxRecord{block.NewTokenTxRecord(iotextypes.TransactionLogType_NATIVE_TRANSFER, "10", addr0, addr3)},
	)

	blk1Time := testutil.TimestampNow()
	if err := ap.Add(ctx, testTransfer1); err != nil {
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
	blkHash[1] = hex.EncodeToString(h[:])

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
		implicitLogs[selpHash] = block.NewTransactionLog(selpHash,
			[]*block.TokenTxRecord{block.NewTokenTxRecord(iotextypes.TransactionLogType_NATIVE_TRANSFER, "1", addr3, recipient)},
		)
	}
	implicitLogs[transferHash2] = block.NewTransactionLog(transferHash2,
		[]*block.TokenTxRecord{block.NewTokenTxRecord(iotextypes.TransactionLogType_NATIVE_TRANSFER, "2", addr3, addr3)},
	)
	if err := ap.Add(ctx, testTransfer2); err != nil {
		return err
	}
	implicitLogs[executionHash1] = block.NewTransactionLog(
		executionHash1,
		[]*block.TokenTxRecord{block.NewTokenTxRecord(iotextypes.TransactionLogType_IN_CONTRACT_TRANSFER, "1", addr3, addr4)},
	)
	if err := ap.Add(ctx, testExecution1); err != nil {
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
	blkHash[2] = hex.EncodeToString(h[:])

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
	blkHash[3] = hex.EncodeToString(h[:])

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
	implicitLogs[tsf1Hash] = block.NewTransactionLog(tsf1Hash,
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
	implicitLogs[tsf2Hash] = block.NewTransactionLog(tsf2Hash,
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
	implicitLogs[execution1Hash] = block.NewTransactionLog(
		execution1Hash,
		[]*block.TokenTxRecord{block.NewTokenTxRecord(iotextypes.TransactionLogType_IN_CONTRACT_TRANSFER, "2", addr3, addr4)},
	)
	if err := ap.Add(ctx, execution1); err != nil {
		return err
	}
	implicitLogs[executionHash3] = block.NewTransactionLog(
		executionHash3,
		[]*block.TokenTxRecord{block.NewTokenTxRecord(iotextypes.TransactionLogType_IN_CONTRACT_TRANSFER, "1", addr1, addr4)},
	)
	if err := ap.Add(ctx, testExecution3); err != nil {
		return err
	}
	if blk, err = bc.MintNewBlock(blk1Time.Add(time.Second * 3)); err != nil {
		return err
	}
	h = blk.HashBlock()
	blkHash[4] = hex.EncodeToString(h[:])
	return bc.CommitBlock(blk)
}

func deployContractV2(cs *chainservice.ChainService, key crypto.PrivateKey, nonce uint64, code string) (string, error) {
	data, _ := hex.DecodeString(code)
	ex1, err := action.SignedExecution(action.EmptyAddress, key, nonce, big.NewInt(0), 500000, big.NewInt(testutil.TestGasPriceInt64), data)
	if err != nil {
		return "", err
	}
	actPool := cs.ActionPool()
	bc := cs.Blockchain()
	dao := cs.BlockDAO()
	height := bc.TipHeight()
	if err := actPool.Add(context.Background(), ex1); err != nil {
		return "", err
	}
	blk, err := cs.Blockchain().MintNewBlock(testutil.TimestampNow())
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
	tsf4, err := action.SignedTransfer(identityset.Address(33).String(), identityset.PrivateKey(33), 1, big.NewInt(0), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	tsf5, err := action.SignedTransfer(identityset.Address(33).String(), identityset.PrivateKey(33), 2, big.NewInt(0), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
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
	if err := ap.Add(ctx, tsf4); err != nil {
		return err
	}
	if err := ap.Add(ctx, tsf5); err != nil {
		return err
	}
	return ap.Add(ctx, execution1)
}

/*
func setupChain(cfg config.Config) (blockchain.Blockchain, blockdao.BlockDAO, blockindex.Indexer, blockindex.BloomFilterIndexer, factory.Factory, actpool.ActPool, *protocol.Registry, string, error) {
	cfg.Chain.ProducerPrivKey = hex.EncodeToString(identityset.PrivateKey(0).Bytes())
	cfg.Genesis.InitBalanceMap[identityset.Address(27).String()] = unit.ConvertIotxToRau(10000000000).String()
	cfg.Genesis.InitBalanceMap[identityset.Address(28).String()] = unit.ConvertIotxToRau(10000000000).String()

		// create indexer
		indexer, err := blockindex.NewIndexer(db.NewMemKVStore(), cfg.Genesis.Hash())
		if err != nil {
			return nil, nil, nil, nil, nil, nil, nil, "", errors.New("failed to create indexer")
		}
		testPath, _ := testutil.PathOfTempFile("bloomfilter")
		cfg.DB.DbPath = testPath
		bfIndexer, err := blockindex.NewBloomfilterIndexer(db.NewBoltDB(cfg.DB), cfg.Indexer)
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
			cfg,
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
		p := poll.NewLifeLongDelegatesProtocol(cfg.Genesis.Delegates)
		rolldposProtocol := rolldpos.NewProtocol(
			genesis.Default.NumCandidateDelegates,
			genesis.Default.NumDelegates,
			genesis.Default.NumSubEpochs,
			rolldpos.EnableDardanellesSubEpoch(cfg.Genesis.DardanellesBlockHeight, cfg.Genesis.DardanellesNumSubEpochs),
		)
		r := rewarding.NewProtocol(cfg.Genesis.Rewarding)

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
*/

func setupActPool(sf factory.Factory, cfg config.ActPool) (actpool.ActPool, error) {
	ap, err := actpool.NewActPool(sf, cfg, actpool.EnableExperimentalActions())
	if err != nil {
		return nil, err
	}

	ap.AddActionEnvelopeValidators(protocol.NewGenericValidator(sf, accountutil.AccountState))

	return ap, nil
}

func newConfig(t *testing.T) config.Config {
	r := require.New(t)
	cfg := config.Default

	testTriePath, err := testutil.PathOfTempFile("trie")
	r.NoError(err)
	testDBPath, err := testutil.PathOfTempFile("db")
	r.NoError(err)
	testIndexPath, err := testutil.PathOfTempFile("index")
	r.NoError(err)
	testSystemLogPath, err := testutil.PathOfTempFile("systemlog")
	r.NoError(err)
	defer func() {
		testutil.CleanupPath(testTriePath)
		testutil.CleanupPath(testDBPath)
		testutil.CleanupPath(testIndexPath)
		testutil.CleanupPath(testSystemLogPath)
	}()

	cfg.Plugins[config.GatewayPlugin] = true
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Chain.IndexDBPath = testIndexPath
	cfg.System.SystemLogDBPath = testSystemLogPath
	cfg.Chain.EnableAsyncIndexWrite = false
	cfg.Genesis.EnableGravityChainVoting = true
	cfg.ActPool.MinGasPriceStr = "0"
	cfg.API.RangeQueryLimit = 100
	cfg.Chain.ProducerPrivKey = hex.EncodeToString(identityset.PrivateKey(0).Bytes())
	cfg.Genesis.InitBalanceMap[identityset.Address(27).String()] = unit.ConvertIotxToRau(10000000000).String()
	cfg.Genesis.InitBalanceMap[identityset.Address(28).String()] = unit.ConvertIotxToRau(10000000000).String()
	cfg.Genesis.PollMode = "lifeLong"
	cfg.Consensus.Scheme = config.RollDPoSScheme
	cfg.Consensus.RollDPoS.ConsensusDBPath = ""

	return cfg
}

func createChainService(cfg config.Config, agent p2p.Agent, needActPool bool) (*chainservice.ChainService, error) {
	builder := chainservice.NewBuilder(cfg)
	cs, err := builder.SetP2PAgent(agent).BuildForTest()
	if err != nil {
		return nil, err
	}
	ctx := context.Background()

	if err := cs.Start(ctx); err != nil {
		return nil, err
	}
	// Add testing blocks
	if err := addTestingBlocks(cs.Blockchain(), cs.ActionPool()); err != nil {
		return nil, err
	}

	if needActPool {
		// Add actions to actpool
		ctx = protocol.WithRegistry(ctx, cs.Registry())
		if err := addActsToActPool(ctx, cs.ActionPool()); err != nil {
			return nil, err
		}
	}

	return cs, nil
}

func TestServerV2Integrity(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	config.SetEVMNetworkID(1)
	builder := chainservice.NewBuilder(cfg)
	cs, err := builder.BuildForTest()
	require.NoError(err)
	svr, err := api.NewServerV2(cfg.API, cs)
	require.NoError(err)

	ctx := context.Background()

	err = svr.Start(ctx)
	require.NoError(err)

	err = testutil.WaitUntil(100*time.Millisecond, 3*time.Second, func() (bool, error) {
		err = svr.Stop(ctx)
		return err == nil, err
	})
	require.NoError(err)
}
