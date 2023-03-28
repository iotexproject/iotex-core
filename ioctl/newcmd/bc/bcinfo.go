// Copyright (c) 2022 IoTeX
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package bc

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
)

// Multi-language support
var (
	_bcInfoCmdShorts = map[config.Language]string{
		config.English: "Get current block chain information",
		config.Chinese: "获取当前区块链信息",
	}
)

type infoMessage struct {
	Node string                `json:"node"`
	Info *iotextypes.ChainMeta `json:"info"`
}

// NewBCInfoCmd represents the bc info command
func NewBCInfoCmd(client ioctl.Client) *cobra.Command {
	bcInfoCmdShort, _ := client.SelectTranslation(_bcInfoCmdShorts)
	return &cobra.Command{
		Use:   "info",
		Short: bcInfoCmdShort,
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			chainMeta, err := GetChainMeta(client)
			if err != nil {
				return err
			}

			message := infoMessage{Node: client.Config().Endpoint, Info: chainMeta}
			cmd.Println(fmt.Sprintf("Blockchain Node: %s\n%s", message.Node, JSONString(message.Info)))
			return nil
		},
	}
}

// JSONString returns json string for message
func JSONString(out interface{}) string {
	byteAsJSON, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		log.Panic(err)
	}
	return fmt.Sprint(string(byteAsJSON))
}
