// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blocksync

import "time"

// Config is the config struct for the BlockSync
type Config struct {
	Interval              time.Duration `yaml:"interval"` // update duration
	ProcessSyncRequestTTL time.Duration `yaml:"processSyncRequestTTL"`
	BufferSize            uint64        `yaml:"bufferSize"`
	IntervalSize          uint64        `yaml:"intervalSize"`
	// MaxRepeat is the maximal number of repeat of a block sync request
	MaxRepeat int `yaml:"maxRepeat"`
	// RepeatDecayStep is the step for repeat number decreasing by 1
	RepeatDecayStep int `yaml:"repeatDecayStep"`
	// MaxBlocksPerSyncRequest is the maximal number of blocks a single
	// ProcessSyncRequest may serve, regardless of the requested start..end range.
	// It bounds the serve-amplification of a cheap {start:0,end:tip} request: a
	// peer cannot force the node to re-serialize and unicast its entire chain in
	// one request. If 0, the cap is disabled (NOT recommended).
	MaxBlocksPerSyncRequest uint64 `yaml:"maxBlocksPerSyncRequest"`
}

// DefaultConfig is the default config
var DefaultConfig = Config{
	Interval:                30 * time.Second,
	ProcessSyncRequestTTL:   10 * time.Second,
	BufferSize:              200,
	IntervalSize:            20,
	MaxRepeat:               3,
	RepeatDecayStep:         1,
	MaxBlocksPerSyncRequest: 128,
}
