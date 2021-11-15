// Copyright (c) 2021 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package testutil

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFile(t *testing.T) {
	assert := assert.New(t)
	tmpPath, err := PathOfTempFile("iotx")
	assert.NoError(err)
	_, err = os.Stat(tmpPath)
	assert.NoError(err)
	defer CleanupPath(t, tmpPath)
}
