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
)

const (
	// localRDPoSConfig is for local RDPoS testing
	localRDPoSConfig = "./config_local_rdpos.yaml"
)

// 4 delegates and 3 full nodes
func TestLocalRDPoS(t *testing.T) {
	assert := assert.New(t)
	flag.Parse()

	cfg, err := config.LoadConfigWithPathWithoutValidation(localRDPoSConfig)
	assert.Nil(err)

	var svrs []itx.Server

	for i := 0; i < 3; i++ {
		cfg.Chain.ChainDBPath = "./test_fullnode_chain" + strconv.Itoa(i) + ".db"
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
		cfg.Chain.ChainDBPath = "./test_delegate_chain" + strconv.Itoa(i) + ".db"
		cfg.NodeType = config.DelegateType
		cfg.Network.Addr = "127.0.0.1:4000" + strconv.Itoa(i)
		cfg.Consensus.Scheme = "RDPOS"
		svr := itx.NewServer(*cfg)
		err = svr.Init()
		assert.Nil(err)
		err = svr.Start()
		assert.Nil(err)
		svrs = append(svrs, svr)
		defer svr.Stop()
	}

	time.Sleep(time.Second * 6)

	var hash1, hash2, hash3, hash4 common.Hash32B

	for i, svr := range svrs {
		bc := svr.Bc()
		assert.NotNil(bc)

		if i == 0 {
			blk, err := bc.GetBlockByHeight(1)
			assert.Nil(err)
			hash1 = blk.HashBlock()
			blk, err = bc.GetBlockByHeight(2)
			assert.Nil(err)
			hash2 = blk.HashBlock()
			blk, err = bc.GetBlockByHeight(3)
			assert.Nil(err)
			hash3 = blk.HashBlock()
			blk, err = bc.GetBlockByHeight(4)
			assert.Nil(err)
			hash4 = blk.HashBlock()
			continue
		}

		// verify 4 received blocks
		blk, err := bc.GetBlockByHeight(1)
		assert.Nil(err)
		assert.Equal(hash1, blk.HashBlock())
		blk, err = bc.GetBlockByHeight(2)
		assert.Nil(err)
		assert.Equal(hash2, blk.HashBlock())
		blk, err = bc.GetBlockByHeight(3)
		assert.Nil(err)
		assert.Equal(hash3, blk.HashBlock())
		blk, err = bc.GetBlockByHeight(4)
		assert.Nil(err)
		assert.Equal(hash4, blk.HashBlock())
	}
}
