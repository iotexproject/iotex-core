package ioid

import (
	"bytes"
	_ "embed" // used to embed contract abi
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/v2/ioctl/cmd/ws"
	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
)

// Multi-language support
var (
	_registerUsages = map[config.Language]string{
		config.English: "register [PROJECT_NAME]",
		config.Chinese: "register [项目名称]",
	}
	_registerShorts = map[config.Language]string{
		config.English: "Register project",
		config.Chinese: "注册项目",
	}
	_projectRegisterUsages = map[config.Language]string{
		config.English: "project registry contract address",
		config.Chinese: "项目注册合约地址",
	}
	_projectTypeUsages = map[config.Language]string{
		config.English: "project type, 0 represents hardware, and 1 represents virtual.",
		config.Chinese: "项目类型，0代表硬件，1代表虚拟",
	}
)

// _projectRegisterCmd represents the project register command
var _projectRegisterCmd = &cobra.Command{
	Use:   config.TranslateInLang(_registerUsages, config.UILanguage),
	Short: config.TranslateInLang(_registerShorts, config.UILanguage),
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		err := register(args)
		return output.PrintError(err)
	},
}

var (
	projectRegistry string
	projectType     uint8
	//go:embed contracts/abis/ProjectRegistry.json
	projectRegistryJSON []byte
	projectRegistryABI  abi.ABI
)

func init() {
	var err error
	projectRegistryABI, err = abi.JSON(bytes.NewReader(projectRegistryJSON))
	if err != nil {
		panic(err)
	}

	_projectRegisterCmd.Flags().StringVarP(
		&projectRegistry, "projectRegistry", "p",
		config.ReadConfig.IoidProjectRegisterContract,
		config.TranslateInLang(_projectRegisterUsages, config.UILanguage),
	)
	_projectRegisterCmd.Flags().Uint8VarP(
		&projectType, "projectType", "t",
		0,
		config.TranslateInLang(_projectTypeUsages, config.UILanguage),
	)
}

func register(args []string) error {
	name := args[0]

	caller, err := ws.NewContractCaller(projectRegistryABI, projectRegistry)
	if err != nil {
		return output.NewError(output.SerializationError, "failed to create contract caller", err)
	}

	tx, err := caller.CallAndRetrieveResult("register0", []any{
		name,
		projectType,
	})
	if err != nil {
		return output.NewError(output.SerializationError, "failed to call contract", err)
	}

	receipt, err := waitReceiptByActionHash(tx)
	if err != nil {
		return output.NewError(output.UpdateError, "failed to register project", err)
	}

	projectId := new(big.Int).SetBytes(receipt.ReceiptInfo.Receipt.Logs[0].Topics[3])
	fmt.Printf("Registerd ioID project id is %s\n", projectId.String())

	return nil
}
