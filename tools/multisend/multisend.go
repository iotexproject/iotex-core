// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/pkg/util/addrutil"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "multisend 'JSON_DATA'",
	Short: "multisend bytecode generator",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		output, err := multiSend(args)
		if err == nil {
			fmt.Println(output)
		}
		return err
	},
}

var abiJSON = `[{"constant":false,"inputs":[{"name":"recipients","type":"address[]"},
{"name":"amounts","type":"uint256[]"},{"name":"payload","type":"string"}],
"name":"multiSend","outputs":[],"payable":true,"stateMutability":"payable","type":"function"},
{"anonymous":false,"inputs":[{"indexed":false,"name":"recipient","type":"address"},
{"indexed":false,"name":"amount","type":"uint256"}],"name":"Transfer","type":"event"},
{"anonymous":false,"inputs":[{"indexed":false,"name":"refund","type":"uint256"}],
"name":"Refund","type":"event"},{"anonymous":false,
"inputs":[{"indexed":false,"name":"payload","type":"string"}],"name":"Payload","type":"event"}]`
var abiFunc = "multiSend"

type targets struct {
	Targets []target `json:"targets"`
	Payload string   `json:"payload"`
}
type target struct {
	Recipient string `json:"recipient"`
	Amount    string `json:"amount"`
}

func multiSend(args []string) (string, error) {
	var targetSet targets
	if err := json.Unmarshal([]byte(args[0]), &targetSet); err != nil {
		return "", err
	}
	recipients := make([]common.Address, 0)
	amounts := make([]*big.Int, 0)
	for _, target := range targetSet.Targets {
		recipient, err := addrutil.IoAddrToEvmAddr(target.Recipient)
		if err != nil {
			return "", err
		}
		recipients = append(recipients, recipient)
		amount, ok := big.NewInt(0).SetString(target.Amount, 10)
		if !ok {
			return "", fmt.Errorf("failed to convert string to big int")
		}
		amounts = append(amounts, amount)
	}
	reader := strings.NewReader(abiJSON)
	multisendABI, err := abi.JSON(reader)
	if err != nil {
		return "", err
	}
	bytecode, err := multisendABI.Pack(abiFunc, recipients, amounts, targetSet.Payload)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(bytecode), nil
}
