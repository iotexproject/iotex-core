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

type patch struct {
	Namespace string
	Key       []byte
	Value     []byte
}

func loadPatchs(filepath string) (map[uint64][]*patch, error) {
	retval := map[uint64][]*patch{}
	if filepath == "" {
		return nil, nil
	}
	file, err := os.Open(filepath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open kvstore patch, %s", filepath)
	}
	reader := csv.NewReader(file)
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, errors.Wrap(err, "failed to read kvstore patch")
		}
		if len(record) != 4 {
			return nil, errors.Errorf("wrong format %+v", record)
		}
		height, err := strconv.ParseUint(record[0], 10, 64)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse height, %s", record[0])
		}
		if _, ok := retval[height]; !ok {
			retval[height] = []*patch{}
		}
		key, err := hex.DecodeString(record[2])
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse key, %s", record[2])
		}
		value, err := hex.DecodeString(record[3])
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse value, %s", record[3])
		}
		retval[height] = append(retval[height], &patch{
			Namespace: record[1],
			Key:       key,
			Value:     value,
		})
	}

	return retval, nil
}
