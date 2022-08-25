// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package filedao

import (
	"context"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"syscall"

	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/db"
)

func readFileHeader(filename, fileType string) (*FileHeader, error) {
	if err := fileExists(filename); err != nil {
		return nil, err
	}

	file := db.NewBoltDB(db.Config{DbPath: filename, NumRetries: 3})
	ctx := context.Background()
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

func fileExists(name string) error {
	info, err := os.Stat(name)
	if err != nil || info.IsDir() || info.Size() == 0 {
		return ErrFileNotExist
	}
	err = syscall.Access(name, syscall.O_RDWR)
	if err != nil {
		return ErrFileCantAccess
	}
	return nil
}

func checkAuxFiles(filename, fileType string) (uint64, []string) {
	file := path.Base(filename)
	if file == "/" {
		return 0, nil
	}

	dir := path.Dir(filename)
	files, err := os.ReadDir(dir)
	if err != nil {
		return 0, nil
	}

	var (
		top      uint64
		possible []string
	)
	for _, v := range files {
		if v.IsDir() {
			continue
		}
		index, ok := isAuxFile(v.Name(), file)
		if !ok {
			continue
		}
		name := dir + "/" + v.Name()
		header, err := readFileHeader(name, fileType)
		if err == nil && header.Version == fileType {
			possible = append(possible, name)
			if index > top {
				top = index
			}
		}
	}
	return top, possible
}

// isAuxFile returns true if file is an auxiliary chain db filename, and its index
func isAuxFile(file, base string) (uint64, bool) {
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
	if err != nil || index < 0 {
		return 0, false
	}
	return uint64(index), true
}

// kthAuxFileName returns k-th auxiliary chain db filename
func kthAuxFileName(file string, k uint64) string {
	ext := path.Ext(file)
	file = strings.TrimSuffix(file, ext)
	return file + fmt.Sprintf("-%08d", k) + ext
}

func hashKey(h hash.Hash256) []byte {
	return append(_hashPrefix, h[:]...)
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
