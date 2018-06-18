// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

// This is a testing tool to generate iotex addresses
// To use, run "make build" and " ./bin/addrgen"
package main

import (
	"flag"
	"fmt"

	"github.com/iotexproject/iotex-core-internal/iotxaddress"
)

func main() {
	var addrNum int
	flag.IntVar(&addrNum, "number", 10, "number of addresses to be generated")
	flag.Parse()

	for i := 0; i < addrNum; i++ {
		addr, err := iotxaddress.NewAddress(iotxaddress.IsTestnet, iotxaddress.ChainID)
		if err != nil {
			panic(err)
		}
		fmt.Printf("Public Key: %x\n", addr.PublicKey)
		fmt.Printf("Private Key: %x\n", addr.PrivateKey)
		fmt.Printf("Raw Address: %s\n", addr.RawAddress)
		fmt.Println()
	}
}
