// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package cronjob

import "time"

// Config is the cronjob config
type Config struct {
	MonitorInterval time.Duration `yaml:"monitorInterval"`
}

// DefaultConfig is the default config
var DefaultConfig = Config{
	MonitorInterval: 5 * time.Minute,
}
