package main

// This example demos how to convert an Ethereum address to an IoTeX address.

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/iotex-address/address"
)

func main() {
	ethAddr := common.HexToAddress("0xcb816E0491Ce3b66f33304473468698Aae97Cc0a")
	pkHash := ethAddr.Bytes()
	ioAddr, _ := address.FromBytes(pkHash)
	ioAddrStr := ioAddr.String()
	fmt.Println(ioAddrStr)
}