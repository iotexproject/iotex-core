package main

import (
	"context"
	"fmt"
	"os"

	"github.com/iotexproject/iotex-core/v2/db"
)

func main() {
	cfg := db.DefaultConfig
	cfg.DbPath = "../../data/archive.db"
	db1 := db.NewBoltDBVersioned(cfg)
	ctx := context.Background()
	if err := db1.Start(ctx); err != nil {
		println(err.Error())
		os.Exit(1)
	}
	println("======= source file:", cfg.DbPath)
	defer func() {
		db1.Stop(ctx)
	}()
	cfg.DbPath = "../../data/archive.db2"
	db2 := db.NewBoltDBVersioned(cfg)
	if err := db2.Start(ctx); err != nil {
		println(err.Error())
		os.Exit(1)
	}
	defer func() {
		db2.Stop(ctx)
	}()
	buckets, err := db1.Buckets()
	if err != nil {
		println(err.Error())
		os.Exit(1)
	}
	fmt.Printf("source buckets = %v\n", buckets)
	buckets, err = db2.Buckets()
	if err != nil {
		println(err.Error())
		os.Exit(1)
	}
	println("======= dest file:", cfg.DbPath)
	fmt.Printf("dest buckets = %v\n", buckets)
	var height uint64 = 17000000
	println("======= merge at height:", height)
	for _, ns := range buckets {
		if db.IsVersioned(ns) {
			if err := db1.CopyVersionedBucket(ns, height, db2); err != nil {
				println(err.Error())
				os.Exit(1)
			}
		} else {
			if err := db1.CopyBucket(ns, db2); err != nil {
				println(err.Error())
				os.Exit(1)
			}
		}
	}
}
