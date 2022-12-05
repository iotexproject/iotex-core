// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package jwt

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/iotexproject/iotex-antenna-go/v2/jwt"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/flag"
	"github.com/iotexproject/iotex-core/ioctl/newcmd/account"
	"github.com/iotexproject/iotex-core/ioctl/newcmd/action"
)

// Multi-language support
var (
	_signCmdShorts = map[config.Language]string{
		config.English: "Sign Json Web Token on IoTeX blockchain",
		config.Chinese: "签发IoTeX区块链上的JWT",
	}
	_signCmdUses = map[config.Language]string{
		config.English: "sign [-s SIGNER] [-P PASSWORD] [-y] --with-arguments [INVOKE_INPUT]",
		config.Chinese: "sign [-s 签署人] [-P 密码] [-y] --with-arguments [输入]",
	}
)

// NewJwtSignCmd represents the jwt sign command
func NewJwtSignCmd(client ioctl.Client) *cobra.Command {
	use, _ := client.SelectTranslation(_signCmdUses)
	short, _ := client.SelectTranslation(_signCmdShorts)

	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			var (
				input struct {
					Exp   string `json:"exp,omitempty"`
					Sub   string `json:"sub,omitempty"`
					Scope string `json:"scope,omitempty"`
					Iat   string `json:"iat,omitempty"`
					Iss   string `json:"iss,omitempty"`
				}
				exp int64
				err error
			)

			arg := flag.WithArgumentsFlag.Value().(string)
			if arg == "" {
				arg = "{}"
			}
			if err = json.Unmarshal([]byte(arg), &input); err != nil {
				return errors.Wrap(err, "failed to unmarshal arguments")
			}
			if input.Exp != "" {
				exp, err = strconv.ParseInt(input.Exp, 10, 64)
				if err != nil {
					return errors.Wrap(err, "invalid expire time")
				}
			} else {
				input.Exp = "0"
			}

			signer, err := cmd.Flags().GetString("signer")
			if err != nil {
				return errors.Wrap(err, "failed to get flag signer")
			}
			signer, err = action.Signer(client, signer)
			if err != nil {
				return errors.Wrap(err, "failed to get signer address")
			}
			prvKey, err := account.PrivateKeyFromSigner(client, cmd, signer, "")
			if err != nil {
				return err
			}
			pubKey := prvKey.PublicKey()
			addr := pubKey.Address()

			// sign JWT
			now := time.Now().Unix()
			jwtString, err := jwt.SignJWT(now, exp, input.Sub, input.Scope, prvKey)
			if err != nil {
				return errors.Wrap(err, "failed to sign JWT token")
			}
			prvKey.Zero()

			// format output
			input.Iat = strconv.FormatInt(now, 10)
			input.Iss = "0x" + pubKey.HexString()
			byteAsJSON, err := json.MarshalIndent(input, "", "  ")
			if err != nil {
				return err
			}
			cmd.Printf("JWT token: %s\n\n"+
				"signed by:\n"+
				"{\n"+
				"  address: %s\n"+
				"  public key: %s\n"+
				"}\n"+
				"with following claims:\n"+
				"%s\n",
				jwtString, addr.String(), input.Iss, string(byteAsJSON))
			return nil
		},
	}
	action.RegisterWriteCommand(client, cmd)
	flag.WithArgumentsFlag.RegisterCommand(cmd)
	return cmd
}
