// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package sql

import (
	"testing"

	"github.com/iotexproject/iotex-core/config"
)

func TestRDSStorePutGet(t *testing.T) {
	t.Skip("Skipping when RDS credentail not provided.")
	testRDSStorePutGet := TestStorePutGet

	cfg := config.RDS{}
	t.Run("RDS Store", func(t *testing.T) {
		testRDSStorePutGet(NewAwsRDS(cfg), t)
	})
}

func TestRDSStoreTransaction(t *testing.T) {
	t.Skip("Skipping when RDS credentail not provided.")
	testRDSStoreTransaction := TestStoreTransaction

	cfg := config.RDS{}
	t.Run("RDS Store", func(t *testing.T) {
		testRDSStoreTransaction(NewAwsRDS(cfg), t)
	})
}
