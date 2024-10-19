package did

import (
	"encoding/json"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/v2/ioctl/cmd/action"
	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
)

// Multi-language support
var (
	_serviceremoveCmdShorts = map[config.Language]string{
		config.English: "Remove service to DID document using private key from wallet",
		config.Chinese: "用钱包中的私钥从DID document移除服务",
	}
	_serviceremoveCmdUses = map[config.Language]string{
		config.English: "serviceremove [-s SIGNER] RESOLVER_ENDPOINT TAG",
		config.Chinese: "serviceremove [-s 签署人] Resolver端点 标签",
	}
)

// _didServiceRemoveCmd represents service remove command
var _didServiceRemoveCmd = &cobra.Command{
	Use:   config.TranslateInLang(_serviceremoveCmdUses, config.UILanguage),
	Short: config.TranslateInLang(_serviceremoveCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := removeService(args)
		return output.PrintError(err)
	},
}

func init() {
	action.RegisterWriteCommand(_didServiceRemoveCmd)
}

func removeService(args []string) error {
	endpoint := args[0]

	signature, _, addr, err := signPermit(endpoint)
	if err != nil {
		return err
	}

	serviceReq := &ServiceRemoveRequest{
		Signature: *signature,
		Tag:       args[1],
	}
	serviceBytes, err := json.Marshal(&serviceReq)
	if err != nil {
		return output.NewError(output.ConvertError, "failed to encode request", err)
	}
	return postToResolver(endpoint+"/did/"+addr+"/service/delete", serviceBytes)
}
