// Copyright (c) 2021 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

// Config is the config for database
type Config struct {
	DbPath string `yaml:"dbPath"`
	// NumRetries is the number of retries
	NumRetries uint8 `yaml:"numRetries"`
	// MaxCacheSize is the max number of blocks that will be put into an LRU cache. 0 means disabled
	MaxCacheSize int `yaml:"maxCacheSize"`
	// BlockStoreBatchSize is the number of blocks to be stored together as a unit (to get better compression)
	BlockStoreBatchSize int `yaml:"blockStoreBatchSize"`
	// V2BlocksToSplitDB is the accumulated number of blocks to split a new file after v1.1.2
	V2BlocksToSplitDB uint64 `yaml:"v2BlocksToSplitDB"`
	// Compressor is the compression used on block data, used by new DB file after v1.1.2
	Compressor string `yaml:"compressor"`
	// CompressLegacy enables gzip compression on block data, used by legacy DB file before v1.1.2
	CompressLegacy bool `yaml:"compressLegacy"`
	// SplitDBSize is the config for DB's split file size
	SplitDBSizeMB uint64 `yaml:"splitDBSizeMB"`
	// SplitDBHeight is the config for DB's split start height
	SplitDBHeight uint64 `yaml:"splitDBHeight"`
	// HistoryStateRetention is the number of blocks account/contract state will be retained
	HistoryStateRetention uint64 `yaml:"historyStateRetention"`
	// ReadOnly is set db to be opened in read only mode
	ReadOnly bool `yaml:"readOnly"`
}

// SplitDBSize returns the configured SplitDBSizeMB
func (cfg Config) SplitDBSize() uint64 {
	return cfg.SplitDBSizeMB * 1024 * 1024
}

// DefaultConfig returns the default config
var DefaultConfig = Config{
	NumRetries:            3,
	MaxCacheSize:          64,
	BlockStoreBatchSize:   16,
	V2BlocksToSplitDB:     1000000,
	Compressor:            "Snappy",
	CompressLegacy:        false,
	SplitDBSizeMB:         0,
	SplitDBHeight:         900000,
	HistoryStateRetention: 2000,
}
