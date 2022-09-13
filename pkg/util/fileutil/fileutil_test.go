// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package fileutil

import (
	"testing"

	// "github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestFileExists(t *testing.T) {
	t.Run("valid file path", func(t *testing.T) {
		isFileExists := FileExists("./fileutil.go")
		require.True(t, isFileExists)
	})

	t.Run("invalid file path", func(t *testing.T) {
		isFileExists := FileExists("")
		require.False(t, isFileExists)
	})
}
