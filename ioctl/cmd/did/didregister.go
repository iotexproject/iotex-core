// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package did

import (
	"encoding/hex"
	"encoding/json"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/v2/ioctl/cmd/action"
	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
)

// Multi-language support
var (
	_registerCmdUses = map[config.Language]string{
		config.English: "register (RESOLVER_ENDPOINT) [-s SIGNER]",
		config.Chinese: "register (Resolver端点) [-s 签署人]",
	}
	_registerCmdShorts = map[config.Language]string{
		config.English: "Register DID on IoTeX blockchain",
		config.Chinese: "Register 在IoTeX链上注册DID",
	}
)

// _didRegisterCmd represents the contract invoke register command
var _didRegisterCmd = &cobra.Command{
	Use:   config.TranslateInLang(_registerCmdUses, config.UILanguage),
	Short: config.TranslateInLang(_registerCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := registerDID(args)
		return output.PrintError(err)
	},
}

func init() {
	action.RegisterWriteCommand(_didRegisterCmd)
}

func registerDID(args []string) error {
	endpoint := args[0]

	signature, publicKey, _, err := signPermit(endpoint)
	if err != nil {
		return err
	}

	createReq := &CreateRequest{
		Signature: *signature,
		PublicKey: hex.EncodeToString(publicKey),
	}
	createBytes, err := json.Marshal(&createReq)
	if err != nil {
		return output.NewError(output.ConvertError, "failed to encode request", err)
	}

	return postToResolver(endpoint+"/did", createBytes)
}
