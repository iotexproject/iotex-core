package rds

import (
	"testing"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
)

func TestRDSStorePutGet(t *testing.T) {
	t.Skip("Skipping when RDS credentail not provided.")
	testRDSStorePutGet := db.TestStorePutGet

	cfg := &config.RDS{}
	t.Run("RDS Store", func(t *testing.T) {
		testRDSStorePutGet(NewAwsRDS(cfg), t)
	})
}

func TestRDSStoreTransaction(t *testing.T) {
	t.Skip("Skipping when RDS credentail not provided.")
	testRDSStoreTransaction := db.TestStoreTransaction

	cfg := &config.RDS{}
	t.Run("RDS Store", func(t *testing.T) {
		testRDSStoreTransaction(NewAwsRDS(cfg), t)
	})
}
