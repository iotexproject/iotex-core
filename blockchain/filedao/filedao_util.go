// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package filedao

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
)

func checkChainDBFiles(cfg config.DB) (*FileHeader, []string, error) {
	h, err := readFileHeader(cfg, FileAll)
	if err == ErrFileNotExist || err == ErrFileInvalid {
		return nil, nil, err
	}

	switch h.Version {
	case FileLegacyAuxiliary:
		// default chain db file is legacy format, but not master, the default file has been corrupted
		return h, nil, ErrFileInvalid
	case FileLegacyMaster, FileV2:
		return h, checkAuxFiles(cfg, FileV2), nil
	default:
		panic(fmt.Errorf("corrupted file version: %s", h.Version))
	}
}

func readFileHeader(cfg config.DB, fileType string) (*FileHeader, error) {
	size, exist := fileExists(cfg.DbPath)
	if !exist || size == 0 {
		// default chain db file does not exist
		return nil, ErrFileNotExist
	}

	ctx := context.Background()
	file := db.NewBoltDB(cfg)
	if err := file.Start(ctx); err != nil {
		// not a valid db file
		return nil, ErrFileInvalid
	}
	defer file.Stop(ctx)

	switch fileType {
	case FileLegacyMaster, FileLegacyAuxiliary:
		return ReadHeaderLegacy(file)
	case FileV2:
		if headerV2, err := ReadHeaderV2(file); err == nil {
			return headerV2, nil
		}
		return nil, ErrFileInvalid
	case FileAll:
		if header, err := ReadHeaderLegacy(file); err == nil {
			return header, nil
		}
		if headerV2, err := ReadHeaderV2(file); err == nil {
			return headerV2, nil
		}
		return nil, ErrFileInvalid
	default:
		panic(fmt.Errorf("unsupported check type %s", fileType))
	}
}

func fileExists(name string) (int64, bool) {
	info, err := os.Stat(name)
	if err != nil || info.IsDir() {
		return 0, false
	}
	return info.Size(), true
}

func checkAuxFiles(cfg config.DB, fileType string) []string {
	possible := possibleDBFiles(cfg.DbPath)

	var files []string
	for _, name := range possible {
		cfg.DbPath = name
		header, err := readFileHeader(cfg, fileType)
		if err == nil && header.Version == fileType {
			files = append(files, name)
		}
	}
	return files
}

func possibleDBFiles(fullname string) []string {
	file := path.Base(fullname)
	if file == "/" {
		return nil
	}

	dir := path.Dir(fullname)
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil
	}

	var possible []string
	for _, v := range files {
		if v.IsDir() {
			continue
		}
		if _, ok := isAuxFile(v.Name(), file); ok {
			possible = append(possible, dir+"/"+v.Name())
		}
	}
	return possible
}

// isAuxFile returns true if file is an auxiliary chain db filename, and its index
func isAuxFile(file, base string) (int, bool) {
	extB := path.Ext(base)
	ext := path.Ext(file)
	if extB != ext {
		return 0, false
	}

	base = strings.TrimSuffix(base, extB)
	file = strings.TrimSuffix(file, ext)
	file = strings.TrimPrefix(file, base)
	if len(file) != 9 || file[0] != '-' {
		return 0, false
	}

	index, err := strconv.Atoi(file[1:])
	if err != nil {
		return 0, false
	}
	return index, true
}

// kthAuxFileName returns k-th auxiliary chain db filename
func kthAuxFileName(file string, k int) string {
	ext := path.Ext(file)
	file = strings.TrimSuffix(file, ext)
	return file + fmt.Sprintf("-%08d", k) + ext
}

func hashKey(h hash.Hash256) []byte {
	return append(hashPrefix, h[:]...)
}

func getValueMustBe8Bytes(kvs db.KVStore, ns string, key []byte) ([]byte, error) {
	value, err := kvs.Get(ns, key)
	if err != nil {
		return nil, err
	}
	if len(value) != 8 {
		return nil, ErrDataCorruption
	}
	return value, nil
}
