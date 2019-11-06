// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"context"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	"github.com/iotexproject/iotex-core/action/protocol/poll"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestUpdateRound(t *testing.T) {
	require := require.New(t)
	bc, roll := makeChain(t)
	rc := &roundCalculator{bc, true, roll, bc.CandidatesByHeight, 0}
	ra, err := rc.NewRound(1, time.Second, time.Unix(1562382392, 0), nil)
	require.NoError(err)

	// height < round.Height()
	_, err = rc.UpdateRound(ra, 0, time.Second, time.Unix(1562382492, 0), time.Second)
	require.Error(err)

	// height == round.Height() and now.Before(round.StartTime())
	_, err = rc.UpdateRound(ra, 1, time.Second, time.Unix(1562382092, 0), time.Second)
	require.NoError(err)

	// height >= round.NextEpochStartHeight() Delegates error
	_, err = rc.UpdateRound(ra, 500, time.Second, time.Unix(1562382092, 0), time.Second)
	require.Error(err)

	// (31+120)%24
	ra, err = rc.UpdateRound(ra, 31, time.Second, time.Unix(1562382522, 0), time.Second)
	require.NoError(err)
	require.Equal(identityset.Address(7).String(), ra.proposer)
}

func TestNewRound(t *testing.T) {
	require := require.New(t)
	bc, roll := makeChain(t)
	rc := &roundCalculator{bc, true, roll, bc.CandidatesByHeight, 0}
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

	ra, err := rc.NewRound(1, time.Second, time.Unix(1562382392, 0), nil)
	require.NoError(err)
	require.Equal(uint32(19), ra.roundNum)
	require.Equal(uint64(1), ra.height)
	// sorted by address hash
	require.Equal(identityset.Address(16).String(), ra.proposer)

	rc.timeBasedRotation = true
	ra, err = rc.NewRound(1, time.Second, time.Unix(1562382392, 0), nil)
	require.NoError(err)
	require.Equal(uint32(19), ra.roundNum)
	require.Equal(uint64(1), ra.height)
	require.Equal(identityset.Address(5).String(), ra.proposer)
}

func TestDelegates(t *testing.T) {
	require := require.New(t)
	bc, roll := makeChain(t)
	rc := &roundCalculator{bc, true, roll, bc.CandidatesByHeight, 0}
	_, err := rc.Delegates(361)
	require.Error(err)

	dels, err := rc.Delegates(4)
	require.NoError(err)
	require.Equal(roll.NumDelegates(), uint64(len(dels)))

	require.False(rc.IsDelegate(identityset.Address(25).String(), 2))
	require.True(rc.IsDelegate(identityset.Address(5).String(), 2))
}

func TestRoundInfo(t *testing.T) {
	require := require.New(t)
	bc, roll := makeChain(t)
	rc := &roundCalculator{bc, true, roll, bc.CandidatesByHeight, 0}
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

func makeChain(t *testing.T) (blockchain.Blockchain, *rolldpos.Protocol) {
	require := require.New(t)
	cfg := config.Default

	testTrieFile, _ := ioutil.TempFile(os.TempDir(), "trie")
	testTriePath := testTrieFile.Name()
	testDBFile, _ := ioutil.TempFile(os.TempDir(), "db")
	testDBPath := testDBFile.Name()
	testIndexFile, _ := ioutil.TempFile(os.TempDir(), "index")
	testIndexPath := testIndexFile.Name()
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Chain.IndexDBPath = testIndexPath

	cfg.Consensus.Scheme = config.RollDPoSScheme
	cfg.Network.Port = testutil.RandomPort()
	cfg.API.Port = testutil.RandomPort()
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

	registry := protocol.Registry{}
	chain := blockchain.NewBlockchain(
		cfg,
		nil,
		blockchain.DefaultStateFactoryOption(),
		blockchain.BoltDBDaoOption(),
		blockchain.RegistryOption(&registry),
	)
	rolldposProtocol := rolldpos.NewProtocol(
		cfg.Genesis.NumCandidateDelegates,
		cfg.Genesis.NumDelegates,
		cfg.Genesis.NumSubEpochs,
	)

	require.NoError(registry.Register(rolldpos.ProtocolID, rolldposProtocol))
	rewardingProtocol := rewarding.NewProtocol(chain, rolldposProtocol)
	registry.Register(rewarding.ProtocolID, rewardingProtocol)
	acc := account.NewProtocol(config.NewHeightUpgrade(cfg))
	registry.Register(account.ProtocolID, acc)
	require.NoError(registry.Register(poll.ProtocolID, poll.NewLifeLongDelegatesProtocol(cfg.Genesis.Delegates)))
	chain.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(chain))
	chain.Validator().AddActionValidators(acc, rewardingProtocol)
	chain.GetFactory().AddActionHandlers(acc, rewardingProtocol)
	ctx := context.Background()
	require.NoError(chain.Start(ctx))
	for i := 0; i < 50; i++ {
		blk, err := chain.MintNewBlock(
			nil,
			time.Unix(cfg.Genesis.Timestamp+int64(i), 0),
		)
		require.NoError(blk.Finalize(nil, time.Unix(cfg.Genesis.Timestamp+int64(i), 0)))
		require.NoError(err)
		require.NoError(chain.CommitBlock(blk))
	}
	require.Equal(uint64(50), chain.TipHeight())
	require.NoError(err)
	return chain, rolldposProtocol
}
