// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/iotexproject/iotex-address/address"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

// Multi-language support
var (
	infoCmdUses = map[config.Language]string{
		config.English: "info [ALIAS|ADDRESS]",
		config.Chinese: "info [别名|地址]",
	}
	infoCmdShorts = map[config.Language]string{
		config.English: "Display an account's information",
		config.Chinese: "显示账号信息",
	}
)

// NewAccountInfo represents the account info command
func NewAccountInfo(client ioctl.Client) *cobra.Command {
	use, _ := client.SelectTranslation(infoCmdUses)
	short, _ := client.SelectTranslation(infoCmdShorts)

	cmd := &cobra.Command{
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
					return output.NewError(output.AddressError, "failed to get address", err)
				}
			}
			accountMeta, err := GetAccountMeta(addr, client)
			if err != nil {
				return output.NewError(output.APIError, "failed to get account meta", err)
			}
			balance := big.NewInt(0)
			if accountMeta.Balance != "" {
				var ok bool
				balance, ok = new(big.Int).SetString(accountMeta.Balance, 10)
				if !ok {
					return output.NewError(output.ConvertError, "failed to set account balance", err)
				}
			}
			ethAddr, err := address.FromString(addr)
			if err != nil {
				return output.NewError(output.ConvertError, "failed to convert address to eth address", err)
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

			fmt.Println(message.String())
			return nil
		},
	}

	return cmd
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
	if output.Format == "" {
		return fmt.Sprintf("%s:\n%s", m.Address, output.JSONString(m))
	}
	return output.FormatString(output.Result, m)
}
