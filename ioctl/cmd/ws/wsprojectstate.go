package ws

import (
	"fmt"
	"math/big"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

var wsProjectPauseCmd = &cobra.Command{
	Use: "pause",
	Short: config.TranslateInLang(map[config.Language]string{
		config.English: "pause project",
		config.Chinese: "停止项目",
	}, config.UILanguage),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectID := flagProjectID.Value().(uint64)
		if err := pauseProject(projectID); err != nil {
			return output.PrintError(err)
		}
		output.PrintResult(fmt.Sprintf("project %d paused", projectID))
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
		projectID := flagProjectID.Value().(uint64)
		if err := resumeProject(projectID); err != nil {
			return output.PrintError(err)
		}
		output.PrintResult(fmt.Sprintf("project %d resumed", projectID))
		return nil
	},
}

func init() {
	flagProjectID.RegisterCommand(wsProjectPauseCmd)
	flagProjectID.MarkFlagRequired(wsProjectPauseCmd)

	flagProjectID.RegisterCommand(wsProjectResumeCmd)
	flagProjectID.MarkFlagRequired(wsProjectResumeCmd)

	wsProject.AddCommand(wsProjectPauseCmd)
	wsProject.AddCommand(wsProjectResumeCmd)
}

func pauseProject(projectID uint64) error {
	caller, err := NewContractCaller(projectStoreABI, projectStoreAddress)
	if err != nil {
		return errors.Wrap(err, "failed to new contract caller")
	}

	result := NewContractResult(&projectStoreABI, eventProjectPaused, nil)
	_, err = caller.CallAndRetrieveResult(funcPauseProject, []any{big.NewInt(int64(projectID))}, result)
	if err != nil {
		return errors.Wrap(err, "failed to call contract")
	}
	_, err = result.Result()
	if err != nil {
		return err
	}
	return nil
}

func resumeProject(projectID uint64) error {
	caller, err := NewContractCaller(projectStoreABI, projectStoreAddress)
	if err != nil {
		return errors.Wrap(err, "failed to new contract caller")
	}

	result := NewContractResult(&projectStoreABI, eventProjectResumed, nil)
	_, err = caller.CallAndRetrieveResult(funcResumeProject, []any{big.NewInt(int64(projectID))}, result)
	if err != nil {
		return errors.Wrap(err, "failed to call contract")
	}
	_, err = result.Result()
	if err != nil {
		return err
	}
	return nil
}
