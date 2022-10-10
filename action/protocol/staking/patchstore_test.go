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

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/test/identityset"
)

func TestInvalidDirectory(t *testing.T) {
	require := require.New(t)
	dir := filepath.Join(t.TempDir(), "invalid")
	_, err := os.Create(dir)
	require.NoError(err)
	_, _, _, err = NewPatchStore(dir).Read(0)
	require.ErrorContains(err, "not a directory")
}

func TestInvalidDirectory2(t *testing.T) {
	require := require.New(t)
	dir := t.TempDir()
	require.NoError(os.Remove(dir))
	_, err := os.Stat(dir)
	require.ErrorIs(err, os.ErrNotExist)
	_, _, _, err = NewPatchStore(dir).Read(0)
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
			Votes:              big.NewInt(1),
			SelfStakeBucketIdx: 2,
			SelfStake:          unit.ConvertIotxToRau(1200000),
		},
		&Candidate{
			Owner:              identityset.Address(3),
			Operator:           identityset.Address(5),
			Reward:             identityset.Address(9),
			Name:               "name1",
			Votes:              big.NewInt(2),
			SelfStakeBucketIdx: 3,
			SelfStake:          unit.ConvertIotxToRau(200000),
		},
	}
	listByOperator := CandidateList{
		&Candidate{
			Owner:              identityset.Address(8),
			Operator:           identityset.Address(6),
			Reward:             identityset.Address(2),
			Name:               "operator",
			Votes:              big.NewInt(3),
			SelfStakeBucketIdx: 5,
			SelfStake:          unit.ConvertIotxToRau(1200000),
		},
		&Candidate{
			Owner:              identityset.Address(3),
			Operator:           identityset.Address(4),
			Reward:             identityset.Address(5),
			Name:               "operator1",
			Votes:              big.NewInt(4),
			SelfStakeBucketIdx: 6,
			SelfStake:          unit.ConvertIotxToRau(1800000),
		},
	}
	listByOwner := CandidateList{
		&Candidate{
			Owner:              identityset.Address(2),
			Operator:           identityset.Address(8),
			Reward:             identityset.Address(6),
			Name:               "owner",
			Votes:              big.NewInt(6),
			SelfStakeBucketIdx: 9,
			SelfStake:          unit.ConvertIotxToRau(1100000),
		},
		&Candidate{
			Owner:              identityset.Address(3),
			Operator:           identityset.Address(9),
			Reward:             identityset.Address(7),
			Name:               "owner1",
			Votes:              big.NewInt(7),
			SelfStakeBucketIdx: 10,
			SelfStake:          unit.ConvertIotxToRau(2400000),
		},
		&Candidate{
			Owner:              identityset.Address(7),
			Operator:           identityset.Address(3),
			Reward:             identityset.Address(1),
			Name:               "owner2",
			Votes:              big.NewInt(8),
			SelfStakeBucketIdx: 5,
			SelfStake:          unit.ConvertIotxToRau(1200000),
		},
	}
	require.ErrorIs(patch.Write(2, nil, nil, nil), ErrNilParameters)
	require.ErrorIs(patch.Write(2, nil, listByOperator, nil), ErrNilParameters)
	require.ErrorIs(patch.Write(2, nil, nil, listByOwner), ErrNilParameters)
	require.NoError(patch.Write(2, listByName, listByOperator, listByOwner))
	listByNameCopy, listByOperatorCopy, listByOwnerCopy, err := patch.Read(1)
	require.ErrorIs(err, os.ErrNotExist)
	listByNameCopy, listByOperatorCopy, listByOwnerCopy, err = patch.Read(2)
	require.NoError(err)
	require.Equal(listByName, listByNameCopy)
	require.Equal(listByOperator, listByOperatorCopy)
	require.Equal(listByOwner, listByOwnerCopy)
}
