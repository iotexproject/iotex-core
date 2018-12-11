// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"context"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/state/factory"
)

func TestGenesis(t *testing.T) {
	assert := assert.New(t)

	cfg := config.Default
	sf, err := factory.NewFactory(cfg, factory.PrecreatedTrieDBOption(db.NewMemKVStore()))
	assert.NoError(err)
	assert.NoError(sf.Start(context.Background()))
	ws, err := sf.NewWorkingSet()
	assert.NoError(err)
	genesisBlk := NewGenesisBlock(cfg.Chain, ws)
	assert.NotNil(genesisBlk)

	t.Log("The Genesis Block has the following header:")
	t.Logf("Version: %d", genesisBlk.Header.version)
	t.Logf("ChainID: %d", genesisBlk.Header.chainID)
	t.Logf("Height: %d", genesisBlk.Header.height)
	t.Logf("Timestamp: %d", genesisBlk.Header.timestamp)
	t.Logf("PrevBlockHash: %x", genesisBlk.Header.prevBlockHash)

	assert.Equal(uint32(1), genesisBlk.Header.version)
	assert.Equal(cfg.Chain.ID, genesisBlk.Header.chainID)
	assert.Equal(uint64(0), genesisBlk.Header.height)
	assert.Equal(uint64(1524676419), genesisBlk.Header.timestamp)
	assert.Equal(hash.ZeroHash32B, genesisBlk.Header.prevBlockHash)
	genesisHash, _ := hex.DecodeString("237eab11367eaaf2c0b1916f5009e87da4298d579c0c8697e96b9f557d23d877")
	h := genesisBlk.HashBlock()
	assert.Equal(genesisHash, h[:])
}
