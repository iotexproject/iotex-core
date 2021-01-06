// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package sql

import (
	"fmt"

	// we need mysql import because it's called in file, (but compile will complain because there is no display)
	_ "github.com/go-sql-driver/mysql"
)

// RDS is the cloud rds config
type RDS struct {
	// AwsRDSEndpoint is the endpoint of aws rds
	AwsRDSEndpoint string `yaml:"awsRDSEndpoint"`
	// AwsRDSPort is the port of aws rds
	AwsRDSPort uint64 `yaml:"awsRDSPort"`
	// AwsRDSUser is the user to access aws rds
	AwsRDSUser string `yaml:"awsRDSUser"`
	// AwsPass is the pass to access aws rds
	AwsPass string `yaml:"awsPass"`
	// AwsDBName is the db name of aws rds
	AwsDBName string `yaml:"awsDBName"`
}

// NewAwsRDS instantiates an aws rds
func NewAwsRDS(cfg RDS) Store {
	connectStr := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s",
		cfg.AwsRDSUser, cfg.AwsPass, cfg.AwsRDSEndpoint, cfg.AwsRDSPort, cfg.AwsDBName,
	)
	return newStoreBase("mysql", connectStr)
}
