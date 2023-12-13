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
	// wsProjectUpdate represents the update w3bstream project command
	wsProjectUpdate = &cobra.Command{
		Use:   "update",
		Short: config.TranslateInLang(wsProjectUpdateShorts, config.UILanguage),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := cmd.Flags().GetUint64("project-id")
			if err != nil {
				return output.PrintError(err)
			}
			filename, err := cmd.Flags().GetString("project-config-file")
			if err != nil {
				return output.PrintError(err)
			}
			hash, err := cmd.Flags().GetString("project-config-hash")
			if err != nil {
				return output.PrintError(err)
			}
			out, err := updateProject(id, filename, hash)
			if err != nil {
				return output.PrintError(err)
			}
			output.PrintResult(out)
			return nil
		},
	}

	// wsProjectUpdateShorts update w3bstream project shorts multi-lang support
	wsProjectUpdateShorts = map[config.Language]string{
		config.English: "update w3bstream project",
		config.Chinese: "更新项目",
	}
)

func init() {
	wsProjectUpdate.Flags().Uint64P("project-id", "i", 0, config.TranslateInLang(_flagProjectIDUsages, config.UILanguage))
	wsProjectUpdate.Flags().StringP("project-config-file", "u", "", config.TranslateInLang(_flagProjectConfigFileUsages, config.UILanguage))
	wsProjectUpdate.Flags().StringP("project-config-hash", "v", "", config.TranslateInLang(_flagProjectConfigHashUsages, config.UILanguage))

	_ = wsProjectCreate.MarkFlagRequired("project-id")
	_ = wsProjectCreate.MarkFlagRequired("project-config-file")
}

func updateProject(projectID uint64, filename, hashstr string) (string, error) {
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

	bytecode, err := wsProjectRegisterContractABI.Pack(updateWsProjectFuncName, projectID, uri, hashv)
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
	_projectid, ok := inputs["projectId"]
	if !ok {
		return "", errors.New("result `projectId` not found in event inputs")
	}
	_uri, ok := inputs["uri"]
	if !ok {
		return "", errors.New("result `uri` not found in event inputs")
	}
	_hash, ok := inputs["hash"].([32]byte)
	if !ok {
		return "", errors.New("result `hash` not found in event inputs")
	}
	hashstr = hex.EncodeToString(_hash[:])

	return fmt.Sprintf("Your project is successfully created. project id is : %d\n"+
		"project config url:  %s\n"+
		"project config hash: %s", _projectid, _uri, hashstr), nil

}
