// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package integrity

import (
	"context"
	"encoding/hex"
	"errors"
	"math/big"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	ethCrypto "github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/execution"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/blockindex"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
)

var (
	_totalActions     = 10
	_contractAddr     string
	_randAccountSize  = 50
	_randAccountsAddr = make([]address.Address, 0)
	_randAccountsPvk  = make([]ethCrypto.PrivateKey, 0)
	userA             = identityset.Address(28)
	priKeyA           = identityset.PrivateKey(28)
	userB             = identityset.Address(29)
	priKeyB           = identityset.PrivateKey(29)
	genesisPriKey     = identityset.PrivateKey(27)
	genesisNonce      uint64
)

func init() {
	for i := 0; i < _randAccountSize; i++ {
		pvk, _ := ethCrypto.GenerateKey()
		_randAccountsPvk = append(_randAccountsPvk, pvk)
		_randAccountsAddr = append(_randAccountsAddr, pvk.PublicKey().Address())
	}
}

func BenchmarkMintAndCommitBlock(b *testing.B) {
	require := require.New(b)
	rand.Seed(time.Now().Unix())
	bc, ap, err := newChainInDB()
	require.NoError(err)
	nonceMap := make(map[string]uint64)

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		err = injectExecution(nonceMap, ap)
		require.NoError(err)

		blk, err := bc.MintNewBlock(testutil.TimestampNow())
		require.NoError(err)
		err = bc.ValidateBlock(blk)
		require.NoError(err)
		err = bc.CommitBlock(blk)
		require.NoError(err)
	}
}

func BenchmarkMintBlock(b *testing.B) {
	require := require.New(b)
	rand.Seed(time.Now().Unix())
	bc, ap, err := newChainInDB()
	require.NoError(err)
	nonceMap := make(map[string]uint64)

	err = injectMultipleAccountsTransfer(nonceMap, ap)
	require.NoError(err)

	for n := 0; n < b.N; n++ {
		_, err := bc.MintNewBlock(testutil.TimestampNow())
		require.NoError(err)
	}
}

func BenchmarkValidateBlock(b *testing.B) {
	require := require.New(b)
	rand.Seed(time.Now().Unix())
	bc, ap, err := newChainInDB()
	require.NoError(err)
	nonceMap := make(map[string]uint64)

	err = injectTransfer(nonceMap, ap)
	require.NoError(err)

	blk, err := bc.MintNewBlock(testutil.TimestampNow())
	require.NoError(err)

	for n := 0; n < b.N; n++ {
		err = bc.ValidateBlock(blk)
		require.NoError(err)
	}
}

func injectTransfer(nonceMap map[string]uint64, ap actpool.ActPool) error {
	for i := 0; i < _totalActions/2; i++ {
		nonceMap[userA.String()]++
		tsf1, err := action.SignedTransfer(userB.String(), priKeyA, nonceMap[userA.String()], big.NewInt(int64(rand.Intn(2))), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
		if err != nil {
			return err
		}
		err = ap.Add(context.Background(), tsf1)
		if err != nil {
			return err
		}
		nonceMap[userB.String()]++
		tsf2, err := action.SignedTransfer(userA.String(), priKeyB, nonceMap[userB.String()], big.NewInt(int64(rand.Intn(2))), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
		if err != nil {
			return err
		}
		err = ap.Add(context.Background(), tsf2)
		if err != nil {
			return err
		}
	}
	return nil
}

func injectMultipleAccountsTransfer(nonceMap map[string]uint64, ap actpool.ActPool) error {
	rand.Seed(time.Now().UnixNano())
	for i := 0; i < _totalActions; i++ {
		senderIdx := rand.Intn(_randAccountSize)
		recipientIdx := senderIdx
		for ; recipientIdx != senderIdx; recipientIdx = rand.Intn(_randAccountSize) {
		}
		nonceMap[_randAccountsAddr[senderIdx].String()]++
		tsf, err := action.SignedTransfer(
			_randAccountsAddr[recipientIdx].String(),
			_randAccountsPvk[senderIdx],
			nonceMap[_randAccountsAddr[senderIdx].String()],
			big.NewInt(int64(rand.Intn(2))),
			[]byte{},
			testutil.TestGasLimit,
			big.NewInt(testutil.TestGasPriceInt64))
		if err != nil {
			return err
		}
		if err = ap.Add(context.Background(), tsf); err != nil {
			return err
		}
	}
	return nil
}

// Todo: get precise gaslimit by estimateGas
func injectExecution(nonceMap map[string]uint64, ap actpool.ActPool) error {
	for i := 0; i < _totalActions; i++ {
		genesisNonce++
		ex1, err := action.SignedExecution(_contractAddr, genesisPriKey, genesisNonce, big.NewInt(0), 2e9, big.NewInt(testutil.TestGasPriceInt64), opMultiSend)
		if err != nil {
			return err
		}
		err = ap.Add(context.Background(), ex1)
		if err != nil {
			return err
		}
	}
	return nil
}

func newChainInDB() (blockchain.Blockchain, actpool.ActPool, error) {
	cfg := config.Default
	testTriePath, err := testutil.PathOfTempFile("trie")
	if err != nil {
		return nil, nil, err
	}
	testDBPath, err := testutil.PathOfTempFile("db")
	if err != nil {
		return nil, nil, err
	}
	testIndexPath, err := testutil.PathOfTempFile("index")
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		testutil.CleanupPath(testTriePath)
		testutil.CleanupPath(testDBPath)
		testutil.CleanupPath(testIndexPath)
	}()

	cfg.DB.DbPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Chain.IndexDBPath = testIndexPath
	cfg.Chain.EnableArchiveMode = true
	cfg.Consensus.Scheme = config.RollDPoSScheme
	cfg.Genesis.BlockGasLimit = config.Default.Genesis.BlockGasLimit * 100
	cfg.ActPool.MinGasPriceStr = "0"
	cfg.ActPool.MaxNumActsPerAcct = 1000000000
	cfg.Genesis.EnableGravityChainVoting = false
	registry := protocol.NewRegistry()
	var sf factory.Factory
	kv := db.NewBoltDB(cfg.DB)
	sf, err = factory.NewStateDB(cfg, factory.PrecreatedStateDBOption(kv), factory.RegistryStateDBOption(registry))
	if err != nil {
		return nil, nil, err
	}

	ap, err := actpool.NewActPool(sf, cfg.ActPool)
	if err != nil {
		return nil, nil, err
	}
	acc := account.NewProtocol(rewarding.DepositGas)
	if err = acc.Register(registry); err != nil {
		return nil, nil, err
	}
	rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
	if err = rp.Register(registry); err != nil {
		return nil, nil, err
	}
	var indexer blockindex.Indexer
	indexers := []blockdao.BlockIndexer{sf}
	if _, gateway := cfg.Plugins[config.GatewayPlugin]; gateway && !cfg.Chain.EnableAsyncIndexWrite {
		// create indexer
		cfg.DB.DbPath = cfg.Chain.IndexDBPath
		indexer, err = blockindex.NewIndexer(db.NewBoltDB(cfg.DB), cfg.Genesis.Hash())
		if err != nil {
			return nil, nil, err
		}
		indexers = append(indexers, indexer)
	}
	cfg.Genesis.InitBalanceMap[identityset.Address(27).String()] = unit.ConvertIotxToRau(1000000000000).String()
	// create BlockDAO
	cfg.DB.DbPath = cfg.Chain.ChainDBPath
	cfg.DB.CompressLegacy = cfg.Chain.CompressBlock
	dao := blockdao.NewBlockDAO(indexers, cfg.DB)
	if dao == nil {
		return nil, nil, errors.New("pointer is nil")
	}
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
		return nil, nil, errors.New("pointer is nil")
	}
	ep := execution.NewProtocol(dao.GetBlockHash, rewarding.DepositGas)
	if err = ep.Register(registry); err != nil {
		return nil, nil, err
	}
	if err = bc.Start(context.Background()); err != nil {
		return nil, nil, err
	}

	genesisNonce = 0

	// make a transfer from genesisAccount to a and b,because stateTX cannot store data in height 0
	genesisNonce++
	tsf, err := action.SignedTransfer(userA.String(), genesisPriKey, genesisNonce, big.NewInt(1e17), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return nil, nil, err
	}
	if err = ap.Add(context.Background(), tsf); err != nil {
		return nil, nil, err
	}
	genesisNonce++
	tsf2, err := action.SignedTransfer(userB.String(), genesisPriKey, genesisNonce, big.NewInt(1e17), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return nil, nil, err
	}
	if err = ap.Add(context.Background(), tsf2); err != nil {
		return nil, nil, err
	}
	for i := 0; i < _randAccountSize; i++ {
		genesisNonce++
		tsf, err := action.SignedTransfer(_randAccountsAddr[i].String(), genesisPriKey, genesisNonce, big.NewInt(1e16), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
		if err != nil {
			return nil, nil, err
		}
		if err = ap.Add(context.Background(), tsf); err != nil {
			return nil, nil, err
		}
	}
	// deploy contract
	data, err := hex.DecodeString(_ERC20contractByteCode)
	// data, err := hex.DecodeString(_contractByteCode)
	if err != nil {
		return nil, nil, err
	}
	genesisNonce++
	ex1, err := action.SignedExecution(action.EmptyAddress, genesisPriKey, genesisNonce, big.NewInt(0), 3e8, big.NewInt(testutil.TestGasPriceInt64), data)
	if err != nil {
		return nil, nil, err
	}
	if err := ap.Add(context.Background(), ex1); err != nil {
		return nil, nil, err
	}

	blk, err := bc.MintNewBlock(testutil.TimestampNow())
	if err != nil {
		return nil, nil, err
	}
	if err = bc.CommitBlock(blk); err != nil {
		return nil, nil, err
	}

	ex1Hash, err := ex1.Hash()
	if err != nil {
		return nil, nil, err
	}
	r, err := dao.GetReceiptByActionHash(ex1Hash, 1)
	if err != nil {
		return nil, nil, err
	}
	_contractAddr = r.ContractAddress

	return bc, ap, nil
}
