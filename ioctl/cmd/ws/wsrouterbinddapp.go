package ws

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
	"github.com/iotexproject/iotex-core/v2/ioctl/util"
)

var (
	// wsRouterBindDapp
	wsRouterBindDapp = &cobra.Command{
		Use:   "bind",
		Short: config.TranslateInLang(wsRouterBindDappShorts, config.UILanguage),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := cmd.Flags().GetUint64("project-id")
			if err != nil {
				return errors.Wrap(err, "failed to get flag project-id")
			}
			dapp, err := cmd.Flags().GetString("dapp")
			if err != nil {
				return errors.Wrap(err, "failed to get flag dapp")
			}

			out, err := bindDapp(projectID, dapp)
			if err != nil {
				return output.PrintError(err)
			}
			output.PrintResult(out)
			return nil
		},
	}

	wsRouterBindDappShorts = map[config.Language]string{
		config.English: "bind project to dapp",
		config.Chinese: "绑定项目与dapp",
	}

	_flagDappUsages = map[config.Language]string{
		config.English: "dapp address for the project",
		config.Chinese: "该project的Dapp地址",
	}
)

func init() {
	wsRouterBindDapp.Flags().Uint64P("project-id", "", 0, config.TranslateInLang(_flagProjectIDUsages, config.UILanguage))
	wsRouterBindDapp.Flags().StringP("dapp", "", "", config.TranslateInLang(_flagDappUsages, config.UILanguage))

	_ = wsRouterBindDapp.MarkFlagRequired("project-id")
	_ = wsRouterBindDapp.MarkFlagRequired("dapp")

	wsRouterCmd.AddCommand(wsRouterBindDapp)
}

func bindDapp(projectID uint64, dapp string) (string, error) {
	addrStr, err := util.Address(dapp)
	if err != nil {
		return "", errors.Wrapf(err, "invalid dapp address: %s", dapp)
	}
	// Convert addrStr to an Address type
	addr, err := address.FromString(addrStr)
	if err != nil {
		return "", errors.Wrapf(err, "failed to parse address: %s", addrStr)
	}

	newAddr := common.BytesToAddress(addr.Bytes())

	caller, err := NewContractCaller(routerABI, routerAddress)
	if err != nil {
		return "", errors.Wrap(err, "failed to create contract caller")
	}

	result := NewContractResult(&routerABI, eventDappBound, nil)
	if _, err = caller.CallAndRetrieveResult(funcBindDapp, []any{
		big.NewInt(int64(projectID)),
		newAddr,
	}, result); err != nil {
		return "", errors.Wrap(err, "failed to read contract")
	}
	if _, err = result.Result(); err != nil {
		return "", err
	}

	return fmt.Sprintf("bind dapp %s success", dapp), nil
}
