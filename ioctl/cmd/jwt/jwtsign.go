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
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/cmd/account"
	"github.com/iotexproject/iotex-core/ioctl/cmd/action"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/flag"
	"github.com/iotexproject/iotex-core/ioctl/output"
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

// _jwtSignCmd represents the jwt sign command
var _jwtSignCmd = &cobra.Command{
	Use:   config.TranslateInLang(_signCmdUses, config.UILanguage),
	Short: config.TranslateInLang(_signCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		return output.PrintError(jwtSign())
	},
}

func jwtSign() error {
	arg := flag.WithArgumentsFlag.Value().(string)
	if arg == "" {
		arg = "{}"
	}

	var (
		input map[string]string
		exp   int64
		err   error
	)
	if err = json.Unmarshal([]byte(arg), &input); err != nil {
		return output.NewError(output.SerializationError, "failed to unmarshal arguments", err)
	}
	if input["exp"] != "" {
		exp, err = strconv.ParseInt(input["exp"], 10, 64)
		if err != nil {
			return output.NewError(output.SerializationError, "invalid expire time", err)
		}
	} else {
		input["exp"] = "0"
	}

	signer, err := action.Signer()
	if err != nil {
		return output.NewError(output.AddressError, "failed to get signer address", err)
	}
	prvKey, err := account.PrivateKeyFromSigner(signer, "")
	if err != nil {
		return err
	}
	pubKey := prvKey.PublicKey()
	addr := pubKey.Address()

	// sign JWT
	now := time.Now().Unix()
	jwtString, err := jwt.SignJWT(now, exp, input["sub"], input["scope"], prvKey)
	if err != nil {
		return output.NewError(output.CryptoError, "failed to sign JWT token", err)
	}
	prvKey.Zero()

	// format output
	input["iat"] = strconv.FormatInt(now, 10)
	input["iss"] = "0x" + pubKey.HexString()
	out := "JWT token: " + jwtString + "\n\n"
	out += "signed by:\n"
	out += "{\n"
	out += "  address: " + addr.String() + "\n"
	out += "  public key: " + input["iss"] + "\n"
	out += "}\n"
	out += "with following claims:\n"
	out += output.JSONString(input)
	output.PrintResult(out)
	return nil
}
