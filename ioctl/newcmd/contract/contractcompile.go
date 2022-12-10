// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package contract

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
)

var (
	_abiOut string
	_binOut string
)

// Multi-language support
var (
	_contractCompileCmdUses = map[config.Language]string{
		config.English: "compile CONTRACT_NAME [CODE_FILES...] [--abi-out ABI_PATH] [--bin-out BIN_PATH]",
		config.Chinese: "compile 合约名 [代码文件...] [--abi-out ABI路径] [--bin-out BIN路径]",
	}
	_contractCompileCmdShorts = map[config.Language]string{
		config.English: "Compile smart contract of IoTeX blockchain from source code file(s).",
		config.Chinese: "编译IoTeX区块链的智能合约代码,支持多文件编译",
	}
	_flagAbiOutUsage = map[config.Language]string{
		config.English: "set abi file output path",
		config.Chinese: "设置abi文件输出路径",
	}
	_flagBinOutUsage = map[config.Language]string{
		config.English: "set bin file output path",
		config.Chinese: "设置bin文件输出路径",
	}
)

// NewContractCompileCmd represents the contract compile command
func NewContractCompileCmd(client ioctl.Client) *cobra.Command {
	use, _ := client.SelectTranslation(_contractCompileCmdUses)
	short, _ := client.SelectTranslation(_contractCompileCmdShorts)
	flagAbi, _ := client.SelectTranslation(_flagAbiOutUsage)
	flagBin, _ := client.SelectTranslation(_flagBinOutUsage)

	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			contractName := args[0]

			files := args[1:]
			if len(files) == 0 {
				dirInfo, err := os.ReadDir("./")
				if err != nil {
					return errors.Wrap(err, "failed to get current directory")
				}

				for _, fileInfo := range dirInfo {
					if !fileInfo.IsDir() && strings.HasSuffix(fileInfo.Name(), ".sol") {
						files = append(files, fileInfo.Name())
					}
				}

				if len(files) == 0 {
					return errors.New("failed to get source file(s)")
				}
			}

			contracts, err := Compile(files...)
			if err != nil {
				return errors.Wrap(err, "failed to compile")
			}

			for name := range contracts {
				if name == contractName {
					break
				}
				nameSplit := strings.Split(name, ":")
				if nameSplit[len(nameSplit)-1] == contractName {
					contractName = name
					break
				}
			}

			contract, ok := contracts[contractName]
			if !ok {
				return errors.Errorf("failed to find out contract %s", contractName)
			}

			abiByte, err := json.Marshal(contract.Info.AbiDefinition)
			if err != nil {
				return errors.Wrap(err, "failed to marshal abi")
			}
			cmd.Printf("======= %s =======\nBinary:\n%s\nContract JSON ABI\n%s", contractName, contract.Code, string(abiByte))

			if _binOut != "" {
				// bin file starts with "0x" prefix
				if err := os.WriteFile(_binOut, []byte(contract.Code), 0600); err != nil {
					return errors.Wrap(err, "failed to write bin file")
				}
			}

			if _abiOut != "" {
				if err := os.WriteFile(_abiOut, abiByte, 0600); err != nil {
					return errors.Wrap(err, "failed to write abi file")
				}
			}

			return nil
		},
	}
	cmd.Flags().StringVar(&_abiOut, "abi-out", "", flagAbi)
	cmd.Flags().StringVar(&_binOut, "bin-out", "", flagBin)
	return cmd
}
