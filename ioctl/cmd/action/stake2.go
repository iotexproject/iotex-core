// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/validator"
)

// Multi-language support
var (
	_stake2CmdShorts = map[config.Language]string{
		config.English: "Support native staking of IoTeX blockchain",
		config.Chinese: "支持来自ioctl的本地质押",
	}
	_stake2FlagEndpointUsages = map[config.Language]string{
		config.English: "set endpoint for once",
		config.Chinese: "一次设置所有端点",
	}
	_stake2FlagInsecureUsages = map[config.Language]string{
		config.English: "insecure connection for once (default false)",
		config.Chinese: "一次不安全的连接（默认为false)",
	}
	_stake2FlagAutoStakeUsages = map[config.Language]string{
		config.English: "auto-stake boost: the voting power will not decrease",
		config.Chinese: "自动质押，权重不会衰减",
	}
)

var _stake2AutoStake bool

//Stake2Cmd represent stake2 command
var Stake2Cmd = &cobra.Command{
	Use:   "stake2",
	Short: config.TranslateInLang(_stake2CmdShorts, config.UILanguage),
}

func init() {
	Stake2Cmd.AddCommand(_stake2CreateCmd)
	Stake2Cmd.AddCommand(_stake2RenewCmd)
	Stake2Cmd.AddCommand(_stake2WithdrawCmd)
	Stake2Cmd.AddCommand(_stake2UpdateCmd)
	Stake2Cmd.AddCommand(_stake2AddCmd)
	Stake2Cmd.AddCommand(_stake2TransferCmd)
	Stake2Cmd.AddCommand(_stake2ReclaimCmd)
	Stake2Cmd.AddCommand(_stake2ReleaseCmd)
	Stake2Cmd.AddCommand(_stake2RegisterCmd)
	Stake2Cmd.AddCommand(_stake2ChangeCmd)
	Stake2Cmd.PersistentFlags().StringVar(&config.ReadConfig.Endpoint, "endpoint", config.ReadConfig.Endpoint, config.TranslateInLang(_stake2FlagEndpointUsages, config.UILanguage))
	Stake2Cmd.PersistentFlags().BoolVar(&config.Insecure, "insecure", config.Insecure, config.TranslateInLang(_stake2FlagInsecureUsages, config.UILanguage))
}

func parseStakeDuration(stakeDurationString string) (*big.Int, error) {
	stakeDuration, ok := new(big.Int).SetString(stakeDurationString, 10)
	if !ok {
		return nil, output.NewError(output.ConvertError, "failed to convert stake duration", nil)
	}

	if err := validator.ValidateStakeDuration(stakeDuration); err != nil {
		return nil, output.NewError(output.ValidationError, "invalid stake duration", err)
	}

	return stakeDuration, nil
}
