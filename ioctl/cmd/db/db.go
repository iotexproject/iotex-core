// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package db

import (
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/v2/ioctl/config"
)

// Multi-language support
var (
	_dbCmdShorts = map[config.Language]string{
		config.English: "Read/write to IoTeX database",
		config.Chinese: "读取/写入 IoTeX 数据库",
	}
)

// DBCmd represents the db command
var DBCmd = &cobra.Command{
	Use:   "db",
	Short: config.TranslateInLang(_dbCmdShorts, config.UILanguage),
}

func init() {
	DBCmd.AddCommand(_dbGetCmd)
}
