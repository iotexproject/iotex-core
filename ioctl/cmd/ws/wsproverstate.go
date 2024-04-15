package ws

import (
	"fmt"
	"math/big"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

var wsProverPauseCmd = &cobra.Command{
	Use: "pause",
	Short: config.TranslateInLang(map[config.Language]string{
		config.English: "pause prover",
		config.Chinese: "停止prover",
	}, config.UILanguage),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := big.NewInt(int64(proverID.Value().(uint64)))
		if err := pauseProver(id); err != nil {
			return output.PrintError(err)
		}
		output.PrintResult(fmt.Sprintf("prover %d paused", id))
		return nil
	},
}

var wsProverResumeCmd = &cobra.Command{
	Use: "resume",
	Short: config.TranslateInLang(map[config.Language]string{
		config.English: "resume prover",
		config.Chinese: "启动prover",
	}, config.UILanguage),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := big.NewInt(int64(proverID.Value().(uint64)))
		if err := resumeProver(id); err != nil {
			return output.PrintError(err)
		}
		output.PrintResult(fmt.Sprintf("prover %d resumed", id))
		return nil
	},
}

func init() {
	proverID.RegisterCommand(wsProverPauseCmd)
	proverID.MarkFlagRequired(wsProverPauseCmd)

	proverID.RegisterCommand(wsProverResumeCmd)
	proverID.MarkFlagRequired(wsProverResumeCmd)

	wsProverCmd.AddCommand(wsProverPauseCmd)
	wsProverCmd.AddCommand(wsProverResumeCmd)
}

func pauseProver(proverID *big.Int) error {
	caller, err := NewContractCaller(proverStoreABI, proverStoreAddress)
	if err != nil {
		return errors.Wrap(err, "failed to new contract caller")
	}

	result := NewContractResult(&proverStoreABI, eventOnProverPaused, nil)
	_, err = caller.CallAndRetrieveResult(funcPauseProver, []any{proverID}, result)
	if err != nil {
		return errors.Wrap(err, "failed to call contract")
	}
	_, err = result.Result()
	if err != nil {
		return err
	}
	return nil

}

func resumeProver(proverID *big.Int) error {
	caller, err := NewContractCaller(proverStoreABI, proverStoreAddress)
	if err != nil {
		return errors.Wrap(err, "failed to new contract caller")
	}

	result := NewContractResult(&proverStoreABI, eventOnProverResumed, nil)
	_, err = caller.CallAndRetrieveResult(funcResumeProver, []any{proverID}, result)
	if err != nil {
		return errors.Wrap(err, "failed to call contract")
	}
	_, err = result.Result()
	if err != nil {
		return err
	}
	return nil
}
