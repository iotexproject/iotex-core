// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenesis(t *testing.T) {
	genesis, err := LoadGenesisWithPath(DefaultGenesisPath)
	assert.Nil(t, err)
	t.Logf("The TotalSupply is %d", genesis.TotalSupply)
	genesisBlk := NewGenesisBlock(genesis)
	t.Log("The Genesis Block has the following header:")
	t.Logf("Version: %d", genesisBlk.Header.version)
	t.Logf("ChainID: %d", genesisBlk.Header.chainID)
	t.Logf("Height: %d", genesisBlk.Header.height)
	t.Logf("Timestamp: %d", genesisBlk.Header.timestamp)
	t.Logf("PrevBlockHash: %x", genesisBlk.Header.prevBlockHash)
	t.Logf("TrnxNumber: %d", genesisBlk.Header.trnxNumber)
}
