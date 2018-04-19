// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package e2etests

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blocksync"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/delegate"
	"github.com/iotexproject/iotex-core/dispatcher"
	"github.com/iotexproject/iotex-core/network"
	ta "github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/txpool"
)

const (
	// localFullnodeConfig is the testnet config path
	localFullnodeConfig = "./config_local_fullnode.yaml"
)

func TestNetSync(t *testing.T) {
	defer os.Remove(testDBPath)
	assert := assert.New(t)

	config, err := config.LoadConfigWithPathWithoutValidation(localFullnodeConfig)
	assert.Nil(err)
	if testing.Short() {
		t.Skip("Skipping the overlay test in short mode.")
	}

	// create Blockchain
	// create Blockchain
	bc := blockchain.CreateBlockchain(ta.Addrinfo["miner"].Address, config)
	assert.NotNil(bc)
	t.Log("Create blockchain pass")
	defer bc.Close()

	// create TxPool
	tp := txpool.New(bc)
	assert.NotNil(tp)

	// create client
	p1 := network.NewOverlay(&config.Network)
	assert.NotNil(p1)
	p1.Init()
	p1.Start()
	defer p1.Stop()

	pool := delegate.NewConfigBasedPool(&config.Delegate)
	pool.Init()
	pool.Start()
	defer pool.Stop()

	// create block sync
	bs := blocksync.NewBlockSyncer(config, bc, tp, p1, pool)
	assert.NotNil(bs)

	// create dispatcher
	dp := dispatcher.NewDispatcher(config, bc, nil, bs, pool)
	assert.NotNil(dp)
	p1.AttachDispatcher(dp)
	dp.Start()
	defer dp.Stop()

	select {}
}
