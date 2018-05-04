// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"github.com/golang/glog"

	"github.com/iotexproject/iotex-core/common"
	ta "github.com/iotexproject/iotex-core/test/testaddress"
)

// Quota defines the allocation of orginal tokens to the given address
type Quota struct {
	Address string
	Balance uint64
}

// GenConfig defines the Genesis Configuration
type GenConfig struct {
	ChainID uint32
}

// Genesis defines the Genesis default settings
type Genesis struct {
	Alloc               []Quota
	GenConfig           GenConfig
	TotalSupply         uint64
	BlockReward         uint64
	Timestamp           uint64
	ParentHash          common.Hash32B
	GenesisCoinbaseData string
}

// Gen hardcodes genesis default settings
var Gen = &Genesis{
	Alloc: []Quota{
		Quota{"Whatever Address 1", uint64(1000000)},
		Quota{"Whatever Address 2", uint64(1000000)},
		Quota{"Whatever Address 3", uint64(1000000)},
		Quota{"Whatever Address 4", uint64(1000000)},
	},
	GenConfig:           GenConfig{uint32(1)},
	TotalSupply:         uint64(10000000000),
	BlockReward:         uint64(5),
	Timestamp:           uint64(1524676419),
	ParentHash:          common.Hash32B{},
	GenesisCoinbaseData: "The Times 03/Jan/2009 Chancellor on brink of second bailout for banks",
}

// NewGenesisBlock creates a new genesis block
func NewGenesisBlock(gen *Genesis) *Block {
	cbtx := NewCoinbaseTx(ta.Addrinfo["miner"].RawAddress, gen.TotalSupply, gen.GenesisCoinbaseData)
	if cbtx == nil {
		glog.Error("Cannot create coinbase transaction")
		return nil
	}
	block := &Block{
		Header: &BlockHeader{Version, gen.GenConfig.ChainID, uint64(0), gen.Timestamp,
			gen.ParentHash, common.ZeroHash32B, common.ZeroHash32B, uint32(1),
			0, []byte{}},
		Tranxs: []*Tx{cbtx},
	}

	block.Header.txRoot = block.TxRoot()

	for _, tx := range block.Tranxs {
		// add up trnx size
		block.Header.trnxDataSize += tx.TotalSize()
	}

	return block
}
