package main

import (
	"context"

	"github.com/iotexproject/iotex-core/db"
)

func main() {
	// file to convert needs to be in /iotex-core/data folder
	// in this example, convert the file at 24m height
	cfg := db.DefaultConfig
	cfg.DbPath = "../../data/trie.24m"
	println("transform file", cfg.DbPath)
	db := db.NewKVStoreWithVersion(cfg, db.VersionedNamespaceOption("Account", "Contract"))
	ctx := context.Background()
	if err := db.Start(ctx); err != nil {
		panic(err.Error())
	}
	defer func() {
		db.Stop(ctx)
	}()

	var v uint64 = 24000000
	if err := db.TransformToVersioned(v, "Account"); err != nil {
		panic(err.Error())
	}
	if err := db.TransformToVersioned(v, "Contract"); err != nil {
		panic(err.Error())
	}
}
