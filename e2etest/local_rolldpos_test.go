// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package e2etest

import (
	"context"
	"encoding/hex"
	"flag"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/server/itx"
)

func TestLocalRollDPoS(t *testing.T) {
	t.Skip()
	// TODO: figure out why there's race condition with the following two tests
	/*
		t.Run("FixedProposer-NeverStarNewEpoch", func(t *testing.T) {
			testLocalRollDPoS("FixedProposer", "NeverStartNewEpoch", 4, t)
		})
		t.Run("PseudoRotatedProposer-NeverStarNewEpoch", func(t *testing.T) {
			testLocalRollDPoS("PseudoRotatedProposer", "NeverStartNewEpoch", 4, t)
		})
	*/
	t.Run("FixedProposer-PseudoStarNewEpoch-NoInterval", func(t *testing.T) {
		testLocalRollDPoS("FixedProposer", "PseudoStarNewEpoch", 8, t, 0)
	})
	t.Run("PseudoRotatedProposer-PseudoStarNewEpoch-Interval", func(t *testing.T) {
		testLocalRollDPoS(
			"PseudoRotatedProposer", "PseudoStarNewEpoch", 8, t, 100*time.Millisecond)
	})
	t.Run("PseudoRotatedProposer-PseudoStarNewEpoch-NoInterval", func(t *testing.T) {
		testLocalRollDPoS("PseudoRotatedProposer", "PseudoStarNewEpoch", 8, t, 0)
	})
	t.Run("PseudoRotatedProposer-PseudoStarNewEpoch-Interval", func(t *testing.T) {
		testLocalRollDPoS(
			"PseudoRotatedProposer", "PseudoStarNewEpoch", 8, t, 100*time.Millisecond)
	})
}

// 4 delegates and 3 full nodes
func testLocalRollDPoS(prCb string, epochCb string, numBlocks uint64, t *testing.T, interval time.Duration) {
	require := require.New(t)
	ctx := context.Background()
	flag.Parse()
	cfg, err := newConfig(prCb, epochCb, interval)
	require.Nil(err)
	var svrs []*itx.Server
	for i := 0; i < 3; i++ {
		cfg.NodeType = config.FullNodeType
		cfg.Network.Addr = "127.0.0.1:5000" + strconv.Itoa(i)
		svr := itx.NewInMemTestServer(cfg)
		require.Nil(svr.Start(ctx))
		svrs = append(svrs, svr)
		defer require.Nil(svr.Stop(ctx))
	}

	for i := 0; i < 4; i++ {
		cfg.NodeType = config.DelegateType
		cfg.Network.Addr = "127.0.0.1:4000" + strconv.Itoa(i)
		cfg.Consensus.Scheme = config.RollDPoSScheme
		svr := itx.NewInMemTestServer(cfg)
		require.Nil(svr.Start(ctx))
		svrs = append(svrs, svr)
		defer require.Nil(svr.Stop(ctx))
	}

	satisfy := func() bool {
		for _, svr := range svrs {
			bc := svr.Bc()
			if bc == nil {
				return false
			}
			height, err := bc.TipHeight()
			if err != nil {
				return false
			}
			if height < numBlocks {
				return false
			}
		}
		return true
	}
	waitUntil(t, satisfy, 100*time.Millisecond, 10*time.Second, "at least one node misses enough block")

	hashes := make([]hash.Hash32B, numBlocks+1)
	for i, svr := range svrs {
		bc := svr.Bc()
		require.NotNil(bc)

		if i == 0 {
			for j := uint64(1); j <= numBlocks; j++ {
				blk, err := bc.GetBlockByHeight(j)
				require.Nil(err, "%s gets non-nil error", svr.P2p().Self().String())
				require.NotNil(blk, "%s gets nil block", svr.P2p().Self().String())
				hashes[j] = blk.HashBlock()
			}
		}

		// verify received blocks
		for j := uint64(1); j <= numBlocks; j++ {
			blk, err := bc.GetBlockByHeight(j)
			require.Nil(err, "%s gets non-nil error", svr.P2p().Self().String())
			require.NotNil(blk, "%s gets nil block", svr.P2p().Self().String())
			require.Equal(hashes[j], blk.HashBlock())
		}
	}
}

func waitUntil(t *testing.T, satisfy func() bool, interval time.Duration, timeout time.Duration, msg string) {
	ready := make(chan bool)
	go func() {
		for range time.NewTicker(interval).C {
			if satisfy() {
				ready <- true
			}
		}
	}()
	select {
	case <-ready:
	case <-time.After(timeout):
		require.Fail(t, msg)
	}
}

func newConfig(prCb string, epochCb string, interval time.Duration) (*config.Config, error) {
	addrs := []string{
		"127.0.0.1:40000",
		"127.0.0.1:40001",
		"127.0.0.1:40002",
		"127.0.0.1:40003",
	}
	cfg := config.Default
	cfg.Network.BootstrapNodes = addrs
	cfg.Delegate.Addrs = addrs
	// disable account-based testing
	cfg.Chain.TrieDBPath = ""
	cfg.Consensus.RollDPoS = config.RollDPoS{
		DelegateInterval:  100 * time.Millisecond,
		ProposerInterval:  interval,
		ProposerCB:        prCb,
		EpochCB:           epochCb,
		UnmatchedEventTTL: 100 * time.Millisecond,
		RoundStartTTL:     10 * time.Second,
		AcceptProposeTTL:  100 * time.Millisecond,
		AcceptPrevoteTTL:  100 * time.Millisecond,
		AcceptVoteTTL:     100 * time.Millisecond,
		Delay:             2 * time.Second,
		NumSubEpochs:      1,
		EventChanSize:     10000,
	}
	addr, err := iotxaddress.NewAddress(true, iotxaddress.ChainID)
	if err != nil {
		return nil, err
	}
	cfg.Chain.ProducerPubKey = hex.EncodeToString(addr.PublicKey)
	cfg.Chain.ProducerPrivKey = hex.EncodeToString(addr.PrivateKey)
	return &cfg, nil
}
