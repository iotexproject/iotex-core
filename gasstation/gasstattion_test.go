// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package gasstation

import (
	"context"
	"fmt"
	"math/big"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"

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
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockchain"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockdao"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestNewGasStation(t *testing.T) {
	require := require.New(t)
	require.NotNil(NewGasStation(nil, nil, nil, config.Default.API.GasStation))
}
func TestSuggestGasPriceForUserAction(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default
	cfg.Genesis.BlockGasLimit = uint64(100000)
	cfg.Genesis.EnableGravityChainVoting = false
	registry := protocol.NewRegistry()
	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(t, acc.Register(registry))
	rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
	require.NoError(t, rp.Register(registry))
	sf, err := factory.NewFactory(cfg, factory.InMemTrieOption(), factory.RegistryOption(registry))
	require.NoError(t, err)
	ap, err := actpool.NewActPool(sf, cfg.ActPool)
	require.NoError(t, err)
	blkMemDao := blockdao.NewBlockDAOInMemForTest([]blockdao.BlockIndexer{sf})
	bc := blockchain.NewBlockchain(
		cfg,
		blkMemDao,
		factory.NewMinter(sf, ap),
		blockchain.BlockValidatorOption(block.NewValidator(
			sf,
			protocol.NewGenericValidator(sf, accountutil.AccountState),
		)),
	)
	ep := execution.NewProtocol(blkMemDao.GetBlockHash, rewarding.DepositGas)
	require.NoError(t, ep.Register(registry))
	rewardingProtocol := rewarding.NewProtocol(cfg.Genesis.Rewarding)
	require.NoError(t, rewardingProtocol.Register(registry))
	require.NoError(t, bc.Start(ctx))
	defer func() {
		require.NoError(t, bc.Stop(ctx))
	}()

	for i := 0; i < 30; i++ {
		tsf, err := action.NewTransfer(
			uint64(i)+1,
			big.NewInt(100),
			identityset.Address(27).String(),
			[]byte{}, uint64(100000),
			big.NewInt(1).Mul(big.NewInt(int64(i)+10), big.NewInt(unit.Qev)),
		)
		require.NoError(t, err)

		bd := &action.EnvelopeBuilder{}
		elp1 := bd.SetAction(tsf).
			SetNonce(uint64(i) + 1).
			SetGasLimit(100000).
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

	gs := NewGasStation(bc, sf.SimulateExecution, blkMemDao, cfg.API.GasStation)
	require.NotNil(t, gs)

	gp, err := gs.SuggestGasPrice()
	require.NoError(t, err)
	// i from 10 to 29,gasprice for 20 to 39,60%*20+20=31
	require.Equal(t, big.NewInt(1).Mul(big.NewInt(int64(31)), big.NewInt(unit.Qev)).Uint64(), gp)
}

func TestSuggestGasPriceForSystemAction(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default
	cfg.Genesis.BlockGasLimit = uint64(100000)
	cfg.Genesis.EnableGravityChainVoting = false
	registry := protocol.NewRegistry()
	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(t, acc.Register(registry))
	rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
	require.NoError(t, rp.Register(registry))
	sf, err := factory.NewFactory(cfg, factory.InMemTrieOption(), factory.RegistryOption(registry))
	require.NoError(t, err)
	ap, err := actpool.NewActPool(sf, cfg.ActPool)
	require.NoError(t, err)
	blkMemDao := blockdao.NewBlockDAOInMemForTest([]blockdao.BlockIndexer{sf})
	bc := blockchain.NewBlockchain(
		cfg,
		blkMemDao,
		factory.NewMinter(sf, ap),
		blockchain.BlockValidatorOption(block.NewValidator(
			sf,
			protocol.NewGenericValidator(sf, accountutil.AccountState),
		)),
	)
	ep := execution.NewProtocol(blkMemDao.GetBlockHash, rewarding.DepositGas)
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

	gs := NewGasStation(bc, sf.SimulateExecution, blkMemDao, cfg.API.GasStation)
	require.NotNil(t, gs)

	gp, err := gs.SuggestGasPrice()
	fmt.Println(gp)
	require.NoError(t, err)
	// i from 10 to 29,gasprice for 20 to 39,60%*20+20=31
	require.Equal(t, gs.cfg.DefaultGas, gp)
}

func TestEstimateGasForAction(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	cfg := config.Default
	sf, err := factory.NewFactory(cfg, factory.InMemTrieOption())
	require.NoError(err)
	dao := mock_blockdao.NewMockBlockDAO(ctrl)
	mBc := mock_blockchain.NewMockBlockchain(ctrl)
	gs := NewGasStation(mBc, sf.SimulateExecution, dao, config.Default.API.GasStation)
	require.NotNil(gs)

	act := getAction()
	require.NotNil(act)
	ret, err := gs.EstimateGasForAction(act)
	require.NoError(err)
	// base intrinsic gas 10000
	require.Equal(uint64(10000), ret)

	// test for payload
	act = getActionWithPayload()
	require.NotNil(act)
	ret, err = gs.EstimateGasForAction(act)
	require.NoError(err)
	// base intrinsic gas 10000,plus data size*ExecutionDataGas
	require.Equal(uint64(10000)+10*action.ExecutionDataGas, ret)

	ret, err = gs.EstimateGasForAction(nil)
	require.ErrorIs(err, action.ErrNilProto)
	require.Equal(ret, uint64(0))
}

func TestIsSystemAction(t *testing.T) {
	require := require.New(t)
	gs := NewGasStation(nil, nil, nil, config.Default.API.GasStation)
	require.NotNil(gs)
	builder := action.EnvelopeBuilder{}
	cf := action.ClaimFromRewardingFundBuilder{}
	actClaimFromRewarding := cf.Build()
	act := builder.SetAction(&actClaimFromRewarding).Build()
	sel, err := action.Sign(act, identityset.PrivateKey(1))
	require.NoError(err)
	require.False(gs.isSystemAction(sel))

	gb := action.GrantRewardBuilder{}
	actGrantReward := gb.Build()
	act = builder.SetAction(&actGrantReward).Build()
	sel, err = action.Sign(act, identityset.PrivateKey(1))
	require.NoError(err)
	require.True(gs.isSystemAction(sel))

	actPollResult := action.NewPutPollResult(1, 1, nil)
	act = builder.SetAction(actPollResult).Build()
	sel, err = action.Sign(act, identityset.PrivateKey(1))
	require.NoError(err)
	require.True(gs.isSystemAction(sel))
}

func getAction() (act *iotextypes.Action) {
	pubKey1 := identityset.PrivateKey(28).PublicKey()
	addr2 := identityset.Address(29).String()

	act = &iotextypes.Action{
		Core: &iotextypes.ActionCore{
			Action: &iotextypes.ActionCore_Transfer{
				Transfer: &iotextypes.Transfer{Recipient: addr2},
			},
			Version: version.ProtocolVersion,
			Nonce:   101,
		},
		SenderPubKey: pubKey1.Bytes(),
		Signature:    action.ValidSig,
	}
	return
}

func getActionWithPayload() (act *iotextypes.Action) {
	pubKey1 := identityset.PrivateKey(28).PublicKey()
	addr2 := identityset.Address(29).String()

	act = &iotextypes.Action{
		Core: &iotextypes.ActionCore{
			Action: &iotextypes.ActionCore_Transfer{
				Transfer: &iotextypes.Transfer{Recipient: addr2, Payload: []byte("1234567890")},
			},
			Version: version.ProtocolVersion,
			Nonce:   101,
		},
		SenderPubKey: pubKey1.Bytes(),
		Signature:    action.ValidSig,
	}
	return
}
