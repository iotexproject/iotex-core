// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

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
)

// const value
var (
	GcpTimeout  = time.Second * 60
	ErrNotExist = errors.New("height is not existed.")
)

// downloadHeight represents the download command
var downloadHeight = &cobra.Command{
	Use:   "download",
	Short: "Download specific height of block",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		height, err := strconv.ParseUint(args[0], 10, 64)
		if err != nil {
			return errors.Wrapf(err, "Failed to convert input height: %s.", args[0])
		}
		if height == 0 {
			return errors.New("input height cannot be 0.")
		}
		var (
			heightDir string
			destDir   = args[1]
			bucket    = "blockchain-golden"
			prefix    = "fullsync/mainnet/"
			curr      uint64
			wg        sync.WaitGroup
		)

		allObjNames, err := listFiles(bucket, "", "")
		if err != nil {
			return err
		}
		if len(allObjNames) == 0 {
			return ErrNotExist
		}
		heiNames := make(map[string][]string)
		for _, name := range allObjNames {
			if strings.HasPrefix(name, prefix) {
				s := strings.TrimPrefix(name, prefix)
				if len(s) > 0 {
					heiStr := s[:strings.Index(s, "/")]
					_, ok := heiNames[heiStr]
					if ok {
						heiNames[heiStr] = append(heiNames[heiStr], name)
					} else {
						heiNames[heiStr] = make([]string, 0)
					}

					if heiStr != "latest" {
						v := convertHeightStr(heiStr)
						if v > curr {
							curr = v
						}
					}
				}
			}
		}
		latest := curr + 250000

		if height < 8000000 {
			heightDir = "0m"
		} else if height >= 8000000 && height < 12000000 {
			heightDir = "8m"
		} else if height >= 12000000 && height < 13000000 {
			heightDir = "12m"
		} else if height >= 13000000 && height < latest {
			inter := height / 1000000
			deci := height - inter
			interStr := fmt.Sprintf("%dm", inter)
			if deci < 250000 {
				heightDir = interStr
			} else if deci >= 250000 && deci < 500000 {
				heightDir = interStr + "25"
			} else if deci >= 500000 && deci < 750000 {
				heightDir = interStr + "50"
			} else if deci >= 750000 {
				heightDir = interStr + "75"
			}
		} else if height >= latest && height < latest+250000 {
			heightDir = "latest"
		} else {
			return errors.Errorf("input height: %s is larger than latest height: %v.", args[0], latest+250000)
		}

		cmd.Printf("download height dir is %s.\n", heightDir)

		objs, ok := heiNames[heightDir]
		if !ok {
			return errors.Errorf("Failed to get height: %s", heightDir)
		}
		for _, obj := range objs {
			wg.Add(1)
			go func(obj string) {
				defer wg.Done()
				cmd.Printf("downlaoding height from %s.\n", obj)
				if err := downloadFile(bucket, obj, filepath.Join(destDir, obj)); err != nil {
					panic(errors.Wrapf(err, "Failed to downloadFile: %s.", obj))
				}
			}(obj)
		}
		wg.Wait()

		cmd.Println("download height Successfully.")
		return nil
	},
}

// downloadFile downloads an object to a file.
func downloadFile(bucket, object, destFileName string) error {
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		return errors.Wrap(err, "Failed to storage.NewClient.")
	}
	defer client.Close()
	client.SetRetry(storage.WithPolicy(storage.RetryAlways))

	ctx, cancel := context.WithTimeout(ctx, GcpTimeout)
	defer cancel()

	if err = mkdirIfNotExist(filepath.Dir(destFileName)); err != nil {
		return err
	}
	f, err := os.Create(destFileName)
	if err != nil {
		return errors.Wrap(err, "Failed to os.Create.")
	}
	rc, err := client.Bucket(bucket).Object(object).NewReader(ctx)
	if err != nil {
		return errors.Wrapf(err, "Failed to Object(%q).NewReader.", object)
	}
	defer rc.Close()

	if _, err = io.Copy(f, rc); err != nil {
		return errors.Wrap(err, "Failed to io.Copy.")
	}

	if err = f.Close(); err != nil {
		return errors.Wrap(err, "Failed to f.Close.")
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

	ctx, cancel := context.WithTimeout(ctx, GcpTimeout)
	defer cancel()

	query := &storage.Query{Prefix: prefix, Delimiter: delim}
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

func convertHeightStr(heiStr string) uint64 {
	idx := strings.Index(heiStr, "m")
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

func mkdirIfNotExist(destDir string) error {
	if _, err := os.Stat(destDir); os.IsNotExist(err) {
		if err := os.MkdirAll(destDir, 0744); err != nil {
			return errors.Wrapf(err, "Failed to create dir: %s", destDir)
		}
	}
	return nil
}
