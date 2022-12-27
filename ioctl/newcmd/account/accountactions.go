// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package account

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/pkg/errors"
	"github.com/rodaine/table"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
)

type (
	// allActionsByAddressResult is the struct of an Confirmation output
	allActionsByAddressResult struct {
		ActHash    string
		BlkHeight  string
		Sender     string
		Recipient  string
		ActType    string
		Amount     string
		TimeStamp  string
		RecordType string
	}

	// allActionsByAddressResponse is the struct of an Confirmation output
	allActionsByAddressResponse struct {
		Count   string
		Results []*allActionsByAddressResult
	}
)

// Multi-language support
var (
	_actionsCmdShorts = map[config.Language]string{
		config.English: "Show the list of actions for an account",
		config.Chinese: "显示账户的操作列表",
	}
	_actionsCmdUses = map[config.Language]string{
		config.English: "actions [ALIAS|ADDRESS] [SKIP]",
		config.Chinese: "actions [别名|地址] [SKIP]",
	}
)

// NewAccountActions represents the account sign command
func NewAccountActions(client ioctl.Client) *cobra.Command {
	use, _ := client.SelectTranslation(_actionsCmdUses)
	short, _ := client.SelectTranslation(_actionsCmdShorts)
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			var skip uint64 = 0
			var err error
			if len(args) == 2 {
				skip, err = strconv.ParseUint(args[1], 10, 64)
				if err != nil {
					return errors.Wrap(err, "failed to convert skip ")
				}
			}

			addr, err := client.Address(args[0])
			if err != nil {
				return errors.Wrap(err, "failed to get address")
			}
			reqData := map[string]string{
				"address": addr,
				"offset":  fmt.Sprint(skip),
			}
			resp, err := client.QueryAnalyser(reqData)
			if err != nil {
				return err
			}

			var respData allActionsByAddressResponse
			err = json.NewDecoder(resp.Body).Decode(&respData)
			if err != nil {
				return errors.Wrap(err, "failed to deserialize the response")
			}
			actions := respData.Results

			cmd.Println("Total:", len(actions))
			showFields := []interface{}{
				"ActHash",
				"TimeStamp",
				"BlkHeight",
				"ActCategory",
				"ActType",
				"Sender",
				"Recipient",
				"Amount",
			}
			tb := table.New(showFields...)
			for _, actionInfo := range actions {
				tb.AddRow(
					actionInfo.ActHash,
					actionInfo.TimeStamp,
					actionInfo.BlkHeight,
					actionInfo.RecordType,
					actionInfo.ActType,
					actionInfo.Sender,
					actionInfo.Recipient,
					actionInfo.Amount+" IOTX",
				)
			}
			tb.Print()
			return nil
		},
	}
}
