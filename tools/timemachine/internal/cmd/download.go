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
	"path/filepath"
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
	_objPrefix  = "fullsync/mainnet/"
	_latest     = 20000000
)

// download represents the download command
var download = &cobra.Command{
	Use:   "download [height]",
	Short: "Download height block datas into ./tools/timemachine/data/xxm/",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		height, err := strconv.ParseUint(args[0], 10, 64)
		if err != nil {
			return errors.Wrapf(err, "Failed to convert input height: %s.", args[0])
		}
		if height == 0 {
			return errors.New("input height cannot be 0")
		}

		heightDir := genHeightDir(height)
		log.S().Infof("downloading %s", heightDir)

		objs, err := listFiles(_bucket, _objPrefix+heightDir+"/", "/")
		if err != nil {
			return err
		}
		if len(objs) == 0 {
			return errors.New("height is not exist")
		}

		var wg sync.WaitGroup
		for _, obj := range objs {
			wg.Add(1)
			go func(obj string) {
				defer wg.Done()
				if err := downloadFile(_bucket, obj, strings.TrimPrefix(obj, _objPrefix)); err != nil {
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
	dbpath = fmt.Sprintf("./tools/timemachine/data/%s", dbpath)
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

func genHeightDir(height uint64) (heightDir string) {
	if height < 8000000 {
		heightDir = "0m"
	} else if height >= 8000000 && height < 12000000 {
		heightDir = "8m"
	} else if height >= 12000000 && height < 13000000 {
		heightDir = "12m"
	} else if height >= 13000000 && height < _latest {
		inter := height / 1000000
		interStr := fmt.Sprintf("%dm", inter)
		deci := height - inter*1000000
		if deci < 250000 {
			heightDir = interStr
		} else if deci >= 250000 && deci < 500000 {
			heightDir = interStr + "25"
		} else if deci >= 500000 && deci < 750000 {
			heightDir = interStr + "50"
		} else {
			heightDir = interStr + "75"
		}
	} else {
		heightDir = "latest"
	}
	return
}

func mkdirIfNotExist(destDir string) error {
	if _, err := os.Stat(destDir); os.IsNotExist(err) {
		if err = os.MkdirAll(destDir, 0744); err != nil {
			return errors.Wrapf(err, "Failed to create dir: %s", destDir)
		}
	}
	return nil
}
