// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

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
	"github.com/iotexproject/iotex-core/ioctl/util"
)

// Multi-language support
var (
	infoCmdUses = map[ioctl.Language]string{
		ioctl.English: "info [ALIAS|ADDRESS]",
		ioctl.Chinese: "info [别名|地址]",
	}
	infoCmdShorts = map[ioctl.Language]string{
		ioctl.English: "Display an account's information",
		ioctl.Chinese: "显示账号信息",
	}
	invalidAccountBalance = map[ioctl.Language]string{
		ioctl.English: "invalid account balance",
		ioctl.Chinese: "无效的账户余额",
	}
	failToGetAccountMeta = map[ioctl.Language]string{
		ioctl.English: "failed to get account meta",
		ioctl.Chinese: "获取账户信息失败",
	}
)

// NewAccountInfo represents the account info command
func NewAccountInfo(client ioctl.Client) *cobra.Command {
	use, _ := client.SelectTranslation(infoCmdUses)
	short, _ := client.SelectTranslation(infoCmdShorts)
	failToGetAddress, _ := client.SelectTranslation(failToGetAddress)
	failToConvertStringIntoAddress, _ := client.SelectTranslation(failToConvertStringIntoAddress)
	invalidAccountBalance, _ := client.SelectTranslation(invalidAccountBalance)
	failToGetAccountMeta, _ := client.SelectTranslation(failToGetAccountMeta)

	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			addr := args[0]
			if addr != address.StakingBucketPoolAddr && addr != address.RewardingPoolAddr {
				var err error
				addr, err = client.GetAddress(addr)
				if err != nil {
					return errors.Wrap(err, failToGetAddress)
				}
			}

			accountMeta, err := Meta(client, addr)
			if err != nil {
				return errors.Wrap(err, failToGetAccountMeta)
			}
			balance, ok := new(big.Int).SetString(accountMeta.Balance, 10)
			if !ok {
				return errors.New(invalidAccountBalance)
			}
			ethAddr, err := address.FromString(addr)
			if err != nil {
				return errors.Wrap(err, failToConvertStringIntoAddress)
			}

			message := infoMessage{
				Address:          addr,
				EthAddress:       ethAddr.Hex(),
				Balance:          util.RauToString(balance, util.IotxDecimalNum),
				Nonce:            int(accountMeta.Nonce),
				PendingNonce:     int(accountMeta.PendingNonce),
				NumActions:       int(accountMeta.NumActions),
				IsContract:       accountMeta.IsContract,
				ContractByteCode: hex.EncodeToString(accountMeta.ContractByteCode),
			}
			client.PrintInfo(message.String())
			return nil
		},
	}
}

type infoMessage struct {
	Address          string `json:"address"`
	EthAddress       string `json:"ethAddress"`
	Balance          string `json:"balance"`
	Nonce            int    `json:"nonce"`
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
