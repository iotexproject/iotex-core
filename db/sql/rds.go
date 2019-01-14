// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package sql

import (
	"fmt"

	// we need mysql import because it's called in file, (but compile will complain because there is no display)
	_ "github.com/go-sql-driver/mysql"

	"github.com/iotexproject/iotex-core/config"
)

// NewAwsRDS instantiates an aws rds
func NewAwsRDS(cfg config.RDS) Store {
	connectStr := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s",
		cfg.AwsRDSUser, cfg.AwsPass, cfg.AwsRDSEndpoint, cfg.AwsRDSPort, cfg.AwsDBName,
	)
	return newStoreBase("mysql", connectStr)
}
