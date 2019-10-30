// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/hex"

	"github.com/golang/protobuf/proto"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/ioctl/output"
)

// actionSendRawCmd represents the action send raw transaction command
var actionSendRawCmd = &cobra.Command{
	Use:   "sendraw DATA [-s SIGNER] [-n NONCE] [-l GAS_LIMIT] [-p GAS_PRICE] [-P PASSWORD] [-y]",
	Short: "Send raw action on IoTeX blokchain",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := sendRaw(args[0])
		return output.PrintError(err)
	},
}

func init() {
	registerWriteCommand(actionSendRawCmd)
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
