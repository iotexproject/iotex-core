// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
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
