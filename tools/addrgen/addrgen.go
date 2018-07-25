// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

// This is a testing tool to generate iotex addresses
// To use, run "make build" and " ./bin/addrgen"
package main

import "github.com/iotexproject/iotex-core/tools/addrgen/internal/cmd"

func main() {
	cmd.Execute()
}
