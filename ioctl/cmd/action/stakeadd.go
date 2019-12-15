package action

import (
	"encoding/hex"
	"math/big"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

// Multi-language support
var (
	addCmdUses = map[config.Language]string{
		config.English: "add IOTX_AMOUNT BUCKET_INDEX [DATA] [-c ALIAS|CONTRACT_ADDRESS]" +
			" [-s SIGNER] [-n NONCE] [-l GAS_LIMIT] [-p GAS_PRICE] [-P PASSWORD] [-y]",
		config.Chinese: "add IOTX数量 桶索引 [数据] [-c 别名|合约地址]" +
			" [-s 签署人] [-n NONCE] [-l GAS限制] [-p GAS价格] [-P 密码] [-y]",
	}
	addCmdShorts = map[config.Language]string{
		config.English: "Add IOTX to bucket on IoTeX blockchain",
		config.Chinese: "将IOTX添加到IoTeX区块链上的存储桶中",
	}
)

// stakeAddCmd represents the stake add command
var stakeAddCmd = &cobra.Command{
	Use:   config.TranslateInLang(addCmdUses, config.UILanguage),
	Short: config.TranslateInLang(actionCmdShorts, config.UILanguage),
	Args:  cobra.RangeArgs(2, 3),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := add(args)
		return output.PrintError(err)
	},
}

func init() {
	registerWriteCommand(stakeAddCmd)
}

func add(args []string) error {
	amount, err := util.StringToRau(args[0], util.IotxDecimalNum)
	if err != nil {
		return output.NewError(output.ConvertError, "invalid IOTX amount", err)
	}

	bucketIndex, ok := new(big.Int).SetString(args[1], 10)
	if !ok {
		return output.NewError(output.ConvertError, "failed to convert bucket index", nil)
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

	bytecode, err := stakeABI.Pack("storeToPygg", bucketIndex, data)
	if err != nil {
		return output.NewError(output.ConvertError, "cannot generate bytecode from given command", err)
	}

	return Execute(contract.String(), amount, bytecode)
}
