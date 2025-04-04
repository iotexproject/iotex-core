// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package gasstation

import (
	"context"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/account"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/action/protocol/execution"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/v2/actpool"
	"github.com/iotexproject/iotex-core/v2/blockchain"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/v2/blockchain/filedao"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/pkg/unit"
	"github.com/iotexproject/iotex-core/v2/state/factory"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_blockchain"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_blockdao"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

type (
	testConfig struct {
		Genesis    genesis.Genesis
		Chain      blockchain.Config
		ActPool    actpool.Config
		GasStation Config
	}
	testActionGas []struct {
		gasPrice    uint64
		gasConsumed uint64
	}
	testCase struct {
		name           string
		blocks         []testActionGas
		expectGasPrice uint64
	}
)

func TestNewGasStation(t *testing.T) {
	require := require.New(t)
	require.NotNil(NewGasStation(nil, nil, DefaultConfig))
}

func newTestConfig() testConfig {
	cfg := testConfig{
		Genesis:    genesis.TestDefault(),
		Chain:      blockchain.DefaultConfig,
		ActPool:    actpool.DefaultConfig,
		GasStation: DefaultConfig,
	}
	cfg.Genesis.BlockGasLimit = uint64(100000)
	cfg.Genesis.EnableGravityChainVoting = false

	return cfg
}

func TestSuggestGasPriceForUserAction(t *testing.T) {
	ctx := context.Background()
	cfg := newTestConfig()
	registry := protocol.NewRegistry()
	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(t, acc.Register(registry))
	rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
	require.NoError(t, rp.Register(registry))
	factoryCfg := factory.GenerateConfig(cfg.Chain, cfg.Genesis)
	sf, err := factory.NewStateDB(factoryCfg, db.NewMemKVStore(), factory.RegistryStateDBOption(registry))
	require.NoError(t, err)
	ap, err := actpool.NewActPool(cfg.Genesis, sf, cfg.ActPool)
	require.NoError(t, err)
	store, err := filedao.NewFileDAOInMemForTest()
	require.NoError(t, err)
	blkMemDao := blockdao.NewBlockDAOWithIndexersAndCache(store, []blockdao.BlockIndexer{sf}, 16)
	bc := blockchain.NewBlockchain(
		cfg.Chain,
		cfg.Genesis,
		blkMemDao,
		factory.NewMinter(sf, ap),
		blockchain.BlockValidatorOption(block.NewValidator(
			sf,
			protocol.NewGenericValidator(sf, accountutil.AccountState),
		)),
	)
	ep := execution.NewProtocol(blkMemDao.GetBlockHash, rewarding.DepositGas, func(u uint64) (time.Time, error) { return time.Time{}, nil })
	require.NoError(t, ep.Register(registry))
	rewardingProtocol := rewarding.NewProtocol(cfg.Genesis.Rewarding)
	require.NoError(t, rewardingProtocol.Register(registry))
	require.NoError(t, bc.Start(ctx))
	defer func() {
		require.NoError(t, bc.Stop(ctx))
	}()

	for i := 0; i < 30; i++ {
		tsf := action.NewTransfer(big.NewInt(100), identityset.Address(27).String(), []byte{})
		elp1 := (&action.EnvelopeBuilder{}).SetAction(tsf).SetNonce(uint64(i) + 1).SetGasLimit(100000).
			SetGasPrice(big.NewInt(1).Mul(big.NewInt(int64(i)+10), big.NewInt(unit.Qev))).Build()
		selp1, err := action.Sign(elp1, identityset.PrivateKey(0))
		require.NoError(t, err)

		require.NoError(t, ap.Add(context.Background(), selp1))

		blk, err := bc.MintNewBlock(testutil.TimestampNow())
		require.NoError(t, err)
		require.Equal(t, 2, len(blk.Actions))
		require.Equal(t, 2, len(blk.Receipts))
		var gasConsumed uint64
		for _, receipt := range blk.Receipts {
			gasConsumed += receipt.GasConsumed
		}
		require.True(t, gasConsumed <= cfg.Genesis.BlockGasLimit)
		require.NoError(t, bc.CommitBlock(blk))
	}
	height := bc.TipHeight()
	fmt.Printf("Open blockchain pass, height = %d\n", height)

	gs := NewGasStation(bc, blkMemDao, cfg.GasStation)
	require.NotNil(t, gs)

	gp, err := gs.SuggestGasPrice()
	require.NoError(t, err)
	// i from 10 to 29,gasprice for 20 to 39,60%*20+20=31
	require.Equal(
		t,
		big.NewInt(1).Mul(big.NewInt(int64(31)), big.NewInt(unit.Qev)).Uint64()*9/10,
		gp,
	)
}

func TestSuggestGasPriceForSystemAction(t *testing.T) {
	ctx := context.Background()
	cfg := newTestConfig()
	registry := protocol.NewRegistry()
	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(t, acc.Register(registry))
	rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
	require.NoError(t, rp.Register(registry))
	factoryCfg := factory.GenerateConfig(cfg.Chain, cfg.Genesis)
	sf, err := factory.NewStateDB(factoryCfg, db.NewMemKVStore(), factory.RegistryStateDBOption(registry))
	require.NoError(t, err)
	ap, err := actpool.NewActPool(cfg.Genesis, sf, cfg.ActPool)
	require.NoError(t, err)
	store, err := filedao.NewFileDAOInMemForTest()
	require.NoError(t, err)
	blkMemDao := blockdao.NewBlockDAOWithIndexersAndCache(store, []blockdao.BlockIndexer{sf}, 16)
	bc := blockchain.NewBlockchain(
		cfg.Chain,
		cfg.Genesis,
		blkMemDao,
		factory.NewMinter(sf, ap),
		blockchain.BlockValidatorOption(block.NewValidator(
			sf,
			protocol.NewGenericValidator(sf, accountutil.AccountState),
		)),
	)
	ep := execution.NewProtocol(blkMemDao.GetBlockHash, rewarding.DepositGas, func(u uint64) (time.Time, error) { return time.Time{}, nil })
	require.NoError(t, ep.Register(registry))
	rewardingProtocol := rewarding.NewProtocol(cfg.Genesis.Rewarding)
	require.NoError(t, rewardingProtocol.Register(registry))
	require.NoError(t, bc.Start(ctx))
	defer func() {
		require.NoError(t, bc.Stop(ctx))
	}()

	for i := 0; i < 30; i++ {
		blk, err := bc.MintNewBlock(testutil.TimestampNow())
		require.NoError(t, err)
		require.Equal(t, 1, len(blk.Actions))
		require.Equal(t, 1, len(blk.Receipts))
		var gasConsumed uint64
		for _, receipt := range blk.Receipts {
			gasConsumed += receipt.GasConsumed
		}
		require.True(t, gasConsumed <= cfg.Genesis.BlockGasLimit)
		require.NoError(t, bc.CommitBlock(blk))
	}
	height := bc.TipHeight()
	fmt.Printf("Open blockchain pass, height = %d\n", height)

	gs := NewGasStation(bc, blkMemDao, cfg.GasStation)
	require.NotNil(t, gs)

	gp, err := gs.SuggestGasPrice()
	fmt.Println(gp)
	require.NoError(t, err)
	// i from 10 to 29,gasprice for 20 to 39,60%*20+20=31
	require.Equal(t, gs.cfg.DefaultGas, gp)
}

func TestSuggestGasPrice_GasConsumed(t *testing.T) {
	cases := []testCase{
		{
			name: "gas consumed > maxGas/2",
			blocks: []testActionGas{
				{{uint64(unit.Qev) * 2, 100000000}},
				{{uint64(unit.Qev) * 2, 100000000}},
				{{uint64(unit.Qev) * 2, 100000000}},
				{{uint64(unit.Qev) * 2, 100000000}},
				{{uint64(unit.Qev) * 2, 100000000}},
				{{uint64(unit.Qev) * 2, 200000000}},
			},
			expectGasPrice: 2200000000000,
		},
		{
			name: "gas consumed < maxGas/5",
			blocks: []testActionGas{
				{{uint64(unit.Qev) * 2, 1000000}},
				{{uint64(unit.Qev) * 2, 1000000}},
				{{uint64(unit.Qev) * 2, 1000000}},
				{{uint64(unit.Qev) * 2, 1000000}},
				{{uint64(unit.Qev) * 2, 1000000}},
				{{uint64(unit.Qev) * 2, 2000000}},
			},
			expectGasPrice: 1800000000000,
		},
		{
			name: "gas consumed between maxGas/5 - maxGas/2",
			blocks: []testActionGas{
				{{uint64(unit.Qev) * 2, 10000000}},
				{{uint64(unit.Qev) * 2, 10000000}},
				{{uint64(unit.Qev) * 2, 10000000}},
				{{uint64(unit.Qev) * 2, 10000000}},
				{{uint64(unit.Qev) * 2, 10000000}},
				{{uint64(unit.Qev) * 2, 5000000}},
			},
			expectGasPrice: 2000000000000,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := require.New(t)
			blocks := prepareBlocks(r, c.blocks)
			ctrl := gomock.NewController(t)
			bc := mock_blockchain.NewMockBlockchain(ctrl)
			dao := mock_blockdao.NewMockBlockDAO(ctrl)
			gs := NewGasStation(bc, dao, DefaultConfig)
			bc.EXPECT().TipHeight().Return(uint64(len(blocks) - 1)).Times(1)
			bc.EXPECT().Genesis().Return(genesis.TestDefault()).Times(1)
			dao.EXPECT().GetBlockByHeight(gomock.Any()).DoAndReturn(
				func(height uint64) (*block.Block, error) {
					return blocks[height], nil
				},
			).AnyTimes()
			gp, err := gs.SuggestGasPrice()
			r.NoError(err)
			r.Equal(c.expectGasPrice, gp)
		})
	}
}

func prepareBlocks(r *require.Assertions, cases []testActionGas) map[uint64]*block.Block {
	blocks := map[uint64]*block.Block{}
	for i := range cases {
		actions := []*action.SealedEnvelope{}
		receipts := []*action.Receipt{}
		for _, gas := range cases[i] {
			seale, err := action.SignedTransfer(identityset.Address(1).String(), identityset.PrivateKey(1), 1, big.NewInt(0), []byte{}, 1000, big.NewInt(int64(gas.gasPrice)))
			r.NoError(err)
			actions = append(actions, seale)
			receipts = append(receipts, &action.Receipt{GasConsumed: gas.gasConsumed})
		}
		blocks[uint64(i)] = &block.Block{
			Body:     block.Body{Actions: actions},
			Receipts: receipts,
		}
	}
	return blocks
}
