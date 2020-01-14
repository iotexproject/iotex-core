package action

import (
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

// Multi-language support
var (
	releaseCmdUses = map[config.Language]string{
		config.English: "release BUCKET_INDEX [DATA] [-c ALIAS|CONTRACT_ADDRESS]" +
			" [-s SIGNER] [-n NONCE] [-l GAS_LIMIT] [-p GAS_PRICE] [-P PASSWORD] [-y]",
		config.Chinese: "release 桶索引 [数据] [-c 别名|合约地址]" +
			" [-s 签署人] [-n NONCE] [-l GAS限制] [-p GAS价格] [-P 密码] [-y]",
	}
	releaseCmdShorts = map[config.Language]string{
		config.English: "Release bucket on IoTeX blockchain",
		config.Chinese: "发布IoTeX区块链上的存储桶",
	}
)

// stakeReleaseCmd represents the stake release command
var stakeReleaseCmd = &cobra.Command{
	Use:   config.TranslateInLang(releaseCmdUses, config.UILanguage),
	Short: config.TranslateInLang(releaseCmdShorts, config.UILanguage),
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := release(args)
		return output.PrintError(err)
	},
}

func init() {
	registerWriteCommand(stakeReleaseCmd)
}

func release(args []string) error {
	return bucketAction("unstake", args)
}
