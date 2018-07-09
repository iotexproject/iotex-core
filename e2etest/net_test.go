// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package e2etest

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/server/itx"
	"github.com/iotexproject/iotex-core/test/util"
)

const (
	// localFullnodeConfig is the testnet config path
	localFullnodeConfig = "./config_local_fullnode.yaml"
)

func TestNetSync(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping TestNetSync in short mode.")
	}

	assert := assert.New(t)
	util.CleanupPath(t, testDBPath)
	defer util.CleanupPath(t, testDBPath)

	config.Path = localFullnodeConfig
	cfg, err := config.New()
	util.CleanupPath(t, testTriePath)
	defer util.CleanupPath(t, testTriePath)
	util.CleanupPath(t, testDBPath)
	defer util.CleanupPath(t, testDBPath)

	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	assert.Nil(err)
	if testing.Short() {
		t.Skip("Skipping the overlay test in short mode.")
	}

	// create node
	ctx := context.Background()
	svr := itx.NewServer(*cfg)
	assert.NotNil(svr)
	assert.Nil(err)
	err = svr.Start(ctx)
	assert.Nil(err)
	defer svr.Stop(ctx)

	select {}
}
