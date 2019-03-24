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
	"strconv"

	"github.com/golang/protobuf/proto"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/alias"
	"github.com/iotexproject/iotex-core/cli/ioctl/util"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/protogen/iotexapi"
	"github.com/iotexproject/iotex-core/protogen/iotextypes"
)

// actionHashCmd represents the action hash command
var actionHashCmd = &cobra.Command{
	Use:   "hash ACTION_HASH",
	Short: "Get action by hash",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(getActionByHash(args))
	},
}

// getActionByHash gets action of IoTeX Blockchain by hash
func getActionByHash(args []string) string {
	hash := args[0]
	conn, err := util.ConnectToEndpoint()
	if err != nil {
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
		return err.Error()
	}
	action := response.ActionInfo[0]
	output, err := printActionProto(action.Action)
	if err != nil {
		return err.Error()
	}

	request := &iotexapi.GetActionsRequest{
		Lookup: &iotexapi.GetActionsRequest_ByHash{
			ByHash: &iotexapi.GetActionByHashRequest{
				ActionHash:   hash,
				CheckPending: false,
			},
		},
	}
	_, err = cli.GetActions(ctx, request)
	if err != nil {
		return output + "\n#This action is pending\n"
	}

	requestGetReceipt := &iotexapi.GetReceiptByActionRequest{ActionHash: hash}
	responseReceipt, err := cli.GetReceiptByAction(ctx, requestGetReceipt)
	if err != nil {
		return err.Error()
	}
	return output + "\n#This action has been written on blockchain\n" +
		printReceiptProto(responseReceipt.Receipt)
}

func printActionProto(action *iotextypes.Action) (string, error) {
	pubKey, err := keypair.BytesToPublicKey(action.SenderPubKey)
	if err != nil {
		log.L().Error("failed to convert pubkey", zap.Error(err))
		return "", err
	}
	senderAddress, err := address.FromBytes(pubKey.Hash())
	if err != nil {
		log.L().Error("failed to convert bytes into address", zap.Error(err))
		return "", err
	}
	switch {
	case action.Core.GetTransfer() != nil:
		transfer := action.Core.GetTransfer()
		return fmt.Sprintf("\nversion: %d  ", action.Core.GetVersion()) +
			fmt.Sprintf("nonce: %d  ", action.Core.GetNonce()) +
			fmt.Sprintf("gasLimit: %d  ", action.Core.GasLimit) +
			fmt.Sprintf("gasPrice: %s Rau\n", action.Core.GasPrice) +
			fmt.Sprintf("senderAddress: %s %s\n", senderAddress.String(),
				match(senderAddress.String(), "address")) +
			"transfer: <\n" +
			fmt.Sprintf("  recipient: %s %s\n", transfer.Recipient,
				match(transfer.Recipient, "address")) +
			fmt.Sprintf("  amount: %s Rau\n", transfer.Amount) +
			fmt.Sprintf("  payload: %s\n", transfer.Payload) +
			">\n" +
			fmt.Sprintf("senderPubKey: %x\n", action.SenderPubKey) +
			fmt.Sprintf("signature: %x\n", action.Signature), nil
	case action.Core.GetExecution() != nil:
		execution := action.Core.GetExecution()
		return fmt.Sprintf("\nversion: %d  ", action.Core.GetVersion()) +
			fmt.Sprintf("nonce: %d  ", action.Core.GetNonce()) +
			fmt.Sprintf("gasLimit: %d  ", action.Core.GasLimit) +
			fmt.Sprintf("gasPrice: %s Rau\n", action.Core.GasPrice) +
			fmt.Sprintf("senderAddress: %s %s\n", senderAddress.String(),
				match(senderAddress.String(), "address")) +
			"execution: <\n" +
			fmt.Sprintf("  contract: %s %s\n", execution.Contract,
				match(execution.Contract, "address")) +
			fmt.Sprintf("  amount: %s Rau\n", execution.Amount) +
			fmt.Sprintf("  data: %x\n", execution.Data) +
			">\n" +
			fmt.Sprintf("senderPubKey: %x\n", action.SenderPubKey) +
			fmt.Sprintf("signature: %x\n", action.Signature), nil
	case action.Core.GetClaimFromRewardingFund() != nil:
		return fmt.Sprintf("\nversion: %d  ", action.Core.GetVersion()) +
			fmt.Sprintf("nonce: %d  ", action.Core.GetNonce()) +
			fmt.Sprintf("gasLimit: %d  ", action.Core.GasLimit) +
			fmt.Sprintf("gasPrice: %s Rau\n", action.Core.GasPrice) +
			fmt.Sprintf("senderAddress: %s %s\n", senderAddress.String(),
				match(senderAddress.String(), "address")) +
			proto.MarshalTextString(action.Core) +
			fmt.Sprintf("senderPubKey: %x\n", action.SenderPubKey) +
			fmt.Sprintf("signature: %x\n", action.Signature), nil
	}
	return "", errors.New("action can not match")
}

func printReceiptProto(receipt *iotextypes.Receipt) string {
	lines := make([]string, 0)
	for _, l := range receipt.Logs {
		line := fmt.Sprintf("#%d block:%d txHash:%s address:%s data:%s\n",
			l.Index, l.BlockNumber, l.TxnHash, l.Address, l.Data)
		for _, t := range l.Topics {
			line += fmt.Sprintf("  %s\n", t)
		}
		lines = append(lines, line)
	}
	return fmt.Sprintf("returnValue: %x\n", receipt.ReturnValue) +
		fmt.Sprintf("status: %d %s\n", receipt.Status,
			match(strconv.Itoa(int(receipt.Status)), "status")) +
		fmt.Sprintf("actHash: %x\n", receipt.ActHash) +
		// TODO: blkHash
		fmt.Sprintf("gasConsumed: %d\n", receipt.GasConsumed) +
		fmt.Sprintf("contractAddress: %s %s\n", receipt.ContractAddress,
			match(receipt.ContractAddress, "address")) +
		fmt.Sprintf("logs:\n%s", lines)
}

func match(in string, matchType string) string {
	switch matchType {
	case "address":
		alias, err := alias.Alias(in)
		if err != nil {
			return ""
		}
		return "(" + alias + ")"
	case "status":
		if in == "0" {
			return "(Fail)"
		} else if in == "1" {
			return "(Success)"
		}
	}
	return ""
}
