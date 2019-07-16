// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/hex"

	"github.com/golang/protobuf/proto"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/spf13/cobra"
)

// actionSendRawCmd represents the action send raw transaction command
var actionSendRawCmd = &cobra.Command{
	Use:   "sendraw DATA",
	Short: "Send raw action on IoTeX blokchain",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		actBytes, err := hex.DecodeString(args[0])
		if err != nil {
			return err
		}
		act := &iotextypes.Action{}
		if err := proto.Unmarshal(actBytes, act); err != nil {
			return err
		}
		return sendRaw(act)
	},
}

func init() {
	registerWriteCommand(actionSendRawCmd)
}
