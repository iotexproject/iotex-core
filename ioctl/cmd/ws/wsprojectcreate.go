package ws

import (
	"encoding/hex"
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
	// wsProjectCreate represents the create w3bstream project command
	wsProjectCreate = &cobra.Command{
		Use:   "create",
		Short: config.TranslateInLang(wsProjectCreateShorts, config.UILanguage),
		RunE: func(cmd *cobra.Command, args []string) error {
			filename, err := cmd.Flags().GetString("project-config-file")
			if err != nil {
				return output.PrintError(err)
			}
			hash, err := cmd.Flags().GetString("project-config-hash")
			if err != nil {
				return output.PrintError(err)
			}
			out, err := createProject(filename, hash)
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
)

func init() {
	wsProjectCreate.Flags().StringP("project-config-file", "u", "", config.TranslateInLang(_flagProjectConfigFileUsages, config.UILanguage))
	wsProjectCreate.Flags().StringP("project-config-hash", "v", "", config.TranslateInLang(_flagProjectConfigHashUsages, config.UILanguage))

	_ = wsProjectCreate.MarkFlagRequired("project-config-file")
}

func createProject(filename, hashstr string) (string, error) {
	uri, hashv, err := upload(wsProjectIPFSEndpoint, filename, hashstr)
	if err != nil {
		return "", err
	}
	fmt.Printf("project config file validated and uploaded:\n"+
		"\tipfs uri: %s\n"+
		"\thash256:  %s\n\n\n", uri, hex.EncodeToString(hashv[:]))

	contract, err := util.Address(wsProjectRegisterContractAddress)
	if err != nil {
		return "", output.NewError(output.AddressError, "failed to get project register contract address", err)
	}

	bytecode, err := wsProjectRegisterContractABI.Pack(createWsProjectFuncName, uri, hashv)
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

	inputs, err := getEventInputsByName(r.ReceiptInfo.Receipt.Logs, wsProjectUpsertedEventName)
	if err != nil {
		return "", errors.Wrap(err, "get receipt event failed")
	}
	projectid, ok := inputs["projectId"]
	if !ok {
		return "", errors.New("result not found in event inputs")
	}
	hashstr = hex.EncodeToString(hashv[:])
	return fmt.Sprintf("Your project is successfully created:\n%s", output.JSONString(&projectMeta{
		ProjectID:  projectid.(uint64),
		URI:        uri,
		HashSha256: hashstr,
	})), nil
}
