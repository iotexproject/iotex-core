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
	"testing"

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
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/blockindex"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/pkg/util/randutil"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
)

var (
	_totalActionPair  = 2000
	_contractByteCode = "60806040526101f4600055603260015534801561001b57600080fd5b506102558061002b6000396000f3fe608060405234801561001057600080fd5b50600436106100365760003560e01c806358931c461461003b5780637f353d5514610045575b600080fd5b61004361004f565b005b61004d610097565b005b60006001905060005b6000548110156100935760028261006f9190610114565b915060028261007e91906100e3565b9150808061008b90610178565b915050610058565b5050565b60005b6001548110156100e057600281908060018154018082558091505060019003906000526020600020016000909190919091505580806100d890610178565b91505061009a565b50565b60006100ee8261016e565b91506100f98361016e565b925082610109576101086101f0565b5b828204905092915050565b600061011f8261016e565b915061012a8361016e565b9250817fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff0483118215151615610163576101626101c1565b5b828202905092915050565b6000819050919050565b60006101838261016e565b91507fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff8214156101b6576101b56101c1565b5b600182019050919050565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052601160045260246000fd5b7f4e487b7100000000000000000000000000000000000000000000000000000000600052601260045260246000fdfea2646970667358221220cb9cada3f1d447c978af17aa3529d6fe4f25f9c5a174085443e371b6940ae99b64736f6c63430008070033"
	_contractAddr     string

	_randAccountSize  = 50
	_randAccountsAddr = make([]address.Address, 0)
	_randAccountsPvk  = make([]ethCrypto.PrivateKey, 0)
	userA             = identityset.Address(28)
	priKeyA           = identityset.PrivateKey(28)
	userB             = identityset.Address(29)
	priKeyB           = identityset.PrivateKey(29)

	opAppend, _ = hex.DecodeString("7f353d55")
	opMul, _    = hex.DecodeString("58931c46")
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
	bc, ap, err := newChainInDB()
	require.NoError(err)
	nonceMap := make(map[string]uint64)

	for n := 0; n < b.N; n++ {
		err = injectMultipleAccountsTransfer(nonceMap, ap)
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
	for i := 0; i < _totalActionPair; i++ {
		nonceMap[userA.String()]++
		tsf1, err := action.SignedTransfer(userB.String(), priKeyA, nonceMap[userA.String()], big.NewInt(int64(randutil.Intn(2))), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
		if err != nil {
			return err
		}
		err = ap.Add(context.Background(), tsf1)
		if err != nil {
			return err
		}
		nonceMap[userB.String()]++
		tsf2, err := action.SignedTransfer(userA.String(), priKeyB, nonceMap[userB.String()], big.NewInt(int64(randutil.Intn(2))), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
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
	for i := 0; i < 2*_totalActionPair; i++ {
		senderIdx := randutil.Intn(_randAccountSize)
		recipientIdx := senderIdx
		for ; recipientIdx != senderIdx; recipientIdx = randutil.Intn(_randAccountSize) {
		}
		nonceMap[_randAccountsAddr[senderIdx].String()]++
		tsf, err := action.SignedTransfer(
			_randAccountsAddr[recipientIdx].String(),
			_randAccountsPvk[senderIdx],
			nonceMap[_randAccountsAddr[senderIdx].String()],
			big.NewInt(int64(randutil.Intn(2))),
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
	for i := 0; i < _totalActionPair; i++ {
		nonceMap[userA.String()]++
		ex1, err := action.SignedExecution(_contractAddr, priKeyA, nonceMap[userA.String()], big.NewInt(0), 2e6, big.NewInt(testutil.TestGasPriceInt64), opAppend)
		if err != nil {
			return err
		}
		err = ap.Add(context.Background(), ex1)
		if err != nil {
			return err
		}
		nonceMap[userB.String()]++
		ex2, err := action.SignedExecution(_contractAddr, priKeyB, nonceMap[userB.String()], big.NewInt(0), 2e6, big.NewInt(testutil.TestGasPriceInt64), opAppend)
		if err != nil {
			return err
		}
		err = ap.Add(context.Background(), ex2)
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
	cfg.Genesis.BlockGasLimit = genesis.Default.BlockGasLimit * 100
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

	ap, err := actpool.NewActPool(cfg.Genesis, sf, cfg.ActPool)
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
	deser := block.NewDeserializer(cfg.Chain.EVMNetworkID)
	dao := blockdao.NewBlockDAO(indexers, cfg.DB, deser)
	if dao == nil {
		return nil, nil, errors.New("pointer is nil")
	}
	bc := blockchain.NewBlockchain(
		cfg.Chain,
		cfg.Genesis,
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

	genesisPriKey := identityset.PrivateKey(27)
	var genesisNonce uint64 = 0

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
		tsf, err := action.SignedTransfer(_randAccountsAddr[i].String(), genesisPriKey, genesisNonce, big.NewInt(1e17), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
		if err != nil {
			return nil, nil, err
		}
		if err = ap.Add(context.Background(), tsf); err != nil {
			return nil, nil, err
		}
	}
	// deploy contract
	data, err := hex.DecodeString(_contractByteCode)
	if err != nil {
		return nil, nil, err
	}
	genesisNonce++
	ex1, err := action.SignedExecution(action.EmptyAddress, genesisPriKey, genesisNonce, big.NewInt(0), 500000, big.NewInt(testutil.TestGasPriceInt64), data)
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
