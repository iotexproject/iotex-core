// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package cli

import (
	"flag"
	"fmt"
	"os"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
)

// CLI defines the struct of command line interface
type CLI struct {
	bc blockchain.Blockchain
}

func (cli *CLI) printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  printchain                            # print all the blocks of the blockchain")
	fmt.Println("  createchain -address ADDRESS          # create a new blockchain with an address")
	fmt.Println("  getbalance -address ADDRESS           # get the balance of the address")
	fmt.Println("  send -from FROM -to TO -amount AMOUNT # send from one address to another")
}

func (cli *CLI) validateArgs() {
	if len(os.Args) < 2 {
		cli.printUsage()
		os.Exit(1)
	}
}

// Run processes the command line input
func (cli *CLI) Run() {
	cli.validateArgs()

	printChainCmd := flag.NewFlagSet("printchain", flag.ExitOnError)

	createChainCmd := flag.NewFlagSet("createchain", flag.ExitOnError)
	createChainAddress := createChainCmd.String("address", "", "Coinbase transaction output address")

	getBalanceCmd := flag.NewFlagSet("getbalance", flag.ExitOnError)
	getBalanceAddress := getBalanceCmd.String("address", "", "address")

	sendCmd := flag.NewFlagSet("send", flag.ExitOnError)
	sendCmdFrom := sendCmd.String("from", "", "send from address")
	sendCmdTo := sendCmd.String("to", "", "send to address")
	sendCmdAmount := sendCmd.Int("amount", 0, "send amount")

	switch os.Args[1] {
	case "printchain":
		printChainCmd.Parse(os.Args[2:])
	case "createchain":
		createChainCmd.Parse(os.Args[2:])
	case "getbalance":
		getBalanceCmd.Parse(os.Args[2:])
	case "send":
		sendCmd.Parse(os.Args[2:])
	default:
		cli.printUsage()
		os.Exit(1)
	}

	config, err := config.LoadConfig()
	if err != nil {
		os.Exit(1)
	}

	if printChainCmd.Parsed() {
		cli.printChain(config)
	}
	if createChainCmd.Parsed() {
		if *createChainAddress == "" {
			os.Exit(1)
		}
		cli.bc = blockchain.CreateBlockchain(config, nil)
		defer cli.bc.Stop()
	}
	if getBalanceCmd.Parsed() {
		cli.getBalance(*getBalanceAddress, config)
	}
	if sendCmd.Parsed() {
		if *sendCmdFrom == "" || *sendCmdTo == "" || *sendCmdAmount <= 0 {
			sendCmd.Usage()
			os.Exit(1)
		}
		cli.send(*sendCmdFrom, *sendCmdTo, uint64(*sendCmdAmount), config)
	}
}
