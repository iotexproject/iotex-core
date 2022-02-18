// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package testutil

import (
	"os"
	"testing"

	"github.com/iotexproject/iotex-core/pkg/util/fileutil"
)

// PathOfTempFile returns path of a new temporary file
func PathOfTempFile(dirName string) (string, error) {
	tempFile, err := os.CreateTemp(os.TempDir(), dirName)
	if err != nil {
		return "", err
	}
	return tempFile.Name(), tempFile.Close()
}

// CleanupPath detects the existence of test DB file and removes it if found
func CleanupPath(t *testing.T, path string) {
	if fileutil.FileExists(path) && os.RemoveAll(path) != nil {
		t.Error("Fail to remove testDB file")
	}
}

// CleanupPathV2 detects the existence of test DB file and removes it if found
// Todo: replace CleanupPath with CleanupPathV2 for compatibility
func CleanupPathV2(path string) {
	if fileutil.FileExists(path) && os.RemoveAll(path) != nil {
		panic("Fail to remove testDB file")
	}
}
