package ws

import (
	"fmt"
	"github.com/iotexproject/iotex-core/ioctl/cmd/action"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"math/big"
)

var (
	// wsProjectCreate represents the create w3bstream project command
	wsProjectCreate = &cobra.Command{
		Use:   "create",
		Short: config.TranslateInLang(wsProjectCreateShorts, config.UILanguage),
		RunE: func(cmd *cobra.Command, args []string) error {
			uri, err := cmd.Flags().GetString("project-uri")
			if err != nil {
				return output.PrintError(err)
			}
			hash, err := cmd.Flags().GetString("project-hash")
			if err != nil {
				return output.PrintError(err)
			}
			out, err := createProject(uri, hash)
			if err != nil {
				return output.PrintError(err)
			}
			output.PrintResult(out)
			return nil
		},
	}

	// wsProjectCreateShorts create w3bstream project shorts multi-lang support
	wsProjectCreateShorts = map[config.Language]string{
		config.English: "create w3bstream project",
		config.Chinese: "创建项目",
	}

	_flagProjectUriUsages = map[config.Language]string{
		config.English: "project config fetch uri",
		config.Chinese: "项目配置拉取地址",
	}
	_flagProjectHashUsages = map[config.Language]string{
		config.English: "project config hash for validating",
		config.Chinese: "项目配置hash",
	}
)

func init() {
	wsProjectCreate.Flags().StringP("project-uri", "u", "", config.TranslateInLang(_flagProjectUriUsages, config.UILanguage))
	wsProjectCreate.Flags().StringP("project-hash", "v", "", config.TranslateInLang(_flagProjectHashUsages, config.UILanguage))
	wsProjectCreate.Flags().StringP("contract-address", "v", "", config.TranslateInLang(_flagProjectHashUsages, config.UILanguage))

	_ = wsProjectCreate.MarkFlagRequired("project-uri")
	_ = wsProjectCreate.MarkFlagRequired("project-hash")
}

func createProject(uri, hash string) (string, error) {
	contract, err := util.Address(wsProjectRegisterContractAddress)
	if err != nil {
		return "", output.NewError(output.AddressError, "failed to get project register contract address", err)
	}

	hashArg, err := convertStringToAbiBytes32(hash)
	if err != nil {
		return "", err
	}

	bytecode, err := wsProjectRegisterContractABI.Pack(
		createWsProjectFuncName,
		uri, hashArg,
	)
	if err != nil {
		return "", output.NewError(output.ConvertError, fmt.Sprintf("failed to pack abi"), err)
	}

	res, err := action.ExecuteAndResponse(contract, big.NewInt(0), bytecode)
	if err != nil {
		return "", errors.Wrap(err, "execute contract failed")
	}

	r, err := waitReceiptByActionHash(res.ActionHash)
	if err != nil {
		return "", errors.Wrap(err, "wait contract execution receipt failed")
	}

	inputs, err := getEventInputsByName(r.ReceiptInfo.Receipt.Logs, createWsProjectEventName)
	if err != nil {
		return "", errors.Wrap(err, "get receipt event failed")
	}
	projectid, ok := inputs["projectId"]
	if !ok {
		return "", errors.New("result not found in event inputs")
	}
	return fmt.Sprint(projectid), nil
}
