// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/rodaine/table"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

// Multi-language support
var (
	actionsCmdShorts = map[config.Language]string{
		config.English: "Show the list of actions for an account",
		config.Chinese: "显示账户的操作列表",
	}
	actionsCmdUses = map[config.Language]string{
		config.English: "actions (ALIAS|ADDRESS)  [SKIP]",
		config.Chinese: "actions (ALIAS|ADDRESS)  [SKIP]",
	}
)

// TODO: remove struct definition after the release of anlytics v2
type allActionsByAddressResult struct {
	ActHash    string
	BlkHeight  string
	Sender     string
	Recipient  string
	ActType    string
	Amount     string
	TimeStamp  string
	RecordType string
}

type allActionsByAddressResponse struct {
	Count   string
	Results []*allActionsByAddressResult
}

// accountActionsCmd represents the account sign command
var accountActionsCmd = &cobra.Command{
	Use:   config.TranslateInLang(actionsCmdUses, config.UILanguage),
	Short: config.TranslateInLang(actionsCmdShorts, config.UILanguage),
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := accountActions(args)
		return output.PrintError(err)
	},
}

func accountActions(args []string) error {
	var skip uint64 = 0
	var err error
	if len(args) == 2 {
		skip, err = strconv.ParseUint(args[1], 10, 64)
		if err != nil {
			return output.NewError(output.ConvertError, "failed to convert skip ", nil)
		}
	}

	addr, err := util.Address(args[0])
	if err != nil {
		return output.NewError(output.AddressError, "failed to get address", err)
	}
	reqData := map[string]string{
		"address": addr,
		"offset":  fmt.Sprint(skip),
	}
	jsonData, err := json.Marshal(reqData)
	if err != nil {
		return output.NewError(output.ConvertError, "failed to pack in json", nil)
	}
	resp, err := http.Post("https://iotexscout.io/apiproxy/api.ActionsService.GetAllActionsByAddress", "application/json",
		bytes.NewBuffer(jsonData))
	if err != nil {
		return output.NewError(output.NetworkError, "failed to send request", nil)
	}

	var respData allActionsByAddressResponse
	err = json.NewDecoder(resp.Body).Decode(&respData)
	if err != nil {
		return output.NewError(output.SerializationError, "failed to deserialize the response", nil)
	}
	actions := respData.Results

	fmt.Println("Total:", len(actions))
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
}
