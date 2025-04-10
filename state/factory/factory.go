// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"
	"sync"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/iotexproject/go-pkgs/cache"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/actpool"
	"github.com/iotexproject/iotex-core/v2/blockchain"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/db/trie"
	"github.com/iotexproject/iotex-core/v2/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/v2/pkg/prometheustimer"
)

const (
	// AccountKVNamespace is the bucket name for account
	AccountKVNamespace = "Account"
	// ArchiveNamespacePrefix is the prefix of the buckets storing history data
	ArchiveNamespacePrefix = "Archive"
	// CurrentHeightKey indicates the key of current factory height in underlying DB
	CurrentHeightKey = "currentHeight"
	// ArchiveTrieNamespace is the bucket for the latest state view
	ArchiveTrieNamespace = "AccountTrie"
	// ArchiveTrieRootKey indicates the key of accountTrie root hash in underlying DB
	ArchiveTrieRootKey = "archiveTrieRoot"
)

var (
	// ErrNotSupported is the error that the statedb is not for archive mode
	ErrNotSupported = errors.New("not supported")
	// ErrNoArchiveData is the error that the node have no archive data
	ErrNoArchiveData = errors.New("no archive data")

	_dbBatchSizelMtc = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "iotex_db_batch_size",
			Help: "DB batch size",
		},
		[]string{},
	)

	//DefaultConfig is the default config for state factory
	DefaultConfig = Config{
		Chain:   blockchain.DefaultConfig,
		Genesis: genesis.Default,
	}
)

func init() {
	prometheus.MustRegister(_dbBatchSizelMtc)
}

type (
	// Factory defines an interface for managing states
	Factory interface {
		lifecycle.StartStopper
		protocol.StateReader
		Register(protocol.Protocol) error
		Validate(context.Context, *block.Block) error
		Mint(context.Context, actpool.ActPool, crypto.PrivateKey) (*block.Block, error)
		PutBlock(context.Context, *block.Block) error
		WorkingSet(context.Context) (protocol.StateManager, error)
		WorkingSetAtHeight(context.Context, uint64, ...*action.SealedEnvelope) (protocol.StateManager, error)
		StateReaderAt(blkHeight uint64, blkHash hash.Hash256) (protocol.StateReader, error)
	}

	// factory implements StateFactory interface, tracks changes to account/contract and batch-commits to DB
	factory struct {
		lifecycle                lifecycle.Lifecycle
		mutex                    sync.RWMutex
		cfg                      Config
		registry                 *protocol.Registry
		currentChainHeight       uint64
		saveHistory              bool
		twoLayerTrie             trie.TwoLayerTrie // global state trie, this is a read only trie
		dao                      db.KVStore        // the underlying DB for account/contract storage
		timerFactory             *prometheustimer.TimerFactory
		workingsets              cache.LRUCache // lru cache for workingsets
		protocolView             *protocol.Views
		skipBlockValidationOnPut bool
		ps                       *patchStore
	}

	// Config contains the config for factory
	Config struct {
		Chain   blockchain.Config
		Genesis genesis.Genesis
	}
)

// GenerateConfig generates the factory config
func GenerateConfig(chain blockchain.Config, g genesis.Genesis) Config {
	return Config{
		Chain:   chain,
		Genesis: g,
	}
}
