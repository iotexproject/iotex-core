// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package fileutil

import (
	"os"
	"path"
	"strings"

	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/pkg/log"
)

// FileExists checks if a file or a directory already exists
func FileExists(path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}
	return true
}

// GetFileAbsPath returns the absolute path of a file in blockchain directory
func GetFileAbsPath(filename string) string {
	pwd, err := os.Getwd()
	if err != nil {
		log.L().Fatal("Fail to get absolute path of genesis actions yaml file.", zap.Error(err))
	}
	firstIndex := strings.LastIndex(pwd, "iotex-core")
	index := strings.Index(pwd[firstIndex:], "/")
	if index == -1 {
		return path.Join(pwd, "blockchain", filename)
	}
	return path.Join(pwd[0:firstIndex+index], "blockchain", filename)
}
