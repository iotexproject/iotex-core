// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package xrc20

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"syscall"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/golang/protobuf/proto"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh/terminal"
	"google.golang.org/grpc/status"

	"github.com/iotexproject/go-pkgs/byteutil"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/account"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/alias"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/config"
	"github.com/iotexproject/iotex-core/cli/ioctl/util"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

var (
	signer          string
	contractAddress string
	gasLimit        uint64
	gasPrice        string
	nonce           uint64
	bytecodeString  string
	ownerAddress    address.Address
	transferAmount  uint64
	targetAddress   address.Address
	spenderAddress  address.Address
	abiResult       abi.ABI
)

// Xrc20Cmd represents the account command
var Xrc20Cmd = &cobra.Command{
	Use:   "xrc20",
	Short: "Supporting ERC20 standard command-line from ioctl",
}

const abiConst = "[\r\n\t{\r\n\t\t\"constant\": false,\r\n\t\t\"inputs\": [\r\n\t\t\t{\r\n\t\t\t\t\"name\": \"spender\",\r\n\t\t\t\t\"type\": \"address\"\r\n\t\t\t},\r\n\t\t\t{\r\n\t\t\t\t\"name\": \"value\",\r\n\t\t\t\t\"type\": \"uint256\"\r\n\t\t\t}\r\n\t\t],\r\n\t\t\"name\": \"approve\",\r\n\t\t\"outputs\": [\r\n\t\t\t{\r\n\t\t\t\t\"name\": \"\",\r\n\t\t\t\t\"type\": \"bool\"\r\n\t\t\t}\r\n\t\t],\r\n\t\t\"payable\": false,\r\n\t\t\"stateMutability\": \"nonpayable\",\r\n\t\t\"type\": \"function\"\r\n\t},\r\n\t{\r\n\t\t\"constant\": true,\r\n\t\t\"inputs\": [],\r\n\t\t\"name\": \"totalSupply\",\r\n\t\t\"outputs\": [\r\n\t\t\t{\r\n\t\t\t\t\"name\": \"\",\r\n\t\t\t\t\"type\": \"uint256\"\r\n\t\t\t}\r\n\t\t],\r\n\t\t\"payable\": false,\r\n\t\t\"stateMutability\": \"view\",\r\n\t\t\"type\": \"function\"\r\n\t},\r\n\t{\r\n\t\t\"constant\": false,\r\n\t\t\"inputs\": [\r\n\t\t\t{\r\n\t\t\t\t\"name\": \"from\",\r\n\t\t\t\t\"type\": \"address\"\r\n\t\t\t},\r\n\t\t\t{\r\n\t\t\t\t\"name\": \"to\",\r\n\t\t\t\t\"type\": \"address\"\r\n\t\t\t},\r\n\t\t\t{\r\n\t\t\t\t\"name\": \"value\",\r\n\t\t\t\t\"type\": \"uint256\"\r\n\t\t\t}\r\n\t\t],\r\n\t\t\"name\": \"transferFrom\",\r\n\t\t\"outputs\": [\r\n\t\t\t{\r\n\t\t\t\t\"name\": \"\",\r\n\t\t\t\t\"type\": \"bool\"\r\n\t\t\t}\r\n\t\t],\r\n\t\t\"payable\": false,\r\n\t\t\"stateMutability\": \"nonpayable\",\r\n\t\t\"type\": \"function\"\r\n\t},\r\n\t{\r\n\t\t\"constant\": true,\r\n\t\t\"inputs\": [\r\n\t\t\t{\r\n\t\t\t\t\"name\": \"who\",\r\n\t\t\t\t\"type\": \"address\"\r\n\t\t\t}\r\n\t\t],\r\n\t\t\"name\": \"balanceOf\",\r\n\t\t\"outputs\": [\r\n\t\t\t{\r\n\t\t\t\t\"name\": \"\",\r\n\t\t\t\t\"type\": \"uint256\"\r\n\t\t\t}\r\n\t\t],\r\n\t\t\"payable\": false,\r\n\t\t\"stateMutability\": \"view\",\r\n\t\t\"type\": \"function\"\r\n\t},\r\n\t{\r\n\t\t\"constant\": false,\r\n\t\t\"inputs\": [\r\n\t\t\t{\r\n\t\t\t\t\"name\": \"to\",\r\n\t\t\t\t\"type\": \"address\"\r\n\t\t\t},\r\n\t\t\t{\r\n\t\t\t\t\"name\": \"value\",\r\n\t\t\t\t\"type\": \"uint256\"\r\n\t\t\t}\r\n\t\t],\r\n\t\t\"name\": \"transfer\",\r\n\t\t\"outputs\": [\r\n\t\t\t{\r\n\t\t\t\t\"name\": \"\",\r\n\t\t\t\t\"type\": \"bool\"\r\n\t\t\t}\r\n\t\t],\r\n\t\t\"payable\": false,\r\n\t\t\"stateMutability\": \"nonpayable\",\r\n\t\t\"type\": \"function\"\r\n\t},\r\n\t{\r\n\t\t\"constant\": true,\r\n\t\t\"inputs\": [\r\n\t\t\t{\r\n\t\t\t\t\"name\": \"owner\",\r\n\t\t\t\t\"type\": \"address\"\r\n\t\t\t},\r\n\t\t\t{\r\n\t\t\t\t\"name\": \"spender\",\r\n\t\t\t\t\"type\": \"address\"\r\n\t\t\t}\r\n\t\t],\r\n\t\t\"name\": \"allowance\",\r\n\t\t\"outputs\": [\r\n\t\t\t{\r\n\t\t\t\t\"name\": \"\",\r\n\t\t\t\t\"type\": \"uint256\"\r\n\t\t\t}\r\n\t\t],\r\n\t\t\"payable\": false,\r\n\t\t\"stateMutability\": \"view\",\r\n\t\t\"type\": \"function\"\r\n\t},\r\n\t{\r\n\t\t\"anonymous\": false,\r\n\t\t\"inputs\": [\r\n\t\t\t{\r\n\t\t\t\t\"indexed\": true,\r\n\t\t\t\t\"name\": \"owner\",\r\n\t\t\t\t\"type\": \"address\"\r\n\t\t\t},\r\n\t\t\t{\r\n\t\t\t\t\"indexed\": true,\r\n\t\t\t\t\"name\": \"spender\",\r\n\t\t\t\t\"type\": \"address\"\r\n\t\t\t},\r\n\t\t\t{\r\n\t\t\t\t\"indexed\": false,\r\n\t\t\t\t\"name\": \"value\",\r\n\t\t\t\t\"type\": \"uint256\"\r\n\t\t\t}\r\n\t\t],\r\n\t\t\"name\": \"Approval\",\r\n\t\t\"type\": \"event\"\r\n\t},\r\n\t{\r\n\t\t\"anonymous\": false,\r\n\t\t\"inputs\": [\r\n\t\t\t{\r\n\t\t\t\t\"indexed\": true,\r\n\t\t\t\t\"name\": \"from\",\r\n\t\t\t\t\"type\": \"address\"\r\n\t\t\t},\r\n\t\t\t{\r\n\t\t\t\t\"indexed\": true,\r\n\t\t\t\t\"name\": \"to\",\r\n\t\t\t\t\"type\": \"address\"\r\n\t\t\t},\r\n\t\t\t{\r\n\t\t\t\t\"indexed\": false,\r\n\t\t\t\t\"name\": \"value\",\r\n\t\t\t\t\"type\": \"uint256\"\r\n\t\t\t}\r\n\t\t],\r\n\t\t\"name\": \"Transfer\",\r\n\t\t\"type\": \"event\"\r\n\t}\r\n]"

func init() {
	abiJSON := abiConst
	abiResult, _ = abi.JSON(strings.NewReader(abiJSON))
	Xrc20Cmd.AddCommand(Xrc20TotalsupplyCmd)
	Xrc20Cmd.AddCommand(Xrc20BalanceofCmd)
	Xrc20Cmd.AddCommand(Xrc20TransferCmd)
	Xrc20Cmd.AddCommand(Xrc20TransferFromCmd)
	Xrc20Cmd.AddCommand(Xrc20ApproveCmd)
	Xrc20Cmd.AddCommand(Xrc20AllowanceCmd)
	Xrc20Cmd.PersistentFlags().StringVar(&config.ReadConfig.Endpoint, "endpoint",
		config.ReadConfig.Endpoint, "set endpoint for once")
	Xrc20Cmd.PersistentFlags().BoolVar(&config.Insecure, "insecure", config.Insecure,
		"insecure connection for once")
	setActionFlags(Xrc20TotalsupplyCmd, Xrc20BalanceofCmd, Xrc20TransferCmd, Xrc20TransferFromCmd, Xrc20ApproveCmd, Xrc20AllowanceCmd)
}

// GetAccountMeta gets account metadata
func GetAccountMeta(addr string) (*iotextypes.AccountMeta, error) {
	conn, err := util.ConnectToEndpoint(config.ReadConfig.SecureConnect && !config.Insecure)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	cli := iotexapi.NewAPIServiceClient(conn)
	ctx := context.Background()
	request := iotexapi.GetAccountRequest{Address: addr}
	response, err := cli.GetAccount(ctx, &request)
	if err != nil {
		sta, ok := status.FromError(err)
		if ok {
			return nil, fmt.Errorf(sta.Message())
		}
		return nil, err
	}
	return response.AccountMeta, nil
}

func setActionFlags(cmds ...*cobra.Command) {
	for _, cmd := range cmds {
		cmd.Flags().StringVarP(&contractAddress, "contract-address", "c", "1",
			"set contract address")
		if cmd == Xrc20TransferCmd || cmd == Xrc20TransferFromCmd || cmd == Xrc20ApproveCmd {
			cmd.Flags().Uint64VarP(&gasLimit, "gas-limit", "l", 0, "set gas limit")
			cmd.Flags().StringVarP(&signer, "signer", "s", "", "choose a signing account")
			cmd.Flags().StringVarP(&gasPrice, "gas-price", "p", "1",
				"set gas price (unit: 10^(-6)Iotx)")
		}
		if cmd == Xrc20AllowanceCmd {
			cmd.Flags().StringVarP(&signer, "signer", "s", "", "choose a signing account")
		}
	}
}

// GetGasPrice gets the suggest gas price
func GetGasPrice() (*big.Int, error) {
	conn, err := util.ConnectToEndpoint(config.ReadConfig.SecureConnect && !config.Insecure)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	cli := iotexapi.NewAPIServiceClient(conn)
	ctx := context.Background()
	request := &iotexapi.SuggestGasPriceRequest{}
	response, err := cli.SuggestGasPrice(ctx, request)
	if err != nil {
		return nil, err
	}
	return new(big.Int).SetUint64(response.GasPrice), nil
}

func readAction(exec *action.Execution, caller string) (string, error) {
	conn, err := util.ConnectToEndpoint(config.ReadConfig.SecureConnect && !config.Insecure)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	cli := iotexapi.NewAPIServiceClient(conn)
	ctx := context.Background()

	request := &iotexapi.ReadContractRequest{
		Execution:     exec.Proto(),
		CallerAddress: caller}
	res, err := cli.ReadContract(ctx, request)
	if err != nil {
		if sta, ok := status.FromError(err); ok {
			return "", fmt.Errorf(sta.Message())
		}
		return "", err
	}
	return res.Data, nil
}

func sendAction(elp action.Envelope) (string, error) {
	fmt.Printf("Enter password #%s:\n", signer)
	bytePassword, err := terminal.ReadPassword(int(syscall.Stdin))
	if err != nil {
		log.L().Error("failed to get password", zap.Error(err))
		return "", err
	}
	prvKey, err := account.KsAccountToPrivateKey(signer, string(bytePassword))
	if err != nil {
		return "", err
	}
	defer prvKey.Zero()
	sealed, err := action.Sign(elp, prvKey)
	prvKey.Zero()
	if err != nil {
		log.L().Error("failed to sign action", zap.Error(err))
		return "", err
	}
	selp := sealed.Proto()

	actionInfo, err := printActionProto(selp)
	if err != nil {
		return "", err
	}
	var confirm string
	fmt.Println("\n" + actionInfo + "\n" +
		"Please confirm your action.\n" +
		"Type 'YES' to continue, quit for anything else.")
	fmt.Scanf("%s", &confirm)
	if confirm != "YES" && confirm != "yes" {
		return "Quit", nil
	}
	fmt.Println()

	conn, err := util.ConnectToEndpoint(config.ReadConfig.SecureConnect && !config.Insecure)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	cli := iotexapi.NewAPIServiceClient(conn)
	ctx := context.Background()

	request := &iotexapi.SendActionRequest{Action: selp}
	if _, err = cli.SendAction(ctx, request); err != nil {
		if sta, ok := status.FromError(err); ok {
			return "", fmt.Errorf(sta.Message())
		}
		return "", err
	}
	shash := hash.Hash256b(byteutil.Must(proto.Marshal(selp)))
	return "Action has been sent to blockchain.\n" +
		"Wait for several seconds and query this action by hash:\n" +
		hex.EncodeToString(shash[:]), nil
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
