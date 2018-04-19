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

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/server/itx"
	"github.com/stretchr/testify/assert"
)

const (
	// localRDPoSConfig is for local RDPoS testing
	localRDPoSConfig = "./config_local_rdpos.yaml"
)

// 4 delegates and 3 full nodes
func TestLocalRDPoS(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping TestLocalRDPoS in short mode.")
	}

	flag.Parse()

	cfg, err := config.LoadConfigWithPathWithoutValidation(localRDPoSConfig)
	assert.Nil(t, err)

	var svrs []itx.Server

	for i := 0; i < 3; i++ {
		cfg.Chain.ChainDBPath = "./test_fullnode_chain" + strconv.Itoa(i) + ".db"
		cfg.NodeType = config.FullNodeType
		cfg.Network.Addr = "127.0.0.1:5000" + strconv.Itoa(i)
		svr := itx.NewServer(*cfg)
		svr.Init()
		svr.Start()
		svrs = append(svrs, svr)
	}

	for i := 0; i < 4; i++ {
		cfg.Chain.ChainDBPath = "./test_delegate_chain" + strconv.Itoa(i) + ".db"
		cfg.NodeType = config.DelegateType
		cfg.Network.Addr = "127.0.0.1:4000" + strconv.Itoa(i)
		cfg.Consensus.Scheme = "RDPOS"
		svr := itx.NewServer(*cfg)
		svr.Init()
		svr.Start()
		svrs = append(svrs, svr)
	}

	for _, svr := range svrs {
		defer svr.Stop()
	}

	select {}
}
