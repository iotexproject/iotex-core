// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/golang/protobuf/ptypes"
	"github.com/grpc-ecosystem/go-grpc-middleware/util/metautils"
	"github.com/rodaine/table"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/status"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

var countLimit *uint64

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
	flagCountUsages = map[config.Language]string{
		config.English: "choose a count limit",
		config.Chinese: "选择一个计数限制",
	}
)

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

func init() {
	countLimit = accountActionsCmd.Flags().Uint64("limit", 15, config.TranslateInLang(flagCountUsages, config.UILanguage))
}

func accountActions(args []string) error {
	var skip uint64
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

	requestGetAccount := iotexapi.GetAccountRequest{
		Address: addr,
	}
	accountResponse, err := cli.GetAccount(ctx, &requestGetAccount)
	if err != nil {
		return output.NewError(output.APIError, "failed to get account", err)
	}
	numActions := accountResponse.AccountMeta.GetNumActions()
	fmt.Println("Total:", numActions)
	requestGetAction := iotexapi.GetActionsRequest{
		Lookup: &iotexapi.GetActionsRequest_ByAddr{
			ByAddr: &iotexapi.GetActionsByAddressRequest{
				Address: addr,
				Start:   numActions - *countLimit - skip,
				Count:   *countLimit,
			},
		},
	}
	response, err := cli.GetActions(ctx, &requestGetAction)
	if err != nil {
		sta, ok := status.FromError(err)
		if ok {
			return output.NewError(output.APIError, sta.Message(), nil)
		}
		return output.NewError(output.NetworkError, "failed to invoke GetActions api", err)
	}
	if len(response.ActionInfo) == 0 {
		return output.NewError(output.APIError, "no action info returned", nil)
	}
	showFields := []interface{}{
		"Hash",
		"Time",
		"Status",
		"Sender",
		"Type",
		"To",
		"Contract",
		"Amount",
	}
	tbl := table.New(showFields...)

	for i := range response.ActionInfo {
		k := len(response.ActionInfo) - 1 - i
		actionInfo := response.ActionInfo[k]
		requestGetReceipt := &iotexapi.GetReceiptByActionRequest{ActionHash: actionInfo.ActHash}
		responseReceipt, err := cli.GetReceiptByAction(ctx, requestGetReceipt)
		if err != nil {
			sta, ok := status.FromError(err)
			if ok {
				fmt.Println(output.NewError(output.APIError, sta.Message(), nil))
			} else {
				fmt.Println(output.NewError(output.NetworkError, "failed to invoke GetReceiptByAction api", err))
			}
			continue
		}
		amount := "0"
		transfer := actionInfo.Action.Core.GetTransfer()
		if transfer != nil {
			amount, _ = util.StringToIOTX(transfer.Amount)

		}
		tbl.AddRow(
			actionInfo.ActHash,
			getActionTime(actionInfo),
			iotextypes.ReceiptStatus_name[int32(responseReceipt.ReceiptInfo.Receipt.GetStatus())],
			actionInfo.Sender,
			getActionTypeString(actionInfo),
			getActionTo(actionInfo),
			getActionContract(responseReceipt),
			amount+" IOTX",
		)
	}
	tbl.Print()
	return nil
}

func getActionContract(responseReceipt *iotexapi.GetReceiptByActionResponse) string {
	contract := responseReceipt.ReceiptInfo.Receipt.GetContractAddress()
	if contract == "" {
		contract = "-"
	}
	return contract
}

func getActionTypeString(actionInfo *iotexapi.ActionInfo) string {
	actionType := fmt.Sprintf("%T", actionInfo.Action.Core.GetAction())
	return strings.TrimLeft(actionType, "*iotextypes.ActionCore_")
}

func getActionTo(actionInfo *iotexapi.ActionInfo) string {
	recipient := ""
	switch getActionTypeString(actionInfo) {
	case "Transfer":
		transfer := actionInfo.Action.Core.GetTransfer()
		recipient = transfer.GetRecipient()
	case "Execution":
		execution := actionInfo.Action.Core.GetExecution()
		recipient = execution.GetContract()
	}
	if recipient == "" {
		recipient = "-"
	}
	return recipient
}

func getActionTime(actionInfo *iotexapi.ActionInfo) string {
	if actionInfo.Timestamp != nil {
		if ts, err := ptypes.Timestamp(actionInfo.Timestamp); err == nil {
			return ts.String()
		}
	}
	return ""
}
