// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package db

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/ioctl/config"
)

// Multi-language support
var (
	_dbGetCmdShorts = map[config.Language]string{
		config.English: "Get value by namespace and key from database",
		config.Chinese: "通过命名空间和键从数据库获取值",
	}
	_dbGetCmdUses = map[config.Language]string{
		config.English: "get [NAMESPACE] [KEY]",
		config.Chinese: "get [命名空间] [键]",
	}
	_flagDBPathUsages = map[config.Language]string{
		config.English: "path to the database file",
		config.Chinese: "数据库文件路径",
	}
	_flagHexKeyUsages = map[config.Language]string{
		config.English: "treat key as hex string",
		config.Chinese: "将键视为十六进制字符串",
	}
	_flagHexOutputUsages = map[config.Language]string{
		config.English: "output value as hex string",
		config.Chinese: "将值输出为十六进制字符串",
	}
	_flagDBTypeUsages = map[config.Language]string{
		config.English: "Type of the database (boltdb or pebbledb)",
		config.Chinese: "数据库类型（boltdb 或 pebbledb）",
	}
)

var (
	_dbPath    string
	_dbType    string
	_hexKey    bool
	_hexOutput bool
)

// _dbGetCmd represents the db get command
var _dbGetCmd = &cobra.Command{
	Use:   config.TranslateInLang(_dbGetCmdUses, config.UILanguage),
	Short: config.TranslateInLang(_dbGetCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(2),
	RunE: func(_ *cobra.Command, args []string) error {
		namespace := args[0]
		keyStr := args[1]

		// Parse key
		var key []byte
		var err error
		if _hexKey {
			key, err = hex.DecodeString(keyStr)
			if err != nil {
				return fmt.Errorf("failed to decode hex key: %w", err)
			}
		} else {
			key = []byte(keyStr)
		}

		// Open database
		cfg := db.DefaultConfig
		cfg.DbPath = _dbPath
		cfg.DBType = _dbType
		cfg.ReadOnly = true

		kvStore, err := db.CreateKVStore(cfg, _dbPath)
		if err != nil {
			return fmt.Errorf("failed to create kv store: %w", err)
		}
		ctx := context.Background()

		if err := kvStore.Start(ctx); err != nil {
			return fmt.Errorf("failed to start database: %w", err)
		}
		defer func() {
			if err := kvStore.Stop(ctx); err != nil {
				fmt.Printf("failed to stop database: %v\n", err)
			}
		}()

		// Get value
		value, err := kvStore.Get(namespace, key)
		if err != nil {
			return fmt.Errorf("failed to get value: %w", err)
		}

		// Output result
		if _hexOutput {
			fmt.Printf("Namespace: %s\n", namespace)
			fmt.Printf("Key: %s\n", keyStr)
			fmt.Printf("Value (hex): %s\n", hex.EncodeToString(value))
		} else {
			fmt.Printf("Namespace: %s\n", namespace)
			fmt.Printf("Key: %s\n", keyStr)
			fmt.Printf("Value: %s\n", string(value))
		}

		return nil
	},
}

func init() {
	_dbGetCmd.Flags().StringVarP(&_dbPath, "db-path", "p", "./chain.db",
		config.TranslateInLang(_flagDBPathUsages, config.UILanguage))
	_dbGetCmd.Flags().BoolVar(&_hexKey, "hex-key", false,
		config.TranslateInLang(_flagHexKeyUsages, config.UILanguage))
	_dbGetCmd.Flags().BoolVar(&_hexOutput, "hex-output", false,
		config.TranslateInLang(_flagHexOutputUsages, config.UILanguage))
	_dbGetCmd.Flags().StringVar(&_dbType, "db-type", db.DBPebble,
		config.TranslateInLang(_flagDBTypeUsages, config.UILanguage))
}
