package account

import (
	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/spf13/cobra"
)

var signer string

// Multi-language support
var (
	signCmdShorts = map[config.Language]string{
		config.English: "Sign message with private key from wallet",
		config.Chinese: "用钱包中的私钥对信息签名",
	}
	signCmdUses = map[config.Language]string{
		config.English: "sign MESSAGE [-s SIGNER]",
		config.Chinese: "sign 信息 [-s 签署人]",
	}
	flagSignerUsages = map[config.Language]string{
		config.English: "choose a signing account",
		config.Chinese: "选择一个签名账户",
	}
)

// NewAccountSign represents the account sign command
func NewAccountSign(client ioctl.Client) *cobra.Command {
	use, _ := client.SelectTranslation(signCmdUses)
	short, _ := client.SelectTranslation(signCmdShorts)
	usage, _ := client.SelectTranslation(flagSignerUsages)

	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			msg := args[0]

			var (
				addr string
				err  error
			)
			if util.AliasIsHdwalletKey(signer) {
				addr = signer
			} else {
				addr, err = util.Address(signer)
				if err != nil {
					return output.NewError(output.AddressError, "failed to get address", err)
				}
			}
			signedMessage, err := Sign(addr, "", msg)
			if err != nil {
				return output.NewError(output.KeystoreError, "failed to sign message", err)
			}
			output.PrintResult(signedMessage)
			return nil
		},
	}
	cmd.Flags().StringVarP(&signer, "signer", "s", "", usage)
	return cmd
}
