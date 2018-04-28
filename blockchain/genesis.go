// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"io/ioutil"
	"path/filepath"
	"runtime"

	"github.com/golang/glog"
	"gopkg.in/yaml.v2"

	cp "github.com/iotexproject/iotex-core/crypto"
	ta "github.com/iotexproject/iotex-core/test/testaddress"
)

var (
	// DefaultGenesisPath is the default genesis path
	DefaultGenesisPath = basePath() + "/../genesis.yaml"
)

// Alloc defines the allocation of orginal tokens to the given address
type Alloc struct {
	Address string
	Balance uint64
}

// GenConfig defines the Genesis Configuration
type GenConfig struct {
	ChainID uint32
}

// Genesis defines the Genesis default settings
type Genesis struct {
	Allocs              []Alloc
	GenConfig           GenConfig
	TotalSupply         uint64
	Coinbase            uint64
	Timestamp           uint64
	ParentHash          cp.Hash32B
	GenesisCoinbaseData string
}

func basePath() string {
	_, b, _, _ := runtime.Caller(0)
	return filepath.Dir(b)
}

// NewGenesisBlock creates a new genesis block
func NewGenesisBlock(gen *Genesis) *Block {
	cbtx := NewCoinbaseTx(ta.Addrinfo["miner"].Address, gen.TotalSupply, gen.GenesisCoinbaseData)
	if cbtx == nil {
		glog.Error("Cannot create coinbase transaction")
		return nil
	}
	block := &Block{
		Header: &BlockHeader{Version, gen.GenConfig.ChainID, uint32(0), gen.Timestamp, gen.ParentHash, cp.ZeroHash32B, uint32(1), 0},
		Tranxs: []*Tx{cbtx},
	}

	block.Header.merkleRoot = block.MerkleRoot()

	for _, tx := range block.Tranxs {
		// add up trnx size
		block.Header.trnxDataSize += tx.TotalSize()
	}

	return block
}

// LoadGenesisWithPath loads genesis default settings from given yaml file path
func LoadGenesisWithPath(path string) (*Genesis, error) {
	genesisBytes, err := ioutil.ReadFile(path)
	if err != nil {
		glog.Errorf("Error when reading the genesis file: %v\n", err)
		return nil, err
	}

	genesis := Genesis{}
	err = yaml.Unmarshal(genesisBytes, &genesis)
	if err != nil {
		glog.Errorf("Error when decoding the genesis file: %v\n", err)
		return nil, err
	}
	return &genesis, nil
}
