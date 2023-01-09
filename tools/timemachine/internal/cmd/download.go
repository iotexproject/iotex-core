// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/storage"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"google.golang.org/api/iterator"

	"github.com/iotexproject/iotex-core/pkg/log"
)

// const
const (
	_gcpTimeout = time.Second * 60
	_bucket     = "blockchain-golden"
	_objPrefix  = "fullsync/"
)

// download represents the download command
var download = &cobra.Command{
	Use:   "download [mainnet/testnet] [height]",
	Short: "Download height block datas into ./data/xxm/",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		if args[0] != "mainnet" && args[0] != "testnet" {
			return errors.New("block data is not from mainnet or testnet")
		}
		height, err := strconv.ParseUint(args[1], 10, 64)
		if err != nil {
			return errors.Wrapf(err, "Failed to convert input height: %s.", args[1])
		}
		if height == 0 {
			return errors.New("input height cannot be 0")
		}
		objPrefix := _objPrefix + args[0] + "/"
		objs, err := listFiles(_bucket, objPrefix, "")
		if err != nil {
			return err
		}
		if len(objs) == 0 {
			return errors.New("height is not exist")
		}

		dbnames := make(map[uint64][]string)
		orders := []uint64{}
		m := make(map[uint64]int)
		idx := -1
		latest := []string{}
		for _, obj := range objs {
			if !strings.HasSuffix(obj, ".db") {
				continue
			}

			dbpath := strings.TrimPrefix(obj, objPrefix)
			heightStr := path.Dir(dbpath)
			var h uint64
			if heightStr != "latest" {
				h = convertHeightStr(heightStr)
			} else {
				latest = append(latest, dbpath)
			}

			if _, ok := m[h]; !ok {
				orders = append(orders, h)
				m[h] = idx
				idx++
			}
			dbnames[h] = append(dbnames[h], dbpath)
		}

		sort.Slice(orders, func(i, j int) bool {
			return orders[i] < orders[j]
		})

		latestHeight := orders[len(orders)-1] + 250000
		orders = append(orders, latestHeight)
		dbnames[latestHeight] = latest

		hi := sort.Search(len(orders), func(i int) bool {
			return orders[i] > height
		})
		hi--

		var wg sync.WaitGroup
		for _, obj := range dbnames[orders[hi]] {
			wg.Add(1)
			go func(obj string) {
				defer wg.Done()
				if err := downloadFile(_bucket, objPrefix+obj, obj); err != nil {
					panic(errors.Wrapf(err, "Failed to downloadFile: %s.", obj))
				}
			}(obj)
		}
		wg.Wait()

		log.S().Infof("successful download height %d", height)
		return nil
	},
}

// downloadFile downloads an object to a file.
func downloadFile(bucket, object, dbpath string) error {
	dbpath = fmt.Sprintf("./data/%s", dbpath)
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		return errors.Wrap(err, "Failed to storage.NewClient.")
	}
	defer client.Close()
	client.SetRetry(storage.WithPolicy(storage.RetryAlways))

	ctx, cancel := context.WithTimeout(ctx, _gcpTimeout)
	defer cancel()

	if err = mkdirIfNotExist(filepath.Dir(dbpath)); err != nil {
		return err
	}
	f, err := os.Create(dbpath)
	if err != nil {
		return errors.Wrapf(err, "Failed to os.Create %s", dbpath)
	}

	log.S().Infof("downlaoding %s to %s", object, dbpath)

	rc, err := client.Bucket(bucket).Object(object).NewReader(ctx)
	if err != nil {
		return errors.Wrapf(err, "Failed to Object(%q).NewReader %s", object, dbpath)
	}
	defer rc.Close()

	if _, err = io.Copy(f, rc); err != nil {
		return errors.Wrapf(err, "Failed to io.Copy %s", dbpath)
	}

	if err = f.Close(); err != nil {
		return errors.Wrapf(err, "Failed to f.Close %s", dbpath)
	}
	return nil
}

// listFiles lists objects within specified bucket.
func listFiles(bucket, prefix, delim string) ([]string, error) {
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to storage.NewClient.")
	}
	defer client.Close()
	client.SetRetry(storage.WithPolicy(storage.RetryAlways))

	ctx, cancel := context.WithTimeout(ctx, _gcpTimeout)
	defer cancel()

	query := &storage.Query{
		Prefix:    prefix,
		Delimiter: delim,
	}
	if err := query.SetAttrSelection([]string{"Name"}); err != nil {
		return nil, errors.Wrap(err, "Failed to query.SetAttrSelection.")
	}
	it := client.Bucket(bucket).Objects(ctx, query)
	objNames := make([]string, 0)
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to client.Bucket(%q).Objects", bucket)
		}
		if attrs.Name != "" && attrs.Name != prefix {
			objNames = append(objNames, attrs.Name)
		}
	}
	return objNames, nil
}

func mkdirIfNotExist(destDir string) error {
	if _, err := os.Stat(destDir); os.IsNotExist(err) {
		if err = os.MkdirAll(destDir, 0744); err != nil {
			return errors.Wrapf(err, "Failed to create dir: %s", destDir)
		}
	}
	return nil
}

func convertHeightStr(heiStr string) uint64 {
	idx := strings.Index(heiStr, "m")
	if idx == -1 {
		panic(errors.New("height dir is not xxm"))
	}
	inter, err := strconv.ParseUint(heiStr[:idx], 10, 64)
	if err != nil {
		panic(err)
	}
	var deci uint64
	if len(heiStr) > idx+1 {
		deci, err = strconv.ParseUint(heiStr[idx+1:], 10, 64)
		if err != nil {
			panic(err)
		}
	}
	return inter*1000000 + deci*10000
}
