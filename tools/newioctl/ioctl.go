// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package main

import (
	"os"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/newcmd"
	"github.com/iotexproject/iotex-core/ioctl/newcmd/config"
)

func main() {
	readConfig, defaultConfigFile, err := config.InitConfig()
	if err != nil {
		os.Exit(1)
	}
	client := ioctl.NewClient(readConfig, defaultConfigFile)
	if err := newcmd.NewIoctl(client).Execute(); err != nil {
		os.Exit(1)
	}
}
