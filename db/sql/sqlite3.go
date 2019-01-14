// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package sql

import (
	// this is required for sqlite3 usage
	_ "github.com/mattn/go-sqlite3"

	"github.com/iotexproject/iotex-core/config"
)

// NewSQLite3 instantiates an sqlite3
func NewSQLite3(cfg config.SQLITE3) Store {
	return newStoreBase("sqlite3", cfg.SQLite3File)
}
