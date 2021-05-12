// Copyright (c) 2021 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDB_SplitDBSize(t *testing.T) {
	var db = Config{SplitDBSizeMB: uint64(1)}
	var expected = uint64(1 * 1024 * 1024)
	require.Equal(t, expected, db.SplitDBSize())
}
