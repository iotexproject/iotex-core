// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"context"
	"fmt"
	"math/big"
	"strconv"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/iotexproject/iotex-core/ioctl/cmd/alias"
	"github.com/iotexproject/iotex-core/ioctl/cmd/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/pkg/log"
)

// actionHashCmd represents the action hash command
var actionHashCmd = &cobra.Command{
	Use:   "hash ACTION_HASH",
	Short: "Get action by hash",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		output, err := getActionByHash(args)
		if err == nil {
			fmt.Println(output)
		}
		return err
	},
}

// getActionByHash gets action of IoTeX Blockchain by hash
func getActionByHash(args []string) (string, error) {
	hash := args[0]
	conn, err := util.ConnectToEndpoint(config.ReadConfig.SecureConnect && !config.Insecure)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	cli := iotexapi.NewAPIServiceClient(conn)
	ctx := context.Background()

	// search action on blockchain
	requestGetActionByHash := &iotexapi.GetActionByHashRequest{
		ActionHash:   hash,
		CheckPending: false,
	}
	requestGetAction := iotexapi.GetActionsRequest{
		Lookup: &iotexapi.GetActionsRequest_ByHash{
			ByHash: requestGetActionByHash,
		},
	}
	response, err := cli.GetActions(ctx, &requestGetAction)
	if err != nil {
		// search action in action pool
		requestGetActionByHash.CheckPending = true
		response, err = cli.GetActions(ctx, &requestGetAction)
		if err != nil {
			return "", err
		}
	}
	if len(response.ActionInfo) == 0 {
		return "", fmt.Errorf("No action info returned")
	}
	output, err := printAction(response.ActionInfo[0])
	if err != nil {
		return "", err
	}
	fmt.Println(output)

	requestGetReceipt := &iotexapi.GetReceiptByActionRequest{ActionHash: hash}
	responseReceipt, err := cli.GetReceiptByAction(ctx, requestGetReceipt)
	if err != nil {
		sta, ok := status.FromError(err)
		if ok && sta.Code() == codes.NotFound {
			return "\n#This action is pending", nil
		} else if ok {
			return "", fmt.Errorf(sta.Message())
		}
		return "", err
	}
	return "\n#This action has been written on blockchain\n" +
		printReceiptProto(responseReceipt.ReceiptInfo.Receipt), nil
}

func printAction(actionInfo *iotexapi.ActionInfo) (string, error) {
	output, err := printActionProto(actionInfo.Action)
	if err != nil {
		return "", err
	}
	if actionInfo.Timestamp != nil {
		ts, err := ptypes.Timestamp(actionInfo.Timestamp)
		if err != nil {
			return "", err
		}
		output += fmt.Sprintf("timeStamp: %d\n", ts.Unix())
		output += fmt.Sprintf("blkHash: %s\n", actionInfo.BlkHash)
	}
	output += fmt.Sprintf("actHash: %s\n", actionInfo.ActHash)
	return output, nil
}

func printActionProto(action *iotextypes.Action) (string, error) {
	pubKey, err := crypto.BytesToPublicKey(action.SenderPubKey)
	if err != nil {
		log.L().Error("failed to convert pubkey", zap.Error(err))
		return "", err
	}
	senderAddress, err := address.FromBytes(pubKey.Hash())
	if err != nil {
		log.L().Error("failed to convert bytes into address", zap.Error(err))
		return "", err
	}
	output := fmt.Sprintf("\nversion: %d  ", action.Core.GetVersion()) +
		fmt.Sprintf("nonce: %d  ", action.Core.GetNonce()) +
		fmt.Sprintf("gasLimit: %d  ", action.Core.GasLimit) +
		fmt.Sprintf("gasPrice: %s Rau\n", action.Core.GasPrice) +
		fmt.Sprintf("senderAddress: %s %s\n", senderAddress.String(),
			Match(senderAddress.String(), "address"))
	switch {
	default:
		output += proto.MarshalTextString(action.Core)
	case action.Core.GetTransfer() != nil:
		transfer := action.Core.GetTransfer()
		amount, err := util.StringToIOTX(transfer.Amount)
		if err != nil {
			return "", err
		}
		output += "transfer: <\n" +
			fmt.Sprintf("  recipient: %s %s\n", transfer.Recipient,
				Match(transfer.Recipient, "address")) +
			fmt.Sprintf("  amount: %s IOTX\n", amount)
		if len(transfer.Payload) != 0 {
			output += fmt.Sprintf("  payload: %s\n", transfer.Payload)
		}
		output += ">\n"
	case action.Core.GetExecution() != nil:
		execution := action.Core.GetExecution()
		output += "execution: <\n" +
			fmt.Sprintf("  contract: %s %s\n", execution.Contract,
				Match(execution.Contract, "address"))
		if execution.Amount != "0" {
			output += fmt.Sprintf("  amount: %s Rau\n", execution.Amount)
		}
		output += fmt.Sprintf("  data: %x\n", execution.Data) + ">\n"
	case action.Core.GetPutPollResult() != nil:
		putPollResult := action.Core.GetPutPollResult()
		output += "putPollResult: <\n" +
			fmt.Sprintf("  height: %d\n", putPollResult.Height) +
			"  candidates: <\n"
		for _, candidate := range putPollResult.Candidates.Candidates {
			output += "    candidate: <\n" +
				fmt.Sprintf("      address: %s\n", candidate.Address)
			votes := big.NewInt(0).SetBytes(candidate.Votes)
			output += fmt.Sprintf("      votes: %s\n", votes.String()) +
				fmt.Sprintf("      rewardAdress: %s\n", candidate.RewardAddress) +
				"    >\n"
		}
		output += "  >\n" +
			">\n"
	}
	output += fmt.Sprintf("senderPubKey: %x\n", action.SenderPubKey) +
		fmt.Sprintf("signature: %x\n", action.Signature)

	return output, nil
}

func printReceiptProto(receipt *iotextypes.Receipt) string {
	output := fmt.Sprintf("status: %d %s\n", receipt.Status,
		Match(strconv.Itoa(int(receipt.Status)), "status")) +
		fmt.Sprintf("actHash: %x\n", receipt.ActHash) +
		fmt.Sprintf("blkHeight: %d\n", receipt.BlkHeight) +
		fmt.Sprintf("gasConsumed: %d\n", receipt.GasConsumed) +
		fmt.Sprintf("logs: %d", len(receipt.Logs))
	if len(receipt.ContractAddress) != 0 {
		output += fmt.Sprintf("\ncontractAddress: %s %s", receipt.ContractAddress,
			Match(receipt.ContractAddress, "address"))
	}
	return output
}

// Match returns human readable expression
func Match(in string, matchType string) string {
	switch matchType {
	case "address":
		alias, err := alias.Alias(in)
		if err != nil {
			return ""
		}
		return "(" + alias + ")"
	case "status":
		if in == "0" {
			return "(Failure)"
		} else if in == "1" {
			return "(Success)"
		}
	}
	return ""
}
