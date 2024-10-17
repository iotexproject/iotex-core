package ws

import (
	"fmt"
	"math/big"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
)

var wsVmTypePauseCmd = &cobra.Command{
	Use: "pause",
	Short: config.TranslateInLang(map[config.Language]string{
		config.English: "pause vmType",
		config.Chinese: "停止虚拟机类型",
	}, config.UILanguage),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := big.NewInt(int64(vmTypeID.Value().(uint64)))
		out, err := pauseVmType(id)
		if err != nil {
			return output.PrintError(err)
		}
		output.PrintResult(fmt.Sprintf("vmType %d paused", id))
		output.PrintResult(output.JSONString(out))
		return nil
	},
}

var wsVmTypeResumeCmd = &cobra.Command{
	Use: "resume",
	Short: config.TranslateInLang(map[config.Language]string{
		config.English: "resume vmType",
		config.Chinese: "启动虚拟机类型",
	}, config.UILanguage),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := big.NewInt(int64(vmTypeID.Value().(uint64)))
		out, err := resumeVmType(id)
		if err != nil {
			return output.PrintError(err)
		}
		output.PrintResult(fmt.Sprintf("vmType %d resumed", id))
		output.PrintResult(output.JSONString(out))
		return nil
	},
}

func init() {
	vmTypeID.RegisterCommand(wsVmTypePauseCmd)
	vmTypeID.MarkFlagRequired(wsVmTypePauseCmd)

	vmTypeID.RegisterCommand(wsVmTypeResumeCmd)
	vmTypeID.MarkFlagRequired(wsVmTypeResumeCmd)

	wsVmTypeCmd.AddCommand(wsVmTypePauseCmd)
	wsVmTypeCmd.AddCommand(wsVmTypeResumeCmd)
}

func pauseVmType(vmTypeID *big.Int) (any, error) {
	caller, err := NewContractCaller(vmTypeABI, vmTypeAddress)
	if err != nil {
		return nil, errors.Wrap(err, "failed to new contract caller")
	}

	result := NewContractResult(&vmTypeABI, eventOnVmTypePaused, nil)
	_, err = caller.CallAndRetrieveResult(funcVmTypePause, []any{vmTypeID}, result)
	if err != nil {
		return nil, errors.Wrap(err, "failed to call contract")
	}
	_, err = result.Result()
	if err != nil {
		return nil, err
	}
	return queryVmType(vmTypeID)
}

func resumeVmType(vmTypeID *big.Int) (any, error) {
	caller, err := NewContractCaller(vmTypeABI, vmTypeAddress)
	if err != nil {
		return nil, errors.Wrap(err, "failed to new contract caller")
	}

	result := NewContractResult(&vmTypeABI, eventOnVmTypeResumed, nil)
	_, err = caller.CallAndRetrieveResult(funcVmTypeResume, []any{vmTypeID}, result)
	if err != nil {
		return nil, errors.Wrap(err, "failed to call contract")
	}
	_, err = result.Result()
	if err != nil {
		return nil, err
	}

	return queryVmType(vmTypeID)
}
