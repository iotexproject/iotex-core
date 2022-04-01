// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/hex"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

// Multi-language support
var (
	_sendRawCmdShorts = map[config.Language]string{
		config.English: "Send raw action on IoTeX blokchain",
		config.Chinese: "在IoTeX区块链上发送原始行为",
	}
	_sendRawCmdUses = map[config.Language]string{
		config.English: "sendraw DATA [-s SIGNER] [-n NONCE] [-l GAS_LIMIT] [-p GAS_PRICE] [-P PASSWORD] [-y]",
		config.Chinese: "sendraw 数据 [-s 签署人] [-n NONCE] [-l GAS限制] [-p GAS价格] [-P 密码] [-y]",
	}
)

// _actionSendRawCmd represents the action send raw transaction command
var _actionSendRawCmd = &cobra.Command{
	Use:   config.TranslateInLang(_sendRawCmdUses, config.UILanguage),
	Short: config.TranslateInLang(_sendRawCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := sendRaw(args[0])
		return output.PrintError(err)
	},
}

func init() {
	RegisterWriteCommand(_actionSendRawCmd)
}

func sendRaw(arg string) error {
	actBytes, err := hex.DecodeString(arg)
	if err != nil {
		return output.NewError(output.ConvertError, "failed to decode data", err)
	}
	act := &iotextypes.Action{}
	if err := proto.Unmarshal(actBytes, act); err != nil {
		return output.NewError(output.SerializationError, "failed to unmarshal data bytes", err)
	}
	return SendRaw(act)
}
