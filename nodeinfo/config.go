// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package nodeinfo

import "time"

// Config node config
type Config struct {
	EnableBroadcastNodeInfo   bool          `yaml:"enableBroadcastNodeInfo"`
	BroadcastNodeInfoInterval time.Duration `yaml:"broadcastNodeInfoInterval"`
	NodeMapSize               int           `yaml:"nodeMapSize"`
}

// DefaultConfig is the default config
var DefaultConfig = Config{
	EnableBroadcastNodeInfo:   false,
	BroadcastNodeInfoInterval: 5 * time.Minute,
	NodeMapSize:               1000,
}
