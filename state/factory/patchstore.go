// Copyright (c) 2021 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package factory

import (
	"encoding/csv"
	"encoding/hex"
	"io"
	"os"
	"strconv"

	"github.com/pkg/errors"
)

const (
	_PUT patchType = iota
	_DELETE
)

type (
	patchType uint8
	patch     struct {
		Type      patchType
		Namespace string
		Key       []byte
		Value     []byte
	}
	patchStore struct {
		patchs map[uint64][]*patch
	}
)

/**
 * patch format:
 *   height,type,namespace,key,value
 *     height: uint64, the height the record will be applied
 *     type: PUT or DELETE
 *     namespace: text string
 *     key: hex string
 *     value: hex string
 */
func newPatchStore(filepath string) (*patchStore, error) {
	store := &patchStore{
		patchs: map[uint64][]*patch{},
	}
	if filepath == "" {
		return store, nil
	}
	file, err := os.Open(filepath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open kvstore patch, %s", filepath)
	}
	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, errors.Wrap(err, "failed to read kvstore patch")
		}
		if len(record) < 4 {
			return nil, errors.Errorf("wrong format %+v", record)
		}
		height, err := strconv.ParseUint(record[0], 10, 64)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse height, %s", record[0])
		}
		if _, ok := store.patchs[height]; !ok {
			store.patchs[height] = []*patch{}
		}
		var t patchType
		var value []byte
		switch record[1] {
		case "PUT":
			t = _PUT
			if len(record) != 5 {
				return nil, errors.Errorf("wrong put format %+v", record)
			}
			value, err = hex.DecodeString(record[4])
			if err != nil {
				return nil, errors.Wrapf(err, "failed to parse value, %s", record[4])
			}
		case "DELETE":
			t = _DELETE
		default:
			return nil, errors.Errorf("invalid patch type, %s", record[1])
		}
		key, err := hex.DecodeString(record[3])
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse key, %s", record[3])
		}
		store.patchs[height] = append(store.patchs[height], &patch{
			Type:      t,
			Namespace: record[2],
			Key:       key,
			Value:     value,
		})
	}

	return store, nil
}

func (ps *patchStore) Get(height uint64) []*patch {
	if ps == nil {
		return nil
	}
	return ps.patchs[height]
}
