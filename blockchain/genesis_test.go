// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iotexproject/iotex-core/common"
)

func TestGenesis(t *testing.T) {
	t.Logf("The TotalSupply is %d", Gen.TotalSupply)

	genesisBlk := NewGenesisBlock()

	t.Log("The Genesis Block has the following header:")
	t.Logf("Version: %d", genesisBlk.Header.version)
	t.Logf("ChainID: %d", genesisBlk.Header.chainID)
	t.Logf("Height: %d", genesisBlk.Header.height)
	t.Logf("Timestamp: %d", genesisBlk.Header.timestamp)
	t.Logf("PrevBlockHash: %x", genesisBlk.Header.prevBlockHash)
	t.Logf("TrnxNumber: %d", genesisBlk.Header.trnxNumber)

	assert := assert.New(t)

	expectedParentHash := common.Hash32B{}

	assert.Equal(uint32(1), genesisBlk.Header.version)
	assert.Equal(uint32(1), genesisBlk.Header.chainID)
	assert.Equal(uint64(0), genesisBlk.Header.height)
	assert.Equal(uint64(1524676419), genesisBlk.Header.timestamp)
	assert.Equal(expectedParentHash, genesisBlk.Header.prevBlockHash)
	assert.Equal(uint32(1), genesisBlk.Header.trnxNumber)
}
