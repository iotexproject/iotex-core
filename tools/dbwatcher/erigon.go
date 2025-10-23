package main

import (
	"context"
	"fmt"
	"slices"

	"github.com/erigontech/erigon-lib/kv"
	"github.com/erigontech/erigon-lib/kv/mdbx"
	erigonlog "github.com/erigontech/erigon-lib/log/v3"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var WatchErigon = &cobra.Command{
	Use:   "erigon",
	Short: "watch erigon",
	Long:  "watch erigon",
	RunE: func(cmd *cobra.Command, args []string) error {
		return watchErigon(args[0])
	},
}
var (
	keyLimit   = uint64(10)
	namespaces = []string{}
)

var errLimitReached = errors.New("key limit reached")

func init() {
	WatchErigon.PersistentFlags().Uint64VarP(&keyLimit, "limit", "l", 10, "key limit")
	WatchErigon.PersistentFlags().StringArrayVarP(&namespaces, "namespace", "n", []string{}, "namespaces to watch")
}

func watchErigon(path string) error {
	fmt.Println("walking erigon", path)
	lg := erigonlog.New()
	lg.SetHandler(erigonlog.StdoutHandler)
	rw, err := mdbx.NewMDBX(lg).Path(path).WithTableCfg(func(defaultBuckets kv.TableCfg) kv.TableCfg {
		defaultBuckets["erigonsystem"] = kv.TableCfgItem{}
		return defaultBuckets
	}).Readonly().Open(context.Background())
	if err != nil {
		return errors.Wrapf(err, "open db: %s", path)
	}
	defer rw.Close()
	return walkdbkv(rw)
}

func walkdbkv(rw kv.RoDB) error {
	tx, err := rw.BeginRo(context.Background())
	if err != nil {
		return errors.Wrap(err, "begin ro")
	}
	defer tx.Rollback()

	dbsize, err := tx.DBSize()
	if err != nil {
		return errors.Wrap(err, "db size")
	}
	fmt.Printf("db size: %d\n\n", dbsize)
	tables, err := tx.ListBuckets()
	if err != nil {
		return errors.Wrap(err, "list tables")
	}
	if len(namespaces) > 0 {
		filtered := []string{}
		for _, table := range tables {
			if slices.Contains(namespaces, table) {
				filtered = append(filtered, table)
			}
		}
		tables = filtered
	}
	total := uint64(0)
	for _, table := range tables {
		tsize, err := tx.BucketSize(table)
		if err != nil {
			return errors.Wrapf(err, "table size: %s", table)
		}
		if tsize == 0 {
			continue
		}
		total += tsize
		keynum := uint64(0)
		err = tx.ForEach(table, nil, func(k, v []byte) error {
			if keyLimit == 0 || keynum < keyLimit {
				fmt.Printf("table: %s, key: %x, value: %x\n", table, k, v)
			}
			if keyLimit > 0 && keynum >= keyLimit {
				return errLimitReached
			}
			keynum++
			return nil
		})
		if err != nil && !errors.Is(err, errLimitReached) {
			return errors.Wrapf(err, "for each table: %s", table)
		}
		fmt.Printf("table: %s, size: %d, keynum: %d\n\n", table, tsize, keynum)
	}
	fmt.Printf("total size used: %d\n", total)
	return nil
}
