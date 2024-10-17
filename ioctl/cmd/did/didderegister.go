// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

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
	_deregisterCmdUses = map[config.Language]string{
		config.English: "deregister (RESOLVER_ENDPOINT) [-s SIGNER]",
		config.Chinese: "deregister (Resolver端点) [-s 签署人]",
	}
	_deregisterCmdShorts = map[config.Language]string{
		config.English: "Deregister DID on IoTeX blockchain",
		config.Chinese: "Deregister 在IoTeX链上注销DID",
	}
)

// _didDeregisterCmd represents the contract invoke deregister command
var _didDeregisterCmd = &cobra.Command{
	Use:   config.TranslateInLang(_deregisterCmdUses, config.UILanguage),
	Short: config.TranslateInLang(_deregisterCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := deregisterDID(args)
		return output.PrintError(err)
	},
}

func init() {
	action.RegisterWriteCommand(_didDeregisterCmd)
}

func deregisterDID(args []string) (err error) {
	endpoint := args[0]

	signature, _, addr, err := signPermit(endpoint)
	if err != nil {
		return err
	}

	deleteBytes, err := json.Marshal(&signature)
	if err != nil {
		return output.NewError(output.ConvertError, "failed to encode request", err)
	}

	return postToResolver(endpoint+"/did/"+addr+"/delete", deleteBytes)
}
