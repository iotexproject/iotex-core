package ws

import (
	"fmt"
	"math/big"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
)

var (
	// wsRouterUnbindDapp
	wsRouterUnbindDapp = &cobra.Command{
		Use:   "unbind",
		Short: config.TranslateInLang(wsRouterUnbindDappShorts, config.UILanguage),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := cmd.Flags().GetUint64("project-id")
			if err != nil {
				return errors.Wrap(err, "failed to get flag project-id")
			}

			out, err := unbindDapp(projectID)
			if err != nil {
				return output.PrintError(err)
			}
			output.PrintResult(out)
			return nil
		},
	}

	wsRouterUnbindDappShorts = map[config.Language]string{
		config.English: "unbind project to dapp",
		config.Chinese: "解除绑定项目与dapp",
	}
)

func init() {
	wsRouterUnbindDapp.Flags().Uint64P("project-id", "", 0, config.TranslateInLang(_flagProjectIDUsages, config.UILanguage))

	_ = wsRouterUnbindDapp.MarkFlagRequired("project-id")

	wsRouterCmd.AddCommand(wsRouterUnbindDapp)
}

func unbindDapp(projectID uint64) (string, error) {
	caller, err := NewContractCaller(routerABI, routerAddress)
	if err != nil {
		return "", errors.Wrap(err, "failed to create contract caller")
	}

	result := NewContractResult(&routerABI, eventDappUnbound, nil)
	if _, err = caller.CallAndRetrieveResult(funcUnbindDapp, []any{
		big.NewInt(int64(projectID)),
	}, result); err != nil {
		return "", errors.Wrap(err, "failed to read contract")
	}
	if _, err = result.Result(); err != nil {
		return "", err
	}

	return fmt.Sprintf("unbind project %d success", projectID), nil
}
