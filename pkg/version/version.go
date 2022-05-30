// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package version

const (
	// ProtocolVersion defines Protocol version, starting from 1
	ProtocolVersion = 0x01
)

var (
	// PackageVersion gets version of code from git tag
	PackageVersion = "NoBuildInfo"
	// PackageCommitID gets latest commit id of code from git
	PackageCommitID = "NoBuildInfo"
	// GitStatus gets git tree status from git
	GitStatus = "NoBuildInfo"
	// GoVersion gets go version of package
	GoVersion = "NoBuildInfo"
	// BuildTime gets building time
	BuildTime = "NoBuildInfo"
)
