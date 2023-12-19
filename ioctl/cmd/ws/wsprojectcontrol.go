package ws

import (
	"fmt"
	"math/big"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/cmd/action"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

var (
	wsProjectStart = &cobra.Command{
		Use:   "start",
		Short: config.TranslateInLang(wsProjectStartShorts, config.UILanguage),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := cmd.Flags().GetUint64("project-id")
			if err != nil {
				return output.PrintError(err)
			}
			out, err := controlProjectState(id, startProject)
			if err != nil {
				return output.PrintError(err)
			}
			output.PrintResult(out)
			return nil
		},
	}

	wsProjectStop = &cobra.Command{
		Use:   "stop",
		Short: config.TranslateInLang(wsProjectStopShorts, config.UILanguage),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := cmd.Flags().GetUint64("project-id")
			if err != nil {
				return output.PrintError(err)
			}
			out, err := controlProjectState(id, stopProject)
			if err != nil {
				return output.PrintError(err)
			}
			output.PrintResult(out)
			return nil
		},
	}

	wsProjectStartShorts = map[config.Language]string{
		config.English: "start w3bstream project",
		config.Chinese: "开启项目",
	}

	wsProjectStopShorts = map[config.Language]string{
		config.English: "stop w3bstream project",
		config.Chinese: "暂停项目",
	}
)

const (
	startProject = 0
	stopProject  = 1
)

func init() {
	wsProjectStop.Flags().Uint64P("project-id", "i", 0, config.TranslateInLang(_flagProjectIDUsages, config.UILanguage))
	wsProjectStart.Flags().Uint64P("project-id", "i", 0, config.TranslateInLang(_flagProjectIDUsages, config.UILanguage))

	_ = wsProjectStop.MarkFlagRequired("project-id")
	_ = wsProjectStart.MarkFlagRequired("project-id")
}

func controlProjectState(projectID uint64, command int) (string, error) {
	funcName := startWsProjectFuncName
	if command == stopProject {
		funcName = stopWsProjectFuncName
	}

	contract, err := util.Address(wsProjectRegisterContractAddress)
	if err != nil {
		return "", output.NewError(output.AddressError, "failed to get project register contract address", err)
	}

	bytecode, err := wsProjectRegisterContractABI.Pack(funcName, projectID)
	if err != nil {
		return "", output.NewError(output.ConvertError, fmt.Sprintf("failed to pack abi"), err)
	}

	if err = action.Execute(contract, big.NewInt(0), bytecode); err != nil {
		return "", errors.Wrap(err, "failed to execute contract")
	}
	if command == startProject {
		return fmt.Sprintf("project %d started", projectID), nil
	}
	return fmt.Sprintf("project %d stopped", projectID), nil
}
