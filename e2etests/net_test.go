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

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/server/itx"
)

const (
	// localFullnodeConfig is the testnet config path
	localFullnodeConfig = "./config_local_fullnode.yaml"
)

func TestNetSync(t *testing.T) {
	assert := assert.New(t)
	os.Remove(testDBPath)
	defer os.Remove(testDBPath)

	config, err := config.LoadConfigWithPathWithoutValidation(localFullnodeConfig)
	assert.Nil(err)
	if testing.Short() {
		t.Skip("Skipping the overlay test in short mode.")
	}

	// create node
	svr := itx.NewServer(*config)
	assert.NotNil(svr)
	svr.Init()
	svr.Start()
	defer svr.Stop()

	select {}
}
