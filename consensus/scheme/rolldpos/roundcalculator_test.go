// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/poll"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestUpdateRound(t *testing.T) {
	require := require.New(t)
	rc := makeRoundCalculator(t)
	ra, err := rc.NewRound(51, time.Second, time.Unix(1562382522, 0), nil)
	require.NoError(err)

	// height < round.Height()
	_, err = rc.UpdateRound(ra, 50, time.Second, time.Unix(1562382492, 0), time.Second)
	require.Error(err)

	// height == round.Height() and now.Before(round.StartTime())
	_, err = rc.UpdateRound(ra, 51, time.Second, time.Unix(1562382522, 0), time.Second)
	require.NoError(err)

	// height >= round.NextEpochStartHeight() Delegates error
	_, err = rc.UpdateRound(ra, 500, time.Second, time.Unix(1562382092, 0), time.Second)
	require.Error(err)

	// (51+100)%24
	ra, err = rc.UpdateRound(ra, 51, time.Second, time.Unix(1562382522, 0), time.Second)
	require.NoError(err)
	require.Equal(identityset.Address(10).String(), ra.proposer)
}

func TestNewRound(t *testing.T) {
	require := require.New(t)
	rc := makeRoundCalculator(t)
	_, err := rc.calculateProposer(5, 1, []string{"1", "2", "3", "4", "5"})
	require.Error(err)
	var validDelegates [24]string
	for i := 0; i < 24; i++ {
		validDelegates[i] = identityset.Address(i).String()
	}
	proposer, err := rc.calculateProposer(5, 1, validDelegates[:])
	require.NoError(err)
	require.Equal(validDelegates[6], proposer)

	rc.timeBasedRotation = false
	proposer, err = rc.calculateProposer(50, 1, validDelegates[:])
	require.NoError(err)
	require.Equal(validDelegates[2], proposer)

	ra, err := rc.NewRound(51, time.Second, time.Unix(1562382592, 0), nil)
	require.NoError(err)
	require.Equal(uint32(170), ra.roundNum)
	require.Equal(uint64(51), ra.height)
	// sorted by address hash
	require.Equal(identityset.Address(7).String(), ra.proposer)

	rc.timeBasedRotation = true
	ra, err = rc.NewRound(51, time.Second, time.Unix(1562382592, 0), nil)
	require.NoError(err)
	require.Equal(uint32(170), ra.roundNum)
	require.Equal(uint64(51), ra.height)
	require.Equal(identityset.Address(12).String(), ra.proposer)
}

func TestDelegates(t *testing.T) {
	require := require.New(t)
	rc := makeRoundCalculator(t)

	dels, err := rc.Delegates(51)
	require.NoError(err)
	require.Equal(rc.rp.NumDelegates(), uint64(len(dels)))

	require.False(rc.IsDelegate(identityset.Address(25).String(), 51))
	require.True(rc.IsDelegate(identityset.Address(0).String(), 51))
}

func TestRoundInfo(t *testing.T) {
	require := require.New(t)
	rc := makeRoundCalculator(t)
	require.NotNil(rc)

	// error for lastBlockTime.Before(now)
	_, _, err := rc.RoundInfo(1, time.Second, time.Unix(1562382300, 0))
	require.Error(err)

	// height is 1 with withToleration false
	roundNum, roundStartTime, err := rc.RoundInfo(1, time.Second, time.Unix(1562382392, 0))
	require.NoError(err)
	require.Equal(uint32(19), roundNum)
	require.True(roundStartTime.Equal(time.Unix(1562382392, 0)))

	// height is 1 with withToleration true and duration%c.blockInterval < c.toleratedOvertime
	roundNum, roundStartTime, err = rc.roundInfo(1, time.Second, time.Unix(1562382392, 500000), 501*time.Microsecond)
	require.NoError(err)
	require.Equal(uint32(19), roundNum)
	require.True(roundStartTime.Equal(time.Unix(1562382392, 0)))

	// height is 1 with withToleration true and duration%c.blockInterval >= c.toleratedOvertime
	roundNum, roundStartTime, err = rc.roundInfo(1, time.Second, time.Unix(1562382392, 500000), 500*time.Microsecond)
	require.NoError(err)
	require.Equal(uint32(20), roundNum)
	require.True(roundStartTime.After(time.Unix(1562382392, 0)))

	// height is 4 with withToleration true and duration%c.blockInterval >= c.toleratedOvertime
	roundNum, roundStartTime, err = rc.roundInfo(4, time.Second, time.Unix(1562382392, 500000), 500*time.Microsecond)
	require.NoError(err)
	require.Equal(uint32(18), roundNum)
	require.True(roundStartTime.Equal(time.Unix(1562382393, 0)))
}

func makeChain(t *testing.T) (blockchain.Blockchain, factory.Factory, actpool.ActPool, *rolldpos.Protocol, poll.Protocol) {
	require := require.New(t)
	cfg := config.Default

	testTriePath, err := testutil.PathOfTempFile("trie")
	require.NoError(err)
	testDBPath, err := testutil.PathOfTempFile("db")
	require.NoError(err)
	testIndexPath, err := testutil.PathOfTempFile("index")
	require.NoError(err)
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Chain.IndexDBPath = testIndexPath
	defer func() {
		testutil.CleanupPath(testTriePath)
		testutil.CleanupPath(testDBPath)
		testutil.CleanupPath(testIndexPath)
	}()

	cfg.Consensus.Scheme = config.RollDPoSScheme
	cfg.Network.Port = testutil.RandomPort()
	cfg.API.GRPCPort = testutil.RandomPort()
	cfg.API.HTTPPort = testutil.RandomPort()
	cfg.Genesis.Timestamp = 1562382372
	sk, err := crypto.GenerateKey()
	cfg.Chain.ProducerPrivKey = sk.HexString()
	require.NoError(err)

	for i := 0; i < identityset.Size(); i++ {
		addr := identityset.Address(i).String()
		value := unit.ConvertIotxToRau(100000000).String()
		cfg.Genesis.InitBalanceMap[addr] = value
		if uint64(i) < cfg.Genesis.NumDelegates {
			d := genesis.Delegate{
				OperatorAddrStr: addr,
				RewardAddrStr:   addr,
				VotesStr:        value,
			}
			cfg.Genesis.Delegates = append(cfg.Genesis.Delegates, d)
		}
	}
	registry := protocol.NewRegistry()
	factoryCfg := factory.GenerateConfig(cfg.Chain, cfg.Genesis)
	db1, err := db.CreateDAOForStateDB(cfg.DB, cfg.Chain.TrieDBPath)
	require.NoError(err)
	sf, err := factory.NewFactory(factoryCfg, db1, factory.RegistryOption(registry))
	require.NoError(err)
	ap, err := actpool.NewActPool(cfg.Genesis, sf, cfg.ActPool)
	require.NoError(err)
	dbcfg := cfg.DB
	dbcfg.DbPath = cfg.Chain.ChainDBPath
	deser := block.NewDeserializer(cfg.Chain.EVMNetworkID)
	dao := blockdao.NewBlockDAO([]blockdao.BlockIndexer{sf}, dbcfg, deser)
	chain := blockchain.NewBlockchain(
		cfg.Chain,
		cfg.Genesis,
		dao,
		factory.NewMinter(sf, ap),
		blockchain.BlockValidatorOption(block.NewValidator(
			sf,
			protocol.NewGenericValidator(sf, accountutil.AccountState),
		)),
	)
	rolldposProtocol := rolldpos.NewProtocol(
		cfg.Genesis.NumCandidateDelegates,
		cfg.Genesis.NumDelegates,
		cfg.Genesis.NumSubEpochs,
	)

	require.NoError(rolldposProtocol.Register(registry))
	rewardingProtocol := rewarding.NewProtocol(cfg.Genesis.Rewarding)
	require.NoError(rewardingProtocol.Register(registry))
	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(acc.Register(registry))
	pp := poll.NewLifeLongDelegatesProtocol(cfg.Genesis.Delegates)
	require.NoError(pp.Register(registry))
	ctx := context.Background()
	require.NoError(chain.Start(ctx))
	for i := 0; i < 50; i++ {
		blk, err := chain.MintNewBlock(time.Unix(cfg.Genesis.Timestamp+int64(i), 0))
		require.NoError(blk.Finalize(nil, time.Unix(cfg.Genesis.Timestamp+int64(i), 0)))
		require.NoError(err)
		require.NoError(chain.CommitBlock(blk))
	}
	require.Equal(uint64(50), chain.TipHeight())
	require.NoError(err)
	return chain, sf, ap, rolldposProtocol, pp
}

func makeRoundCalculator(t *testing.T) *roundCalculator {
	bc, sf, _, rp, pp := makeChain(t)
	return &roundCalculator{
		NewChainManager(bc),
		true,
		rp,
		func(epochNum uint64) ([]string, error) {
			re := protocol.NewRegistry()
			if err := rp.Register(re); err != nil {
				return nil, err
			}
			tipHeight := bc.TipHeight()
			ctx := genesis.WithGenesisContext(
				protocol.WithBlockchainCtx(
					protocol.WithRegistry(context.Background(), re),
					protocol.BlockchainCtx{
						Tip: protocol.TipInfo{
							Height: tipHeight,
						},
					},
				),
				genesis.Default,
			)
			tipEpochNum := rp.GetEpochNum(tipHeight)
			var candidatesList state.CandidateList
			var addrs []string
			var err error
			switch epochNum {
			case tipEpochNum:
				candidatesList, err = pp.Delegates(ctx, sf)
			case tipEpochNum + 1:
				candidatesList, err = pp.NextDelegates(ctx, sf)
			default:
				err = errors.Errorf("invalid epoch number %d compared to tip epoch number %d", epochNum, tipEpochNum)
			}
			if err != nil {
				return nil, err
			}
			for _, cand := range candidatesList {
				addrs = append(addrs, cand.Address)
			}
			return addrs, nil
		},
		0,
	}
}
