// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package account

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math/big"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

// Multi-language support
var (
	_infoCmdUses = map[config.Language]string{
		config.English: "info [ALIAS|ADDRESS]",
		config.Chinese: "info [别名|地址]",
	}
	_infoCmdShorts = map[config.Language]string{
		config.English: "Display an account's information",
		config.Chinese: "显示账号信息",
	}
	_invalidAccountBalance = map[config.Language]string{
		config.English: "invalid account balance",
		config.Chinese: "无效的账户余额",
	}
	_failToGetAccountMeta = map[config.Language]string{
		config.English: "failed to get account meta",
		config.Chinese: "获取账户信息失败",
	}
)

// NewAccountInfo represents the account info command
func NewAccountInfo(client ioctl.Client) *cobra.Command {
	use, _ := client.SelectTranslation(_infoCmdUses)
	short, _ := client.SelectTranslation(_infoCmdShorts)
	failToGetAddress, _ := client.SelectTranslation(_failToGetAddress)
	failToConvertStringIntoAddress, _ := client.SelectTranslation(_failToConvertStringIntoAddress)
	_invalidAccountBalance, _ := client.SelectTranslation(_invalidAccountBalance)
	_failToGetAccountMeta, _ := client.SelectTranslation(_failToGetAccountMeta)

	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			addr := args[0]
			if addr != address.StakingBucketPoolAddr && addr != address.RewardingPoolAddr {
				var err error
				addr, err = client.AddressWithDefaultIfNotExist(addr)
				if err != nil {
					return errors.Wrap(err, failToGetAddress)
				}
			}

			accountMeta, err := Meta(client, addr)
			if err != nil {
				return errors.Wrap(err, _failToGetAccountMeta)
			}
			balance, ok := new(big.Int).SetString(accountMeta.Balance, 10)
			if !ok {
				return errors.New(_invalidAccountBalance)
			}
			ethAddr, err := address.FromString(addr)
			if err != nil {
				return errors.Wrap(err, failToConvertStringIntoAddress)
			}

			message := infoMessage{
				Address:          addr,
				EthAddress:       ethAddr.Hex(),
				Balance:          util.RauToString(balance, util.IotxDecimalNum),
				PendingNonce:     int(accountMeta.PendingNonce),
				NumActions:       int(accountMeta.NumActions),
				IsContract:       accountMeta.IsContract,
				ContractByteCode: hex.EncodeToString(accountMeta.ContractByteCode),
			}
			cmd.Println(message.String())
			return nil
		},
	}
}

type infoMessage struct {
	Address          string `json:"address"`
	EthAddress       string `json:"ethAddress"`
	Balance          string `json:"balance"`
	PendingNonce     int    `json:"pendingNonce"`
	NumActions       int    `json:"numActions"`
	IsContract       bool   `json:"isContract"`
	ContractByteCode string `json:"contractByteCode"`
}

func (m *infoMessage) String() string {
	byteAsJSON, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		log.Panic(err)
	}
	return fmt.Sprint(string(byteAsJSON))
}
