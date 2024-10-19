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
	_serviceaddCmdShorts = map[config.Language]string{
		config.English: "Add service to DID document using private key from wallet",
		config.Chinese: "用钱包中的私钥向DID document添加服务",
	}
	_serviceaddCmdUses = map[config.Language]string{
		config.English: "serviceadd [-s SIGNER] RESOLVER_ENDPOINT TAG TYPE SERVICE_ENDPOINT",
		config.Chinese: "serviceadd [-s 签署人] Resolver端点 标签 类型 服务端点",
	}
)

// _didServiceAddCmd represents service add command
var _didServiceAddCmd = &cobra.Command{
	Use:   config.TranslateInLang(_serviceaddCmdUses, config.UILanguage),
	Short: config.TranslateInLang(_serviceaddCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(4),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := addService(args)
		return output.PrintError(err)
	},
}

func init() {
	action.RegisterWriteCommand(_didServiceAddCmd)
}

func addService(args []string) error {
	endpoint := args[0]

	signature, _, addr, err := signPermit(endpoint)
	if err != nil {
		return err
	}

	serviceReq := &ServiceAddRequest{
		Signature:       *signature,
		Tag:             args[1],
		Type:            args[2],
		ServiceEndpoint: args[3],
	}
	serviceBytes, err := json.Marshal(&serviceReq)
	if err != nil {
		return output.NewError(output.ConvertError, "failed to encode request", err)
	}
	return postToResolver(endpoint+"/did/"+addr+"/service", serviceBytes)
}
