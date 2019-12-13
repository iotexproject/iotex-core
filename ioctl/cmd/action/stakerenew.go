package action

import (
	"encoding/hex"
	"math/big"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

// Multi-language support
var (
	stakeRenewCmdUses = map[config.Language]string{
		config.English: "renew BUCKET_INDEX STAKE_DURATION [DATA] [--auto-restake] [-c ALIAS|CONTRACT_ADDRESS]" +
			" [-s SIGNER] [-n NONCE] [-l GAS_LIMIT] [-p GAS_PRICE] [-P PASSWORD] [-y]",
		config.Chinese: "renew 桶索引 权益时间 [数据] [--auto-restake] [-c 别名" +
			"|合约地址]" +
			" [-s 签署人] [-n NONCE] [-l GAS限制] [-p GAS价格] [-P 密码] [-y]",
	}
	stakeRenewCmdShorts = map[config.Language]string{
		config.English: "Renew bucket on IoTeX blockchain",
		config.Chinese: "更新IoTeX区块链上的存储桶",
	}
	flagStakeRenewCmdAutoRestakeUsages = map[config.Language]string{
		config.English: "auto restake without power decay",
		config.Chinese: "无功率衰减的自动重启动",
	}
)

// stakeRenewCmd represents the stake renew command
var stakeRenewCmd = &cobra.Command{
	Use:   config.TranslateInLang(stakeRenewCmdUses, config.UILanguage),
	Short: config.TranslateInLang(stakeRenewCmdShorts, config.UILanguage),
	Args:  cobra.RangeArgs(2, 3),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := renew(args)
		return output.PrintError(err)
	},
}

func init() {
	registerWriteCommand(stakeRenewCmd)
	stakeRenewCmd.Flags().BoolVar(&autoRestake, "auto-restake", false,
		config.TranslateInLang(flagStakeRenewCmdAutoRestakeUsages, config.UILanguage))
}

func renew(args []string) error {
	bucketIndex, ok := new(big.Int).SetString(args[0], 10)
	if !ok {
		return output.NewError(output.ConvertError, "failed to convert bucket index", nil)
	}

	stakeDuration, err := parseStakeDuration(args[1])
	if err != nil {
		return output.NewError(0, "", err)
	}

	data := []byte{}
	if len(args) == 3 {
		data = make([]byte, 2*len([]byte(args[2])))
		hex.Encode(data, []byte(args[2]))
	}

	contract, err := stakingContract()
	if err != nil {
		return output.NewError(output.AddressError, "failed to get contract address", err)
	}

	bytecode, err := stakeABI.Pack("restake", bucketIndex, stakeDuration, autoRestake, data)
	if err != nil {
		return output.NewError(output.ConvertError, "cannot generate bytecode from given command", err)
	}

	return Execute(contract.String(), big.NewInt(0), bytecode)
}
