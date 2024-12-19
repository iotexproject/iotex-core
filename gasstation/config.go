// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package gasstation

import "github.com/iotexproject/iotex-core/v2/pkg/unit"

// Config is the gas station config
type Config struct {
	SuggestBlockWindow  int    `yaml:"suggestBlockWindow"`
	DefaultGas          uint64 `yaml:"defaultGas"`
	Percentile          int    `yaml:"Percentile"`
	FeeHistoryCacheSize int    `yaml:"feeHistoryCacheSize"`
}

// DefaultConfig is the default config
var DefaultConfig = Config{
	SuggestBlockWindow:  20,
	DefaultGas:          uint64(unit.Qev),
	Percentile:          60,
	FeeHistoryCacheSize: 1024,
}
