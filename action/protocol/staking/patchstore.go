// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

const (
	_name     = "name"
	_operator = "operator"
)

// PatchStore is the patch store of staking protocol
type PatchStore struct {
	dir string
}

// NewPatchStore creates a new staking patch store
func NewPatchStore(dir string) *PatchStore {
	return &PatchStore{dir: dir}
}

func (store *PatchStore) pathOf(height uint64) string {
	return filepath.Join(store.dir, fmt.Sprintf("%d.patch", height))
}

func (store *PatchStore) read(reader *csv.Reader) (CandidateList, error) {
	record, err := reader.Read()
	if err != nil {
		return nil, err
	}
	if len(record) != 1 {
		return nil, errors.Errorf("invalid record %+v", record)
	}
	data, err := hex.DecodeString(record[0])
	if err != nil {
		return nil, err
	}
	var list CandidateList
	if err := list.Deserialize(data); err != nil {
		return nil, err
	}
	return list, nil
}

// Read reads CandidateList by name and CandidateList by operator of given height
func (store *PatchStore) Read(height uint64) (CandidateList, CandidateList, CandidateList, error) {
	file, err := os.Open(store.pathOf(height))
	if err != nil {
		return nil, nil, nil, err
	}
	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	listByName, err := store.read(reader)
	if err != nil {
		return nil, nil, nil, err
	}
	listByOperator, err := store.read(reader)
	if err != nil {
		return nil, nil, nil, err
	}
	listByOwner, err := store.read(reader)
	if err != nil && err != io.EOF {
		// io.EOF indicates an empty owner list
		return nil, nil, nil, err
	}
	return listByName, listByOperator, listByOwner, nil
}
