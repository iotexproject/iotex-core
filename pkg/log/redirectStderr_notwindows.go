// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

//+build !windows

package log

import (
	"log"
	"os"
	"syscall"
)

// redirectStderr to the file passed in
func redirectStderr(f *os.File) error {
	err := syscall.Dup2(int(f.Fd()), 2)
	if err != nil {
		log.Fatalf("Failed to redirect stderr to file: %v", err)
		return err
	}
	return nil
}
