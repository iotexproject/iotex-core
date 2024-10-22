package ws

import (
	"math/big"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
)

var wsVmTypeQueryCmd = &cobra.Command{
	Use: "query",
	Short: config.TranslateInLang(map[config.Language]string{
		config.English: "query vmType",
		config.Chinese: "查询虚拟机类型信息",
	}, config.UILanguage),
	RunE: func(cmd *cobra.Command, args []string) error {
		out, err := queryVmType(big.NewInt(int64(vmTypeID.Value().(uint64))))
		if err != nil {
			return output.PrintError(err)
		}
		output.PrintResult(output.JSONString(out))
		return nil
	},
}

func init() {
	vmTypeID.RegisterCommand(wsVmTypeQueryCmd)
	vmTypeID.MarkFlagRequired(wsVmTypeQueryCmd)

	wsVmTypeCmd.AddCommand(wsVmTypeQueryCmd)
}

func queryVmType(vmTypeID *big.Int) (any, error) {
	caller, err := NewContractCaller(vmTypeABI, vmTypeAddress)
	if err != nil {
		return nil, errors.Wrap(err, "failed to new contract caller")
	}
	result := NewContractResult(&vmTypeABI, funcQueryVmType, new(string))
	if err = caller.Read(funcQueryVmType, []any{vmTypeID}, result); err != nil {
		return nil, errors.Wrapf(err, "failed to read contract: %s", funcQueryVmType)
	}

	vmTypeName, err := result.Result()
	if err != nil {
		return nil, err
	}

	result = NewContractResult(&vmTypeABI, funcQueryVmTypeIsPaused, new(bool))
	if err = caller.Read(funcQueryVmTypeIsPaused, []any{vmTypeID}, result); err != nil {
		return nil, errors.Wrapf(err, "failed to read contract: %s", funcQueryVmTypeIsPaused)
	}
	isPaused, err := result.Result()
	if err != nil {
		return nil, err
	}

	return &struct {
		VmTypeID uint64 `json:"vmTypeID"`
		VmType   string `json:"vmType"`
		IsPaused bool   `json:"isPaused"`
	}{
		VmTypeID: vmTypeID.Uint64(),
		VmType:   *vmTypeName.(*string),
		IsPaused: *isPaused.(*bool),
	}, nil

}
