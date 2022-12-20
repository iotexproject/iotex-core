// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"strconv"

	protoV1 "github.com/golang/protobuf/proto"
	"github.com/grpc-ecosystem/go-grpc-middleware/util/metautils"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action/protocol/staking"
	"github.com/iotexproject/iotex-core/ioctl/cmd/alias"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
)

// Multi-language support
var (
	_hashCmdShorts = map[config.Language]string{
		config.English: "Get action by hash",
		config.Chinese: "依据哈希值，获取交易",
	}
	_hashCmdUses = map[config.Language]string{
		config.English: "hash ACTION_HASH",
		config.Chinese: "hash 交易哈希",
	}
)

// _actionHashCmd represents the action hash command
var _actionHashCmd = &cobra.Command{
	Use:   config.TranslateInLang(_hashCmdUses, config.UILanguage),
	Short: config.TranslateInLang(_hashCmdShorts, config.UILanguage),
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := getActionByHash(args)
		return output.PrintError(err)
	},
}

type actionState int

const (
	// Pending action is in the action pool but not executed by blockchain
	Pending actionState = iota
	// Executed action has been run and recorded on blockchain
	Executed
)

type actionMessage struct {
	State   actionState          `json:"state"`
	Proto   *iotexapi.ActionInfo `json:"proto"`
	Receipt *iotextypes.Receipt  `json:"receipt"`
}

func (m *actionMessage) String() string {
	if output.Format == "" {
		message, err := printAction(m.Proto)
		if err != nil {
			log.Panic(err.Error())
		}
		if m.State == Pending {
			message += "\n#This action is pending"
		} else {
			message += "\n#This action has been written on blockchain\n\n" + printReceiptProto(m.Receipt)
		}
		return message
	}
	return output.FormatString(output.Result, m)
}

// getActionByHash gets action of IoTeX Blockchain by hash
func getActionByHash(args []string) error {
	hash := args[0]
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

	// search action on blockchain
	requestGetAction := iotexapi.GetActionsRequest{
		Lookup: &iotexapi.GetActionsRequest_ByHash{
			ByHash: &iotexapi.GetActionByHashRequest{
				ActionHash:   hash,
				CheckPending: false,
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
	message := actionMessage{Proto: response.ActionInfo[0]}

	requestGetReceipt := &iotexapi.GetReceiptByActionRequest{ActionHash: hash}
	responseReceipt, err := cli.GetReceiptByAction(ctx, requestGetReceipt)
	if err != nil {
		sta, ok := status.FromError(err)
		if ok && sta.Code() == codes.NotFound {
			message.State = Pending
		} else if ok {
			return output.NewError(output.APIError, sta.Message(), nil)
		}
		return output.NewError(output.NetworkError, "failed to invoke GetReceiptByAction api", err)
	}
	message.State = Executed
	message.Receipt = responseReceipt.ReceiptInfo.Receipt
	fmt.Println(message.String())
	return nil
}

func printAction(actionInfo *iotexapi.ActionInfo) (string, error) {
	result, err := printActionProto(actionInfo.Action)
	if err != nil {
		return "", err
	}
	if actionInfo.Timestamp != nil {
		if err := actionInfo.Timestamp.CheckValid(); err != nil {
			return "", err
		}
		ts := actionInfo.Timestamp.AsTime()
		result += fmt.Sprintf("timeStamp: %d\n", ts.Unix())
		result += fmt.Sprintf("blkHash: %s\n", actionInfo.BlkHash)
	}
	result += fmt.Sprintf("actHash: %s\n", actionInfo.ActHash)
	return result, nil
}

func printActionProto(action *iotextypes.Action) (string, error) {
	pubKey, err := crypto.BytesToPublicKey(action.SenderPubKey)
	if err != nil {
		return "", output.NewError(output.ConvertError, "failed to convert public key from bytes", err)
	}
	senderAddress := pubKey.Address()
	if senderAddress == nil {
		return "", output.NewError(output.ConvertError, "failed to convert bytes into address", nil)
	}
	//ioctl action should display IOTX unit instead Raul
	core := action.Core
	gasPriceUnitIOTX, err := util.StringToIOTX(core.GasPrice)
	if err != nil {
		return "", output.NewError(output.ConfigError, "failed to convert string to IOTX", err)
	}
	result := fmt.Sprintf("\nversion: %d  ", core.GetVersion()) +
		fmt.Sprintf("nonce: %d  ", core.GetNonce()) +
		fmt.Sprintf("gasLimit: %d  ", core.GasLimit) +
		fmt.Sprintf("gasPrice: %s IOTX  ", gasPriceUnitIOTX) +
		fmt.Sprintf("chainID: %d  ", core.GetChainID()) +
		fmt.Sprintf("encoding: %d\n", action.GetEncoding()) +
		fmt.Sprintf("senderAddress: %s %s\n", senderAddress.String(),
			Match(senderAddress.String(), "address"))
	switch {
	case core.GetTransfer() != nil:
		transfer := core.GetTransfer()
		amount, err := util.StringToIOTX(transfer.Amount)
		if err != nil {
			return "", output.NewError(output.ConvertError, "failed to convert string into IOTX amount", err)
		}
		result += "transfer: <\n" +
			fmt.Sprintf("  recipient: %s %s\n", transfer.Recipient,
				Match(transfer.Recipient, "address")) +
			fmt.Sprintf("  amount: %s IOTX\n", amount)
		if len(transfer.Payload) != 0 {
			result += fmt.Sprintf("  payload: %s\n", transfer.Payload)
		}
		result += ">\n"
	case core.GetExecution() != nil:
		execution := core.GetExecution()
		result += "execution: <\n" +
			fmt.Sprintf("  contract: %s %s\n", execution.Contract,
				Match(execution.Contract, "address"))
		if execution.Amount != "0" {
			amount, err := util.StringToIOTX(execution.Amount)
			if err != nil {
				return "", output.NewError(output.ConvertError, "failed to convert string into IOTX amount", err)
			}
			result += fmt.Sprintf("  amount: %s IOTX\n", amount)
		}
		result += fmt.Sprintf("  data: %x\n", execution.Data) + ">\n"
	case core.GetPutPollResult() != nil:
		putPollResult := core.GetPutPollResult()
		result += "putPollResult: <\n" +
			fmt.Sprintf("  height: %d\n", putPollResult.Height) +
			"  candidates: <\n"
		for _, candidate := range putPollResult.Candidates.Candidates {
			result += "    candidate: <\n" +
				fmt.Sprintf("      address: %s\n", candidate.Address)
			votes := big.NewInt(0).SetBytes(candidate.Votes)
			result += fmt.Sprintf("      votes: %s\n", votes.String()) +
				fmt.Sprintf("      rewardAdress: %s\n", candidate.RewardAddress) +
				"    >\n"
		}
		result += "  >\n" +
			">\n"
	default:
		result += protoV1.MarshalTextString(core)
	}
	result += fmt.Sprintf("senderPubKey: %x\n", action.SenderPubKey) +
		fmt.Sprintf("signature: %x\n", action.Signature)

	return result, nil
}

func printReceiptProto(receipt *iotextypes.Receipt) string {
	result := fmt.Sprintf("status: %d %s\n", receipt.Status,
		Match(strconv.Itoa(int(receipt.Status)), "status")) +
		fmt.Sprintf("actHash: %x\n", receipt.ActHash) +
		fmt.Sprintf("blkHeight: %d\n", receipt.BlkHeight) +
		fmt.Sprintf("gasConsumed: %d\n", receipt.GasConsumed) +
		printLogs(receipt.Logs)
	if len(receipt.ContractAddress) != 0 {
		result += fmt.Sprintf("\ncontractAddress: %s %s", receipt.ContractAddress,
			Match(receipt.ContractAddress, "address"))
	}
	if len(receipt.Logs) > 0 {
		if index, ok := staking.BucketIndexFromReceiptLog(receipt.Logs[0]); ok {
			result += fmt.Sprintf("\nbucket index: %d", index)
		}
	}
	if receipt.Status == uint64(iotextypes.ReceiptStatus_ErrExecutionReverted) {
		result += fmt.Sprintf("\nexecution revert reason: %s", receipt.ExecutionRevertMsg)
	}
	return result
}

func printLogs(logs []*iotextypes.Log) string {
	result := "logs:<\n"
	for _, l := range logs {
		result += "  <\n" +
			fmt.Sprintf("    contractAddress: %s\n", l.ContractAddress) +
			"    topics:<\n"
		for _, topic := range l.Topics {
			result += fmt.Sprintf("      %s\n", hex.EncodeToString(topic))
		}
		result += "    >\n"
		if len(l.Data) > 0 {
			result += fmt.Sprintf("    data: %s\n", hex.EncodeToString(l.Data))
		}
		result += "  >\n"

	}
	result += ">\n"
	return result
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
		switch in {
		case "0":
			return "(Failure)"
		case "1":
			return "(Success)"
		case "100":
			return "(Failure : Unknown)"
		case "101":
			return "(Failure : Execution out of gas)"
		case "102":
			return "(Failure : Deployment out of gas - not enough gas to store code)"
		case "103":
			return "(Failure : Max call depth exceeded)"
		case "104":
			return "(Failure : Contract address collision)"
		case "105":
			return "(Failure : No compatible interpreter)"
		case "106":
			return "(Failure : Execution reverted)"
		case "107":
			return "(Failure : Max code size exceeded)"
		case "108":
			return "(Failure : Write protection)"
		}
	}
	return ""
}
