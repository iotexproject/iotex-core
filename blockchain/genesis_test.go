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

	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/iotexproject/iotex-core/test/testaddress"
)

func TestGenesis(t *testing.T) {
	assert := assert.New(t)

	cfg := config.Default
	sf, err := factory.NewFactory(cfg, factory.PrecreatedTrieDBOption(db.NewMemKVStore()))
	assert.NoError(err)
	assert.NoError(sf.Start(context.Background()))
	ws, err := sf.NewWorkingSet()
	assert.NoError(err)
	acts := NewGenesisActions(cfg.Chain, ws)
	racts := block.NewRunnableActionsBuilder().
		SetHeight(0).
		SetTimeStamp(Gen.Timestamp).
		AddActions(acts...).
		Build(testaddress.Addrinfo["producer"].Bech32(), testaddress.Keyinfo["producer"].PubKey)

	genesisBlk, err := block.NewBuilder(racts).
		SetChainID(cfg.Chain.ID).
		SetPrevBlockHash(Gen.ParentHash).
		SignAndBuild(testaddress.Keyinfo["producer"].PubKey, testaddress.Keyinfo["producer"].PriKey)
	assert.NoError(err)

	t.Log("The Genesis Block has the following header:")
	t.Logf("Version: %d", genesisBlk.Version())
	t.Logf("ChainID: %d", genesisBlk.ChainID())
	t.Logf("Height: %d", genesisBlk.Height())
	t.Logf("Timestamp: %d", genesisBlk.Timestamp())
	t.Logf("PrevBlockHash: %x", genesisBlk.PrevHash())

	assert.Equal(uint32(1), genesisBlk.Version())
	assert.Equal(cfg.Chain.ID, genesisBlk.ChainID())
	assert.Equal(uint64(0), genesisBlk.Height())
	assert.Equal(int64(1524676419), genesisBlk.Timestamp())
	assert.Equal(hash.ZeroHash32B, genesisBlk.PrevHash())
	genesisHash, _ := hex.DecodeString("3a258e960726b24ed89dff9b1f785922498b0e930dbbfec1bd6dab1f777f1f6a")
	h := genesisBlk.HashBlock()
	assert.Equal(genesisHash, h[:])
}
