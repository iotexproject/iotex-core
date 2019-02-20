// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package version

import (
	"flag"
	"fmt"
)

const (
	// ProtocolVersion defines Protocol version, starting from 1
	ProtocolVersion = 0x01
)

var (
	// SourceVersion gets version of code from git tag.
	SourceVersion string
	// SourceCommitID gets latest commit id of code from git
	SourceCommitID string
)

func init() {
	flag.StringVar(&SourceVersion, "version", "NoVersionInfo", "source version from git")
	flag.StringVar(&SourceCommitID, "commitID", "NoVersionInfo", "latest commit id from git")
	flag.Parse()

	fmt.Println(SourceCommitID, SourceVersion)
}
