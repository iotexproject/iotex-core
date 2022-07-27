// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockindex

// Config is the config for indexer
type Config struct {
	// RangeBloomFilterNumElements is the number of elements each rangeBloomfilter will store in bloomfilterIndexer
	RangeBloomFilterNumElements uint64 `yaml:"rangeBloomFilterNumElements"`
	// RangeBloomFilterSize is the size (in bits) of rangeBloomfilter
	RangeBloomFilterSize uint64 `yaml:"rangeBloomFilterSize"`
	// RangeBloomFilterNumHash is the number of hash functions of rangeBloomfilter
	RangeBloomFilterNumHash uint64 `yaml:"rangeBloomFilterNumHash"`
}

// DefaultConfig is the default config of indexer
var DefaultConfig = Config{
	RangeBloomFilterNumElements: 100000,
	RangeBloomFilterSize:        1200000,
	RangeBloomFilterNumHash:     8,
}
