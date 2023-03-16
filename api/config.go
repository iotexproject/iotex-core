// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package api

import (
	"github.com/iotexproject/iotex-core/gasstation"
	"github.com/iotexproject/iotex-core/pkg/tracer"
)

// Config is the api service config
type Config struct {
	UseRDS          bool              `yaml:"useRDS"`
	GRPCPort        int               `yaml:"port"`
	HTTPPort        int               `yaml:"web3port"`
	WebSocketPort   int               `yaml:"webSocketPort"`
	RedisCacheURL   string            `yaml:"redisCacheURL"`
	TpsWindow       int               `yaml:"tpsWindow"`
	GasStation      gasstation.Config `yaml:"gasStation"`
	RangeQueryLimit uint64            `yaml:"rangeQueryLimit"`
	Tracer          tracer.Config     `yaml:"tracer"`
	// BatchRequestLimit is the maximum number of requests in a batch.
	BatchRequestLimit int `yaml:"batchRequestLimit"`
}

// DefaultConfig is the default config
var DefaultConfig = Config{
	UseRDS:            false,
	GRPCPort:          14014,
	HTTPPort:          15014,
	WebSocketPort:     16014,
	TpsWindow:         10,
	GasStation:        gasstation.DefaultConfig,
	RangeQueryLimit:   1000,
	BatchRequestLimit: _defaultBatchRequestLimit,
}
