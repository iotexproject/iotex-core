// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"crypto/ecdsa"
	"time"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-election/committee"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/log"
)

type (
	// Config is the config struct for blockchain package
	Config struct {
		ChainDBPath            string           `yaml:"chainDBPath"`
		TrieDBPatchFile        string           `yaml:"trieDBPatchFile"`
		TrieDBPath             string           `yaml:"trieDBPath"`
		IndexDBPath            string           `yaml:"indexDBPath"`
		BloomfilterIndexDBPath string           `yaml:"bloomfilterIndexDBPath"`
		CandidateIndexDBPath   string           `yaml:"candidateIndexDBPath"`
		StakingIndexDBPath     string           `yaml:"stakingIndexDBPath"`
		ID                     uint32           `yaml:"id"`
		EVMNetworkID           uint32           `yaml:"evmNetworkID"`
		Address                string           `yaml:"address"`
		ProducerPrivKey        string           `yaml:"producerPrivKey"`
		PrivKeyConfigFile      string           `yaml:"privKeyConfigFile"`
		SignatureScheme        []string         `yaml:"signatureScheme"`
		EmptyGenesis           bool             `yaml:"emptyGenesis"`
		GravityChainDB         db.Config        `yaml:"gravityChainDB"`
		Committee              committee.Config `yaml:"committee"`

		EnableTrielessStateDB bool `yaml:"enableTrielessStateDB"`
		// EnableStateDBCaching enables cachedStateDBOption
		EnableStateDBCaching bool `yaml:"enableStateDBCaching"`
		// EnableArchiveMode is only meaningful when EnableTrielessStateDB is false
		EnableArchiveMode bool `yaml:"enableArchiveMode"`
		// EnableAsyncIndexWrite enables writing the block actions' and receipts' index asynchronously
		EnableAsyncIndexWrite bool `yaml:"enableAsyncIndexWrite"`
		// deprecated
		EnableSystemLogIndexer bool `yaml:"enableSystemLog"`
		// EnableStakingProtocol enables staking protocol
		EnableStakingProtocol bool `yaml:"enableStakingProtocol"`
		// EnableStakingIndexer enables staking indexer
		EnableStakingIndexer bool `yaml:"enableStakingIndexer"`
		// AllowedBlockGasResidue is the amount of gas remained when block producer could stop processing more actions
		AllowedBlockGasResidue uint64 `yaml:"allowedBlockGasResidue"`
		// MaxCacheSize is the max number of blocks that will be put into an LRU cache. 0 means disabled
		MaxCacheSize int `yaml:"maxCacheSize"`
		// PollInitialCandidatesInterval is the config for committee init db
		PollInitialCandidatesInterval time.Duration `yaml:"pollInitialCandidatesInterval"`
		// StateDBCacheSize is the max size of statedb LRU cache
		StateDBCacheSize int `yaml:"stateDBCacheSize"`
		// WorkingSetCacheSize is the max size of workingset cache in state factory
		WorkingSetCacheSize uint64 `yaml:"workingSetCacheSize"`
		// StreamingBlockBufferSize
		StreamingBlockBufferSize uint64 `yaml:"streamingBlockBufferSize"`
	}
)

// DefaultConfig is the default config of chain
var DefaultConfig = Config{
	ChainDBPath:            "/var/data/chain.db",
	TrieDBPatchFile:        "/var/data/trie.db.patch",
	TrieDBPath:             "/var/data/trie.db",
	IndexDBPath:            "/var/data/index.db",
	BloomfilterIndexDBPath: "/var/data/bloomfilter.index.db",
	CandidateIndexDBPath:   "/var/data/candidate.index.db",
	StakingIndexDBPath:     "/var/data/staking.index.db",
	ID:                     1,
	EVMNetworkID:           4689,
	Address:                "",
	ProducerPrivKey:        generateRandomKey(SigP256k1),
	SignatureScheme:        []string{SigP256k1},
	EmptyGenesis:           false,
	GravityChainDB:         db.Config{DbPath: "/var/data/poll.db", NumRetries: 10},
	Committee: committee.Config{
		GravityChainAPIs: []string{},
	},
	EnableTrielessStateDB:         true,
	EnableStateDBCaching:          false,
	EnableArchiveMode:             false,
	EnableAsyncIndexWrite:         true,
	EnableSystemLogIndexer:        false,
	EnableStakingProtocol:         true,
	EnableStakingIndexer:          false,
	AllowedBlockGasResidue:        10000,
	MaxCacheSize:                  0,
	PollInitialCandidatesInterval: 10 * time.Second,
	StateDBCacheSize:              1000,
	WorkingSetCacheSize:           20,
	StreamingBlockBufferSize:      200,
}

// ProducerAddress returns the configured producer address derived from key
func (cfg *Config) ProducerAddress() address.Address {
	sk := cfg.ProducerPrivateKey()
	addr := sk.PublicKey().Address()
	if addr == nil {
		log.L().Panic("Error when constructing producer address")
	}
	return addr
}

// ProducerPrivateKey returns the configured private key
func (cfg *Config) ProducerPrivateKey() crypto.PrivateKey {
	sk, err := crypto.HexStringToPrivateKey(cfg.ProducerPrivKey)
	if err != nil {
		log.L().Panic(
			"Error when decoding private key",
			zap.Error(err),
		)
	}

	if !cfg.whitelistSignatureScheme(sk) {
		log.L().Panic("The private key's signature scheme is not whitelisted")
	}
	return sk
}

func generateRandomKey(scheme string) string {
	// generate a random key
	switch scheme {
	case SigP256k1:
		sk, _ := crypto.GenerateKey()
		return sk.HexString()
	case SigP256sm2:
		sk, _ := crypto.GenerateKeySm2()
		return sk.HexString()
	}
	return ""
}

func (cfg *Config) whitelistSignatureScheme(sk crypto.PrivateKey) bool {
	var sigScheme string

	switch sk.EcdsaPrivateKey().(type) {
	case *ecdsa.PrivateKey:
		sigScheme = SigP256k1
	case *crypto.P256sm2PrvKey:
		sigScheme = SigP256sm2
	}

	if sigScheme == "" {
		return false
	}
	for _, e := range cfg.SignatureScheme {
		if sigScheme == e {
			// signature scheme is whitelisted
			return true
		}
	}
	return false
}
