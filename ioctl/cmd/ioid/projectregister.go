package ioid

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/iotexproject/iotex-address/address"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/cmd/action"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

// Multi-language support
var (
	_registerUsages = map[config.Language]string{
		config.English: "register",
		config.Chinese: "register",
	}
	_registerShorts = map[config.Language]string{
		config.English: "Register project",
		config.Chinese: "注册项目",
	}
	_projectRegisterUsages = map[config.Language]string{
		config.English: "project registry contract address",
		config.Chinese: "项目注册合约地址",
	}
)

// _projectRegisterCmd represents the action hash command
var _projectRegisterCmd = &cobra.Command{
	Use:   config.TranslateInLang(_registerUsages, config.UILanguage),
	Short: config.TranslateInLang(_registerShorts, config.UILanguage),
	Args:  cobra.MinimumNArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		err := register()
		return output.PrintError(err)
	},
}

var (
	projectRegistry    string
	projectRegistryABI = `[
		{
			"inputs": [],
			"name": "register",
			"outputs": [
				{
					"internalType": "uint256",
					"name": "",
					"type": "uint256"
				}
			],
			"stateMutability": "payable",
			"type": "function"
		}
	]`
)

func init() {
	_projectRegisterCmd.Flags().StringVarP(
		&projectRegistry, "projectRegistry", "p",
		"0xB7E1419d62ef429EE3130aF822e43DaBDDdB4aCE",
		config.TranslateInLang(_projectRegisterUsages, config.UILanguage),
	)
}

func register() error {
	ioProjectRegistry, err := address.FromHex(projectRegistry)
	if err != nil {
		return output.NewError(output.AddressError, "failed to convert project register address", err)
	}

	projectRegistryAbi, err := abi.JSON(strings.NewReader(projectRegistryABI))
	if err != nil {
		return output.NewError(output.SerializationError, "failed to unmarshal abi", err)
	}

	data, err := projectRegistryAbi.Pack("register")
	if err != nil {
		return output.NewError(output.ConvertError, "failed to pack registry arguments", err)
	}
	res, err := action.ExecuteAndResponse(ioProjectRegistry.String(), big.NewInt(0), data)
	if err != nil {
		return output.NewError(output.UpdateError, "failed to register project", err)
	}
	receipt, err := waitReceiptByActionHash(res.ActionHash)
	if err != nil {
		return output.NewError(output.UpdateError, "failed to register project", err)
	}

	projectId := new(big.Int).SetBytes(receipt.ReceiptInfo.Receipt.Logs[0].Topics[3])
	fmt.Printf("Registerd ioID project id is %s\n", projectId.String())

	return nil
}
