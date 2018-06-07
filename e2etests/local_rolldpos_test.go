// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package e2etests

import (
	"flag"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/server/itx"
	"github.com/iotexproject/iotex-core/test/util"
)

const (
	// localRollDPoSConfig is for local RollDPoS testing
	localRollDPoSConfig = "./config_local_rolldpos.yaml"
)

func TestLocalRollDPoS(t *testing.T) {
	// TODO: figure out why there's race condition with the following two tests
	/*
		t.Run("FixedProposer-NeverStarNewEpoch", func(t *testing.T) {
			testLocalRollDPoS("FixedProposer", "NeverStartNewEpoch", 4, t)
		})
		t.Run("PseudoRotatedProposer-NeverStarNewEpoch", func(t *testing.T) {
			testLocalRollDPoS("PseudoRotatedProposer", "NeverStartNewEpoch", 4, t)
		})
	*/
	t.Run("FixedProposer-PseudoStarNewEpoch", func(t *testing.T) {
		testLocalRollDPoS("FixedProposer", "PseudoStarNewEpoch", 8, t)
	})
	t.Run("PseudoRotatedProposer-PseudoStarNewEpoch", func(t *testing.T) {
		testLocalRollDPoS("PseudoRotatedProposer", "PseudoStarNewEpoch", 8, t)
	})
}

// 4 delegates and 3 full nodes
func testLocalRollDPoS(prCb string, epochCb string, numBlocks uint64, t *testing.T) {
	assert := assert.New(t)
	flag.Parse()

	cfg, err := config.LoadConfigWithPathWithoutValidation(localRollDPoSConfig)
	// disable account-based testing
	cfg.Chain.TrieDBPath = ""
	cfg.Chain.InMemTest = true
	cfg.Consensus.RollDPoS.ProposerCB = prCb
	cfg.Consensus.RollDPoS.EpochCB = epochCb
	assert.Nil(err)

	var svrs []*itx.Server

	for i := 0; i < 3; i++ {
		cfg.NodeType = config.FullNodeType
		cfg.Network.Addr = "127.0.0.1:5000" + strconv.Itoa(i)
		svr := itx.NewServer(*cfg)
		err = svr.Init()
		assert.Nil(err)
		err = svr.Start()
		assert.Nil(err)
		svrs = append(svrs, svr)
		defer svr.Stop()
	}

	for i := 0; i < 4; i++ {
		cfg.NodeType = config.DelegateType
		cfg.Network.Addr = "127.0.0.1:4000" + strconv.Itoa(i)
		cfg.Consensus.Scheme = config.RollDPoSScheme
		svr := itx.NewServer(*cfg)
		err = svr.Init()
		assert.Nil(err)
		err = svr.Start()
		assert.Nil(err)
		svrs = append(svrs, svr)
		defer svr.Stop()
	}

	err = util.WaitUntil(time.Millisecond*200, time.Second*10, func() (bool, error) {
		for _, svr := range svrs {
			bc := svr.Bc()
			if bc == nil {
				return false, nil
			}
			height, err := bc.TipHeight()
			if err != nil {
				return false, err
			}
			if height < numBlocks {
				return false, nil
			}
		}
		return true, nil
	})
	assert.Nil(err)

	hashes := make([]common.Hash32B, numBlocks+1)
	for i, svr := range svrs {
		bc := svr.Bc()
		assert.NotNil(bc)

		if i == 0 {
			for j := uint64(1); j <= numBlocks; j++ {
				blk, err := bc.GetBlockByHeight(j)
				assert.Nil(err, "%s gets non-nil error", svr.P2p().PRC.String())
				assert.NotNil(blk, "%s gets nil block", svr.P2p().PRC.String())
				hashes[j] = blk.HashBlock()
			}
		}

		// verify received blocks
		for j := uint64(1); j <= numBlocks; j++ {
			blk, err := bc.GetBlockByHeight(j)
			assert.Nil(err, "%s gets non-nil error", svr.P2p().PRC.String())
			assert.NotNil(blk, "%s gets nil block", svr.P2p().PRC.String())
			assert.Equal(hashes[j], blk.HashBlock())
		}
	}
}
