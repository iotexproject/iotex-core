package ws

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/v2/ioctl/cmd/ws/contracts"
	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
)

var wsVmTypeRegisterCmd = &cobra.Command{
	Use: "register",
	Short: config.TranslateInLang(map[config.Language]string{
		config.English: "register vmType",
		config.Chinese: "注册虚拟机类型",
	}, config.UILanguage),
	RunE: func(cmd *cobra.Command, args []string) error {
		out, err := registerVmType(vmTypeName.Value().(string))
		if err != nil {
			return output.PrintError(err)
		}
		output.PrintResult(output.JSONString(out))
		return nil
	},
}

func init() {
	vmTypeName.RegisterCommand(wsVmTypeRegisterCmd)
	vmTypeName.MarkFlagRequired(wsVmTypeRegisterCmd)

	wsVmTypeCmd.AddCommand(wsVmTypeRegisterCmd)
}

func registerVmType(name string) (any, error) {
	caller, err := NewContractCaller(vmTypeABI, vmTypeAddress)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create contract caller")
	}

	value := new(contracts.W3bstreamVMTypeVMTypeSet)
	result := NewContractResult(&vmTypeABI, eventOnVmTypeSet, value)
	if _, err = caller.CallAndRetrieveResult(funcVmTypeMint, []any{name}, result); err != nil {
		return nil, errors.Wrap(err, "failed to call contract")
	}

	if _, err = result.Result(); err != nil {
		return nil, err
	}

	return queryVmType(value.Id)
}
