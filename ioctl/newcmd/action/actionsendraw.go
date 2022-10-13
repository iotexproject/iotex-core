// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.
package action

import (
	"encoding/hex"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
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

func NewActionSendRawCmd(client ioctl.Client) *cobra.Command {
	use, _ := client.SelectTranslation(_sendRawCmdUses)
	short, _ := client.SelectTranslation(_sendRawCmdShorts)

	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			actBytes, err := hex.DecodeString(args[0])
			if err != nil {
				return errors.Wrap(err, "failed to decode data")
			}
			act := &iotextypes.Action{}
			if err := proto.Unmarshal(actBytes, act); err != nil {
				return errors.Wrap(err, "failed to unmarshal data bytes")
			}
			return SendRaw(client, cmd, act)
		},
	}
}
