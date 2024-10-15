package main

import (
	"context"
	"fmt"

	"github.com/iotexproject/iotex-core/v2/db"
)

func main() {
	cfg := db.DefaultConfig
	cfg.DbPath = "../../data/archive.db"
	db1 := db.NewBoltDBVersioned(cfg)
	ctx := context.Background()
	if err := db1.Start(ctx); err != nil {
		panic(err.Error())
	}
	println("======= source file:", cfg.DbPath)
	defer func() {
		db1.Stop(ctx)
	}()
	cfg.DbPath = "../../data/archive.db2"
	db2 := db.NewBoltDBVersioned(cfg)
	if err := db2.Start(ctx); err != nil {
		panic(err.Error())
	}
	defer func() {
		db2.Stop(ctx)
	}()
	buckets, err := db1.Buckets()
	if err != nil {
		panic(err.Error())
	}
	fmt.Printf("source buckets = %v\n", buckets)
	buckets, err = db2.Buckets()
	if err != nil {
		panic(err.Error())
	}
	println("======= dest file:", cfg.DbPath)
	fmt.Printf("dest buckets = %v\n", buckets)
	var height uint64 = 17000000
	println("======= merge at height:", height)
	for _, v := range buckets {
		if v == "Account" || v == "Contract" {
			if err := db2.PurgeVersion(v, height); err != nil {
				panic(err.Error())
			}
		} else {
			if err := db1.PurgeVersion(v, height); err != nil {
				panic(err.Error())
			}
		}
		if err := db1.CopyBucket(v, db2); err != nil {
			panic(err.Error())
		}
	}
}
