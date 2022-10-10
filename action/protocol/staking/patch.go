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
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

// PatchStore is the patch store of staking protocol
type PatchStore struct {
	dir string
}

// NewPatchStore creates a new staking patch store
func NewPatchStore(dir string) *PatchStore {
	return &PatchStore{dir: dir}
}

func (patch *PatchStore) pathOf(height uint64) string {
	return filepath.Join(patch.dir, fmt.Sprintf("%d.patch", height))
}

func (patch *PatchStore) read(reader *csv.Reader) (CandidateList, error) {
	record, err := reader.Read()
	if err != nil {
		return nil, err
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
func (patch *PatchStore) Read(height uint64) (CandidateList, CandidateList, error) {
	file, err := os.Open(patch.pathOf(height))
	if err != nil {
		return nil, nil, err
	}
	reader := csv.NewReader(file)
	reader.FieldsPerRecord = 1
	listByName, err := patch.read(reader)
	if err != nil {
		return nil, nil, err
	}
	listByOperator, err := patch.read(reader)
	if err != nil {
		return nil, nil, err
	}

	return listByName, listByOperator, nil
}

// Write writes CandidateList by name and CandidateList by operator into store
func (patch *PatchStore) Write(height uint64, listByName, listByOperator CandidateList) (err error) {
	if listByName == nil || listByOperator == nil {
		return errors.Wrap(ErrNilParameters, "invalid candidate lists")
	}
	bytesByName, err := listByName.Serialize()
	if err != nil {
		return errors.Wrap(err, "failed to serialize candidate list by name")
	}
	bytesByOperator, err := listByOperator.Serialize()
	if err != nil {
		return errors.Wrap(err, "failed to serialize candidate list by operator")
	}
	file, err := os.Create(patch.pathOf(height))
	if err != nil {
		return err
	}
	defer func() {
		fileCloseErr := file.Close()
		if fileCloseErr != nil && err == nil {
			err = fileCloseErr
		}
	}()

	return csv.NewWriter(file).WriteAll(
		[][]string{
			{hex.EncodeToString(bytesByName)},
			{hex.EncodeToString(bytesByOperator)},
		},
	)
}
