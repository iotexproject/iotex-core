package ws

import (
	"fmt"
	"math/big"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
)

var wsProjectPauseCmd = &cobra.Command{
	Use: "pause",
	Short: config.TranslateInLang(map[config.Language]string{
		config.English: "pause project",
		config.Chinese: "停止项目",
	}, config.UILanguage),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectID := big.NewInt(int64(projectID.Value().(uint64)))
		out, err := pauseProject(projectID)
		if err != nil {
			return output.PrintError(err)
		}
		output.PrintResult(fmt.Sprintf("project %d paused", projectID))
		output.PrintResult(output.JSONString(out))
		return nil
	},
}

var wsProjectResumeCmd = &cobra.Command{
	Use: "resume",
	Short: config.TranslateInLang(map[config.Language]string{
		config.English: "resume project",
		config.Chinese: "开启项目",
	}, config.UILanguage),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectID := big.NewInt(int64(projectID.Value().(uint64)))
		out, err := resumeProject(projectID)
		if err != nil {
			return output.PrintError(err)
		}
		output.PrintResult(fmt.Sprintf("project %d resumed", projectID))
		output.PrintResult(output.JSONString(out))
		return nil
	},
}

func init() {
	projectID.RegisterCommand(wsProjectPauseCmd)
	projectID.MarkFlagRequired(wsProjectPauseCmd)

	projectID.RegisterCommand(wsProjectResumeCmd)
	projectID.MarkFlagRequired(wsProjectResumeCmd)

	wsProject.AddCommand(wsProjectPauseCmd)
	wsProject.AddCommand(wsProjectResumeCmd)
}

func pauseProject(projectID *big.Int) (any, error) {
	caller, err := NewContractCaller(projectStoreABI, projectStoreAddress)
	if err != nil {
		return nil, errors.Wrap(err, "failed to new contract caller")
	}

	result := NewContractResult(&projectStoreABI, eventProjectPaused, nil)
	_, err = caller.CallAndRetrieveResult(funcPauseProject, []any{projectID}, result)
	if err != nil {
		return nil, errors.Wrap(err, "failed to call contract")
	}
	_, err = result.Result()
	if err != nil {
		return nil, err
	}
	return queryProject(projectID)
}

func resumeProject(projectID *big.Int) (any, error) {
	caller, err := NewContractCaller(projectStoreABI, projectStoreAddress)
	if err != nil {
		return nil, errors.Wrap(err, "failed to new contract caller")
	}

	result := NewContractResult(&projectStoreABI, eventProjectResumed, nil)
	_, err = caller.CallAndRetrieveResult(funcResumeProject, []any{projectID}, result)
	if err != nil {
		return nil, errors.Wrap(err, "failed to call contract")
	}
	_, err = result.Result()
	if err != nil {
		return nil, err
	}
	return queryProject(projectID)
}
