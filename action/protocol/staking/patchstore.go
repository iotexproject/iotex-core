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

	"github.com/iotexproject/iotex-address/address"
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

func (patch *PatchStore) pathOf(height uint64) string {
	return filepath.Join(patch.dir, fmt.Sprintf("%d.patch", height))
}

func (patch *PatchStore) read(reader *csv.Reader) (CandidateList, error) {
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
func (store *PatchStore) Read(height uint64) (
	CandidateList,
	CandidateList,
	map[string]string,
	map[string]address.Address,
	error,
) {
	file, err := os.Open(store.pathOf(height))
	if err != nil {
		return nil, nil, nil, nil, err
	}
	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	listByName, err := store.read(reader)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	listByOperator, err := store.read(reader)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	names := map[string]string{}
	operators := map[string]address.Address{}
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if len(record) != 3 {
			return nil, nil, nil, nil, errors.Errorf("invalid record %+v", record)
		}
		if _, err := address.FromString(record[0]); err != nil {
			return nil, nil, nil, nil, errors.Wrapf(err, "failed to cast owner address %s", record[0])
		}
		switch record[1] {
		case _name:
			names[record[0]] = record[2]
		case _operator:
			operator, err := address.FromString(record[2])
			if err != nil {
				return nil, nil, nil, nil, errors.Wrapf(err, "failed to cast operator address %s", record[2])
			}
			operators[record[0]] = operator
		default:
			return nil, nil, nil, nil, errors.Errorf("invalid type %s", record[1])
		}
	}

	return listByName, listByOperator, names, operators, nil
}

// Write writes CandidateList by name and CandidateList by operator into store
func (store *PatchStore) Write(
	height uint64,
	listByName, listByOperator CandidateList,
	names map[string]string,
	operators map[string]address.Address,
) (err error) {
	if listByName == nil || listByOperator == nil {
		return errors.Wrap(ErrNilParameters, "invalid parameters")
	}
	bytesByName, err := listByName.Serialize()
	if err != nil {
		return errors.Wrap(err, "failed to serialize candidate list by name")
	}
	bytesByOperator, err := listByOperator.Serialize()
	if err != nil {
		return errors.Wrap(err, "failed to serialize candidate list by operator")
	}
	records := [][]string{
		{hex.EncodeToString(bytesByName)},
		{hex.EncodeToString(bytesByOperator)},
	}
	for owner, name := range names {
		records = append(records, []string{owner, _name, name})
	}
	for owner, operator := range operators {
		records = append(records, []string{owner, _operator, operator.String()})
	}
	file, err := os.Create(store.pathOf(height))
	if err != nil {
		return err
	}
	defer func() {
		fileCloseErr := file.Close()
		if fileCloseErr != nil && err == nil {
			err = fileCloseErr
		}
	}()

	return csv.NewWriter(file).WriteAll(records)
}
