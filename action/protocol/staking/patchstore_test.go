// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"math/big"
	"os"
	"path/filepath"
	"testing"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/stretchr/testify/require"
)

func TestInvalidDirectory(t *testing.T) {
	require := require.New(t)
	dir := filepath.Join(t.TempDir(), "invalid")
	_, err := os.Create(dir)
	require.NoError(err)
	_, _, _, _, err = NewPatchStore(dir).Read(0)
	require.ErrorContains(err, "not a directory")
}

func TestInvalidDirectory2(t *testing.T) {
	require := require.New(t)
	dir := t.TempDir()
	require.NoError(os.Remove(dir))
	_, err := os.Stat(dir)
	require.ErrorIs(err, os.ErrNotExist)
	_, _, _, _, err = NewPatchStore(dir).Read(0)
	require.ErrorContains(err, "no such file or directory")
}

func TestCorruptedData(t *testing.T) {
	// TODO: add test for corrupted data
}

func TestWriteAndRead(t *testing.T) {
	require := require.New(t)
	dir := t.TempDir()
	patch := NewPatchStore(dir)
	listByName := CandidateList{
		&Candidate{
			Owner:              identityset.Address(1),
			Operator:           identityset.Address(7),
			Reward:             identityset.Address(1),
			Name:               "name0",
			Votes:              big.NewInt(2),
			SelfStakeBucketIdx: 1,
			SelfStake:          unit.ConvertIotxToRau(1200000),
		},
		&Candidate{
			Owner:              identityset.Address(1),
			Operator:           identityset.Address(7),
			Reward:             identityset.Address(1),
			Name:               "name1",
			Votes:              big.NewInt(2),
			SelfStakeBucketIdx: 1,
			SelfStake:          unit.ConvertIotxToRau(1200000),
		},
	}
	names := map[string]string{
		identityset.Address(1).String(): "name1",
	}
	listByOperator := CandidateList{
		&Candidate{
			Owner:              identityset.Address(1),
			Operator:           identityset.Address(6),
			Reward:             identityset.Address(1),
			Name:               "name",
			Votes:              big.NewInt(2),
			SelfStakeBucketIdx: 1,
			SelfStake:          unit.ConvertIotxToRau(1200000),
		},
		&Candidate{
			Owner:              identityset.Address(1),
			Operator:           identityset.Address(7),
			Reward:             identityset.Address(1),
			Name:               "name",
			Votes:              big.NewInt(2),
			SelfStakeBucketIdx: 1,
			SelfStake:          unit.ConvertIotxToRau(1200000),
		},
	}
	operators := map[string]address.Address{
		identityset.Address(1).String(): identityset.Address(7),
	}
	require.ErrorIs(patch.Write(2, nil, nil, nil, nil), ErrNilParameters)
	require.ErrorIs(patch.Write(2, nil, listByOperator, nil, nil), ErrNilParameters)
	require.ErrorIs(patch.Write(2, listByName, nil, nil, nil), ErrNilParameters)
	require.NoError(patch.Write(2, listByName, listByOperator, nil, nil))
	listByNameCopy, listByOperatorCopy, namesCopy, operatorsCopy, err := patch.Read(2)
	require.NoError(err)
	require.Equal(2, len(listByNameCopy))
	require.Equal(2, len(listByOperatorCopy))
	require.Equal(0, len(namesCopy))
	require.Equal(0, len(operatorsCopy))
	nameMap := map[string]bool{
		listByNameCopy[0].Name: true,
		listByNameCopy[1].Name: true,
	}
	require.Equal(2, len(nameMap))
	require.True(nameMap["name0"])
	require.True(nameMap["name1"])
	operatorMap := map[string]bool{
		listByOperatorCopy[0].Operator.String(): true,
		listByOperatorCopy[1].Operator.String(): true,
	}
	require.Equal(2, len(operatorMap))
	require.True(operatorMap[identityset.Address(6).String()])
	require.True(operatorMap[identityset.Address(7).String()])
	require.NoError(patch.Write(2, listByName, listByOperator, names, operators))
	listByNameCopy, listByOperatorCopy, namesCopy, operatorsCopy, err = patch.Read(2)
	require.NoError(err)
	require.Equal(2, len(listByNameCopy))
	require.Equal(2, len(listByOperatorCopy))
	require.Equal(1, len(namesCopy))
	require.Equal(1, len(operatorsCopy))
	require.Equal("name1", namesCopy[identityset.Address(1).String()])
	require.Equal(identityset.Address(7), operatorsCopy[identityset.Address(1).String()])
}
