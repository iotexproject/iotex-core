package action

import (
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

// Multi-language support
var (
	withDrawCmdUses = map[config.Language]string{
		config.English: "withdraw BUCKET_INDEX [DATA] [-c ALIAS|CONTRACT_ADDRESS]" +
			" [-s SIGNER] [-n NONCE] [-l GAS_LIMIT] [-p GAS_PRICE] [-P PASSWORD] [-y]",
		config.Chinese: "withdraw 桶索引 [数据] [-c 别名|合约地址]" +
			" [-s 签署人] [-n NONCE] [-l GAS限制] [-p GAS价格] [-P 密码] [-y]",
	}
	withDrawCmdShorts = map[config.Language]string{
		config.English: "Withdraw form bucket on IoTeX blockchain",
		config.Chinese: "在IoTeX区块链上提取表单存储桶",
	}
)

// stakeWithdrawCmd represents the stake withdraw command
var stakeWithdrawCmd = &cobra.Command{
	Use:   config.TranslateInLang(withDrawCmdUses, config.UILanguage),
	Short: config.TranslateInLang(withDrawCmdShorts, config.UILanguage),
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := withdraw(args)
		return output.PrintError(err)
	},
}

func init() {
	registerWriteCommand(stakeWithdrawCmd)
}

func withdraw(args []string) error {
	return bucketAction("withdraw", args)
}
