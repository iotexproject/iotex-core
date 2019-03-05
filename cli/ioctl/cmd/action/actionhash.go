// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"context"
	"errors"
	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/config"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/protogen/iotexapi"
	"github.com/iotexproject/iotex-core/protogen/iotextypes"
)

// actionHashCmd represents the account balance command
var actionHashCmd = &cobra.Command{
	Use:   "hash actionhash",
	Short: "Get action by hash",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(getActionByHash(args))
	},
}

// getActionByHash gets balance of an IoTeX Blockchain address
func getActionByHash(args []string) string {
	hash := args[0]
	endpoint := config.GetEndpoint()
	if endpoint == config.ErrEmptyEndpoint {
		log.L().Error(config.ErrEmptyEndpoint)
		return "use \"ioctl config set endpoint\" to config endpoint first."
	}
	conn, err := grpc.Dial(endpoint, grpc.WithInsecure())
	if err != nil {
		log.L().Error("failed to connect to server", zap.Error(err))
		return err.Error()
	}
	defer conn.Close()
	cli := iotexapi.NewAPIServiceClient(conn)
	ctx := context.Background()
	requestCheckPending := iotexapi.GetActionsRequest{
		Lookup: &iotexapi.GetActionsRequest_ByHash{
			ByHash: &iotexapi.GetActionByHashRequest{
				ActionHash:   hash,
				CheckPending: true,
			},
		},
	}
	response, err := cli.GetActions(ctx, &requestCheckPending)
	if err != nil {
		log.L().Error("cannot get action from "+hash, zap.Error(err))
		return err.Error()
	}
	action := response.Actions[0]
	request := &iotexapi.GetActionsRequest{
		Lookup: &iotexapi.GetActionsRequest_ByHash{
			ByHash: &iotexapi.GetActionByHashRequest{
				ActionHash:   hash,
				CheckPending: false,
			},
		},
	}
	_, err = cli.GetActions(ctx, request)

	output, err := printActionProto(action)
	if err != nil {
		return err.Error()
	}
	if err != nil {
		return output + "\n#This action is pending\n"
	}
	if action.Core.GetTransfer() != nil {
		return output + "\n#This action has been writen on blockchain\n"
	}
	requestGetReceipt := &iotexapi.GetReceiptByActionRequest{ActionHash: hash}
	responseReciept, err := cli.GetReceiptByAction(ctx, requestGetReceipt)
	if err != nil {
		log.L().Error("cannot get reciept from "+hash, zap.Error(err))
		return err.Error()
	}

	return output + "\n#This action has been writen on blockchain\n" +
		printRecieptProto(responseReciept.Receipt)
}

func printActionProto(action *iotextypes.Action) (string, error) {
	switch {
	case action.Core.GetTransfer() != nil:
		return proto.MarshalTextString(action.Core) +
			fmt.Sprintf("senderPubKey: %x\n", action.SenderPubKey) +
			fmt.Sprintf("signature: %x\n", action.Signature), nil
	case action.Core.GetExecution() != nil:
		execution := action.Core.GetExecution()
		return fmt.Sprintf("version: %d\n", action.Core.GetVersion()) +
			fmt.Sprintf("nonce: %d\n", action.Core.GetNonce()) +
			fmt.Sprintf("gasLimit: %d\n", action.Core.GasLimit) +
			fmt.Sprintf("gasPrice: %s\n", action.Core.GasPrice) +
			"execution: <\n" +
			fmt.Sprintf("  contract: %s\n", execution.Contract) +
			fmt.Sprintf("  amount: %s\n", execution.Amount) +
			fmt.Sprintf("  data: %x\n", execution.Data) +
			">\n" +
			fmt.Sprintf("senderPubKey: %x\n", action.SenderPubKey) +
			fmt.Sprintf("signature: %x\n", action.Signature), nil
	}
	return "", errors.New("action can not match")
}

func printRecieptProto(reciept *iotextypes.Receipt) string {
	return fmt.Sprintf("returnValue %x\n", reciept.ReturnValue) +
		fmt.Sprintf("status: %d", reciept.Status) +
		fmt.Sprintf("actHash: %x\n", reciept.ActHash) +
		fmt.Sprintf("gasConsumed: %d\n", reciept.GasConsumed) +
		fmt.Sprintf("contractAddress: %s\n", reciept.ContractAddress)
}
