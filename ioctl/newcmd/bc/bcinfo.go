// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package bc

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

// Multi-language support
var (
	bcInfoCmdShorts = map[ioctl.Language]string{
		ioctl.English: "Get current block chain information",
		ioctl.Chinese: "获取当前区块链信息",
	}
)

type infoMessage struct {
	Node string                `json:"node"`
	Info *iotextypes.ChainMeta `json:"info"`
}

func (m *infoMessage) String() string {
	if output.Format == "" {
		message := fmt.Sprintf("Blockchain Node: %s\n%s", m.Node, output.JSONString(m.Info))
		return message
	}
	return output.FormatString(output.Result, m)
}

// NewBCInfoCmd represents the bc info command
func NewBCInfoCmd(client ioctl.Client) *cobra.Command {
	bcInfoCmdShort, _ := client.SelectTranslation(bcInfoCmdShorts)

	var endpoint string
	var insecure bool

	cmd := &cobra.Command{
		Use:   "info",
		Short: bcInfoCmdShort,
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			apiServiceClient, err := client.APIServiceClient(ioctl.APIServiceConfig{
				Endpoint: endpoint,
				Insecure: insecure,
			})
			if err != nil {
				return err
			}

			chainMetaResponse, err := apiServiceClient.GetChainMeta(context.Background(), &iotexapi.GetChainMetaRequest{})
			if err != nil {
				return err
			}

			message := infoMessage{Node: client.Config().Endpoint, Info: chainMetaResponse.ChainMeta}
			fmt.Println(message.String())
			return nil
		},
	}

	flagEndpointUsage, _ := client.SelectTranslation(flagEndpointUsages)
	flagInsecureUsage, _ := client.SelectTranslation(flagInsecureUsages)

	cmd.PersistentFlags().StringVar(&endpoint, "endpoint", client.Config().Endpoint, flagEndpointUsage)
	cmd.PersistentFlags().BoolVar(&insecure, "insecure", !client.Config().SecureConnect, flagInsecureUsage)

	return cmd
}
