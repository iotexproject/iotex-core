// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package compress

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompression(t *testing.T) {
	data := []byte("11111111111111111111111111111111111111110000000000000000000000000000000000000000")
	cd, err := Compress(data)
	require.NoError(t, err)
	dcd, err := Decompress(cd)
	require.NoError(t, err)
	assert.Equal(t, data, dcd)
}
