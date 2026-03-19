// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package account

import (
	"context"
	"fmt"
	"strconv"

	"github.com/grpc-ecosystem/go-grpc-middleware/util/metautils"
	"github.com/rodaine/table"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/status"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"

	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
	"github.com/iotexproject/iotex-core/v2/ioctl/util"
)

// Multi-language support
var (
	_actionsCmdShorts = map[config.Language]string{
		config.English: "Show the list of actions for an account",
		config.Chinese: "显示账户的操作列表",
	}
	_actionsCmdUses = map[config.Language]string{
		config.English: "actions (ALIAS|ADDRESS) [SKIP] [LIMIT]",
		config.Chinese: "actions (别名|地址) [SKIP] [LIMIT]",
	}
)

type actionResult struct {
	ActHash   string `json:"actHash"`
	BlkHeight uint64 `json:"blkHeight"`
	Sender    string `json:"sender"`
	GasLimit  uint64 `json:"gasLimit"`
	GasPrice  string `json:"gasPrice"`
	Nonce     uint64 `json:"nonce"`
}

type actionsMessage struct {
	Address string          `json:"address"`
	Count   int             `json:"count"`
	Actions []*actionResult `json:"actions"`
}

func (m *actionsMessage) String() string {
	if output.Format == "" {
		showFields := []interface{}{"ActHash", "BlkHeight", "Sender", "Nonce", "GasLimit", "GasPrice"}
		tb := table.New(showFields...)
		for _, a := range m.Actions {
			tb.AddRow(a.ActHash, a.BlkHeight, a.Sender, a.Nonce, a.GasLimit, a.GasPrice)
		}
		result := fmt.Sprintf("Total: %d\n", m.Count)
		result += fmt.Sprint(tb)
		return result
	}
	return output.FormatString(output.Result, m)
}

// _accountActionsCmd represents the account sign command
var _accountActionsCmd = &cobra.Command{
	Use:   config.TranslateInLang(_actionsCmdUses, config.UILanguage),
	Short: config.TranslateInLang(_actionsCmdShorts, config.UILanguage),
	Args:  cobra.RangeArgs(1, 3),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := accountActions(args)
		return output.PrintError(err)
	},
}

func accountActions(args []string) error {
	var start, count uint64 = 0, 20
	var err error
	if len(args) >= 2 {
		start, err = strconv.ParseUint(args[1], 10, 64)
		if err != nil {
			return output.NewError(output.ConvertError, "failed to convert skip", nil)
		}
	}
	if len(args) >= 3 {
		count, err = strconv.ParseUint(args[2], 10, 64)
		if err != nil {
			return output.NewError(output.ConvertError, "failed to convert limit", nil)
		}
	}

	addr, err := util.Address(args[0])
	if err != nil {
		return output.NewError(output.AddressError, "failed to get address", err)
	}

	conn, err := util.ConnectToEndpoint(config.ReadConfig.SecureConnect && !config.Insecure)
	if err != nil {
		return output.NewError(output.NetworkError, "failed to connect to endpoint", err)
	}
	defer conn.Close()

	cli := iotexapi.NewAPIServiceClient(conn)
	ctx := context.Background()
	jwtMD, err := util.JwtAuth()
	if err == nil {
		ctx = metautils.NiceMD(jwtMD).ToOutgoing(ctx)
	}

	request := &iotexapi.GetActionsRequest{
		Lookup: &iotexapi.GetActionsRequest_ByAddr{
			ByAddr: &iotexapi.GetActionsByAddressRequest{
				Address: addr,
				Start:   start,
				Count:   count,
			},
		},
	}
	response, err := cli.GetActions(ctx, request)
	if err != nil {
		sta, ok := status.FromError(err)
		if ok {
			return output.NewError(output.APIError, sta.Message(), nil)
		}
		return output.NewError(output.NetworkError, "failed to invoke GetActions api", err)
	}

	actions := response.ActionInfo
	var results []*actionResult
	for _, a := range actions {
		result := &actionResult{
			ActHash:   a.ActHash,
			BlkHeight: a.BlkHeight,
			Sender:    a.Sender,
		}
		if a.Action != nil && a.Action.Core != nil {
			result.GasLimit = a.Action.Core.GasLimit
			result.GasPrice = a.Action.Core.GasPrice
			result.Nonce = a.Action.Core.Nonce
		}
		results = append(results, result)
	}

	message := actionsMessage{
		Address: addr,
		Count:   len(results),
		Actions: results,
	}
	fmt.Println(message.String())
	return nil
}
