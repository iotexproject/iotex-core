// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"syscall"

	"github.com/golang/protobuf/proto"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh/terminal"
	"google.golang.org/grpc/status"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/account"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/alias"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/config"
	"github.com/iotexproject/iotex-core/cli/ioctl/util"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
)

// Flags
var (
	gasLimit       uint64
	gasPrice       string
	nonce          uint64
	signer         string
	bytecodeString string
)

// ActionCmd represents the account command
var ActionCmd = &cobra.Command{
	Use:   "action",
	Short: "Manage actions of IoTeX blockchain",
}

func init() {
	ActionCmd.AddCommand(actionHashCmd)
	ActionCmd.AddCommand(actionTransferCmd)
	ActionCmd.AddCommand(actionDeployCmd)
	ActionCmd.AddCommand(actionInvokeCmd)
	ActionCmd.AddCommand(actionReadCmd)
	ActionCmd.AddCommand(actionClaimCmd)
	ActionCmd.AddCommand(actionDepositCmd)
	ActionCmd.PersistentFlags().StringVar(&config.ReadConfig.Endpoint, "endpoint",
		config.ReadConfig.Endpoint, "set endpoint for once")
	ActionCmd.PersistentFlags().BoolVar(&config.Insecure, "insecure", config.Insecure,
		"insecure connection for once")
	setActionFlags(actionTransferCmd, actionDeployCmd, actionInvokeCmd, actionReadCmd, actionClaimCmd,
		actionDepositCmd)
}

func inputToExecution(contract string, amount *big.Int) (*action.Execution, error) {
	executor, err := alias.Address(signer)
	if err != nil {
		return nil, err
	}
	var gasPriceRau *big.Int
	if len(gasPrice) == 0 {
		gasPriceRau, err = GetGasPrice()
		if err != nil {
			return nil, err
		}
	} else {
		gasPriceRau, err = util.StringToRau(gasPrice, util.GasPriceDecimalNum)
		if err != nil {
			return nil, err
		}
	}
	if nonce == 0 {
		accountMeta, err := account.GetAccountMeta(executor)
		if err != nil {
			return nil, err
		}
		nonce = accountMeta.PendingNonce
	}
	bytecodeBytes, err := hex.DecodeString(strings.TrimLeft(bytecodeString, "0x"))
	if err != nil {
		log.L().Error("cannot decode bytecode string", zap.Error(err))
		return nil, err
	}
	return action.NewExecution(contract, nonce, amount, gasLimit, gasPriceRau, bytecodeBytes)
}

func setActionFlags(cmds ...*cobra.Command) {
	for _, cmd := range cmds {
		cmd.Flags().Uint64VarP(&gasLimit, "gas-limit", "l", 0, "set gas limit")
		cmd.Flags().StringVarP(&gasPrice, "gas-price", "p", "1",
			"set gas price (unit: 10^(-6)Iotx)")
		cmd.Flags().StringVarP(&signer, "signer", "s", "", "choose a signing account")
		cmd.Flags().Uint64VarP(&nonce, "nonce", "n", 0, "set nonce")
		cmd.MarkFlagRequired("signer")
		if cmd == actionDeployCmd || cmd == actionInvokeCmd || cmd == actionReadCmd {
			cmd.Flags().StringVarP(&bytecodeString, "bytecode", "b", "", "set the byte code")

			cmd.MarkFlagRequired("gas-limit")
			cmd.MarkFlagRequired("bytecode")
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

func readAction(exec *action.Execution, caller string) (string, error) {
	fmt.Println()
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
