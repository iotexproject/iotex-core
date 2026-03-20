// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package account

import (
	"fmt"
	"math/big"

	"github.com/spf13/cobra"

	ioAddress "github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
	"github.com/iotexproject/iotex-core/v2/ioctl/util"
)

// Multi-language support
var (
	_balanceCmdUses = map[config.Language]string{
		config.English: "balance [ALIAS|ADDRESS] [ALIAS|ADDRESS] ...",
		config.Chinese: "balance [别名|地址] [别名|地址] ...",
	}
	_balanceCmdShorts = map[config.Language]string{
		config.English: "Get balance of an account",
		config.Chinese: "查询账号余额",
	}
)

// accountBalanceCmd represents the account balance command
var accountBalanceCmd = &cobra.Command{
	Use:   config.TranslateInLang(_balanceCmdUses, config.UILanguage),
	Short: config.TranslateInLang(_balanceCmdShorts, config.UILanguage),
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		if len(args) == 0 {
			args = []string{""}
		}
		// Single address: original behavior
		if len(args) == 1 {
			msg, err := getBalance(args[0])
			if err != nil {
				return output.PrintError(err)
			}
			fmt.Println(msg.String())
			return nil
		}
		// Batch: collect all balances
		var results []balanceMessage
		for _, arg := range args {
			msg, err := getBalance(arg)
			if err != nil {
				return output.PrintError(err)
			}
			results = append(results, *msg)
		}
		if output.Format != "" {
			batchMsg := batchBalanceMessage{Balances: results}
			fmt.Println(output.FormatString(output.Result, &batchMsg))
		} else {
			for _, m := range results {
				fmt.Println(m.String())
			}
		}
		return nil
	},
}

type batchBalanceMessage struct {
	Balances []balanceMessage `json:"balances"`
}

func (m *batchBalanceMessage) String() string {
	return output.FormatString(output.Result, m)
}

type balanceMessage struct {
	Address string `json:"address"`
	Balance string `json:"balance"`
}

func (m *balanceMessage) String() string {
	if output.Format == "" {
		return fmt.Sprintf("%s: %s IOTX", m.Address, m.Balance)
	}
	return output.FormatString(output.Result, m)
}

// getBalance resolves address and fetches balance, returning a balanceMessage
func getBalance(arg string) (*balanceMessage, error) {
	addr := arg
	if arg != ioAddress.StakingBucketPoolAddr && arg != ioAddress.RewardingPoolAddr {
		var err error
		addr, err = util.GetAddress(arg)
		if err != nil {
			return nil, output.NewError(output.AddressError, "", err)
		}
	}
	accountMeta, err := GetAccountMeta(addr)
	if err != nil {
		return nil, output.NewError(0, "", err)
	}
	balance, ok := new(big.Int).SetString(accountMeta.Balance, 10)
	if !ok {
		return nil, output.NewError(output.ConvertError, "failed to convert balance", nil)
	}
	return &balanceMessage{
		Address: addr,
		Balance: util.RauToString(balance, util.IotxDecimalNum),
	}, nil
}
