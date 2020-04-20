// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package contract

import (
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

// Multi-language support
var (
	deployCmdUses = map[config.Language]string{
		config.English: "deploy command has the following five usages:\n\tioctl contract deploy BYTE_CODE\n\tioctl contract deploy BYTE_CODE INIT_INPUT --abi ABI_FILE\n\tioctl contract deploy --abi ABI_FILE\n\tioctl contract deploy INIT_INPUT --bin BIN_FILE --abi ABI_FILE\n\tioctl contract deploy CONTRACT_NAME [INIT_INPUT] --source CODE_FILE",
		config.Chinese: "deploy 命令有以下五种用法:\n\tioctl contract deploy BYTE_CODE\n\tioctl contract deploy BYTE_CODE 初始化参数 --abi ABI_FILE\n\tioctl contract deploy --abi ABI_FILE\n\tioctl contract deploy 初始化参数 --bin BIN_FILE --abi ABI_FILE\n\tioctl contract deploy 合约名 [初始化参数] --source CODE_FILE",
	}
	deployCmdShorts = map[config.Language]string{
		config.English: "deploy smart contract of IoTeX blockchain",
		config.Chinese: "在IoTeX区块链部署智能合约",
	}
)

// contractDeployCmd represents the contract deploy command
var contractDeployCmd = &cobra.Command{
	Use:   config.TranslateInLang(deployCmdUses, config.UILanguage),
	Short: config.TranslateInLang(deployCmdShorts, config.UILanguage),
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {

		abiLen := len(abiFileFlag.Value().(string))
		binLen := len(binFileFlag.Value().(string))
		sourceLen := len(sourceFileFlag.Value().(string))

		switch len(args) {
		case 0:
			if binLen != 0 && abiLen == 0 && sourceLen == 0 {
				err := constractDeployConstructorWithoutParams()
				return err
			}
			return output.NewError(output.InputError, "unknown command. \nRun 'ioctl contract deploy --help' for usage.", nil)

		case 1:
			if abiLen == 0 && binLen == 0 && sourceLen == 0 {
				err := constractDeployConstructorWithByteCode(args)
				return err
			}
			if abiLen != 0 && binLen != 0 && sourceLen == 0 {
				err := constractDeployConstructorWithInitInput(args)
				return err
			}
			if abiLen == 0 && binLen == 0 && sourceLen != 0 {
				err := constractDeployConstructorWithContractName(args)
				return err
			}
			return output.NewError(output.InputError, "unknown command. \nRun 'ioctl contract deploy --help' for usage.", nil)

		case 2:
			if abiLen != 0 && binLen == 0 && sourceLen == 0 {
				err := constractDeployConstructorWithByteCodeAndInitInput(args)
				return err
			}
			if abiLen == 0 && binLen == 0 && sourceLen != 0 {
				err := constractDeployConstructorWithContractName(args)
				return err
			}
			return output.NewError(output.InputError, "unknown command. \nRun 'ioctl contract deploy --help' for usage.", nil)

		default:
			return output.NewError(output.InputError, "unknown command. \nRun 'ioctl contract deploy --help' for usage.", nil)
		}
	},
}

func init() {
	registerWriteCommand(contractDeployCmd)
}

func constractDeployConstructorWithoutParams() error {
	//binFile := binFileFlag.Value().(string)
	return nil
}

func constractDeployConstructorWithInitInput(args []string) error {
	////initInput := args[0]
	//abiFile := abiFileFlag.Value().(string)
	//binFile := binFileFlag.Value().(string)
	return nil
}

func constractDeployConstructorWithByteCode(args []string) error {
	//byteCode := args[0]
	return nil
}

func constractDeployConstructorWithContractName(args []string) error {
	//contractName := args[0]
	//var initInput string
	//if(len(args)==2){
	//	initInput = args[1]
	//}
	//sourceFile := sourceFileFlag.Value().(string)
	return nil
}

func constractDeployConstructorWithByteCodeAndInitInput(args []string) error {
	//byteCode := args[0]
	//initInput := args[1]
	//abiFile := abiFileFlag.Value().(string)
	return nil
}
