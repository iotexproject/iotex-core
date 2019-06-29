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

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"google.golang.org/grpc/status"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/account"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/config"
	"github.com/iotexproject/iotex-core/cli/ioctl/flag"
	"github.com/iotexproject/iotex-core/cli/ioctl/util"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

// Flags
var (
	gasLimitFlag = flag.NewUint64VarP("gas-limit", "l", 300000, "set gas limit")
	gasPriceFlag = flag.NewStringVarP("gas-price", "p", "1", "set gas price (unit: 10^(-6)IOTX), use suggested gas price if input is \"0\"")
	nonceFlag    = flag.NewUint64VarP("nonce", "n", 0, "set nonce (default using pending nonce)")
	signerFlag   = flag.NewStringVarP("signer", "s", "", "choose a signing account")
	bytecodeFlag = flag.NewStringVarP("bytecode", "b", "", "set the byte code")
	notConfirmed = errors.New("not confirmed and quit")
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
	ActionCmd.AddCommand(actionSendRawCmd)
	ActionCmd.AddCommand(actionEstimateCmd)
	ActionCmd.PersistentFlags().StringVar(&config.ReadConfig.Endpoint, "endpoint",
		config.ReadConfig.Endpoint, "set endpoint for once")
	ActionCmd.PersistentFlags().BoolVar(&config.Insecure, "insecure", config.Insecure,
		"insecure connection for once")
}

func decodeBytecode() ([]byte, error) {
	return hex.DecodeString(strings.TrimPrefix(bytecodeFlag.Value().(string), "0x"))
}

func signer() (address string, err error) {
	return util.GetAddress([]string{signerFlag.Value().(string)})
}

func nonce(executor string) (uint64, error) {
	nonce := nonceFlag.Value().(uint64)
	if nonce != 0 {
		return nonce, nil
	}
	accountMeta, err := account.GetAccountMeta(executor)
	if err != nil {
		return 0, err
	}
	return accountMeta.PendingNonce, nil
}

func registerWriteCommand(cmd *cobra.Command) {
	gasLimitFlag.RegisterCommand(cmd)
	gasPriceFlag.RegisterCommand(cmd)
	signerFlag.RegisterCommand(cmd)
	nonceFlag.RegisterCommand(cmd)
}

// gasPriceInRau returns the suggest gas price
func gasPriceInRau() (*big.Int, error) {
	gasPrice := gasPriceFlag.Value().(string)
	if len(gasPrice) != 0 {
		return util.StringToRau(gasPrice, util.GasPriceDecimalNum)
	}
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

func isBalanceEnough(address string, act action.SealedEnvelope) (err error) {
	accountMeta, err := account.GetAccountMeta(address)
	if err != nil {
		return
	}
	balance, ok := big.NewInt(0).SetString(accountMeta.Balance, 10)
	if !ok {
		err = fmt.Errorf("failed to convert balance into big int")
		return
	}
	cost, err := act.Cost()
	if balance.Cmp(cost) < 0 {
		err = fmt.Errorf("balance is not enough")
		return
	}
	return
}

func makeExecution(contract string, amount *big.Int, bytecode []byte) (elp action.Envelope, signers string, err error) {
	gasPriceRau, err := gasPriceInRau()
	if err != nil {
		return
	}
	signers, err = signer()
	if err != nil {
		return
	}
	n, err := nonce(signers)
	if err != nil {
		return
	}
	gasLimit := gasLimitFlag.Value().(uint64)
	tx, err := action.NewExecution(contract, n, amount, gasLimit, gasPriceRau, bytecode)
	if err != nil || tx == nil {
		err = errors.Wrap(err, "cannot make a Execution instance")
		log.L().Error("error when invoke an execution", zap.Error(err))
		return
	}
	elp = (&action.EnvelopeBuilder{}).
		SetNonce(n).
		SetGasPrice(gasPriceRau).
		SetGasLimit(gasLimit).
		SetAction(tx).Build()
	return
}

func sendToChain(request interface{}) (err error) {
	conn, err := util.ConnectToEndpoint(config.ReadConfig.SecureConnect && !config.Insecure)
	if err != nil {
		return
	}
	defer conn.Close()
	cli := iotexapi.NewAPIServiceClient(conn)
	ctx := context.Background()
	switch req := request.(type) {
	case *iotexapi.SendActionRequest:
		if _, err = cli.SendAction(ctx, req); err != nil {
			if sta, ok := status.FromError(err); ok {
				err = fmt.Errorf(sta.Message())
				return
			}
			return
		}
	case *iotexapi.EstimateGasForActionRequest:
		resp, errs := cli.EstimateGasForAction(ctx, req)
		if errs != nil {
			if sta, ok := status.FromError(errs); ok {
				return fmt.Errorf(sta.Message())
			}
			return errs
		}
		fmt.Printf("Gas estimated is: %d\n", resp.Gas)
	}
	return
}

func sendRaw(selp *iotextypes.Action) error {
	request := &iotexapi.SendActionRequest{Action: selp}
	err := sendToChain(request)
	if err != nil {
		return err
	}
	shash := hash.Hash256b(byteutil.Must(proto.Marshal(selp)))
	fmt.Println("Action has been sent to blockchain.")
	fmt.Printf("Wait for several seconds and query this action by hash: %s\n", hex.EncodeToString(shash[:]))
	return nil
}

func signAndConfirm(elp action.Envelope, signer string, forEstimate bool) (selp *iotextypes.Action, err error) {
	fmt.Printf("Enter password #%s:\n", signer)
	password, err := util.ReadSecretFromStdin()
	if err != nil {
		log.L().Error("failed to get password", zap.Error(err))
		return
	}
	prvKey, err := account.KsAccountToPrivateKey(signer, password)
	if err != nil {
		return
	}
	defer prvKey.Zero()
	sealed, err := action.Sign(elp, prvKey)
	prvKey.Zero()
	if err != nil {
		log.L().Error("failed to sign action", zap.Error(err))
		return
	}
	selp = sealed.Proto()
	var actionInfo string
	if !forEstimate {
		if err = isBalanceEnough(signer, sealed); err != nil {
			return
		}
		actionInfo, err = printActionProto(selp)
		if err != nil {
			return
		}
		var confirm string
		fmt.Println("\n" + actionInfo + "\n" +
			"Please confirm your action.\n" +
			"Type 'YES' to continue, quit for anything else.")
		fmt.Scanf("%s", &confirm)
		if confirm != "YES" && confirm != "yes" {
			fmt.Println("Quit")
			return nil, notConfirmed
		}
		fmt.Println()
	}
	return
}

func sendAction(elp action.Envelope, signer string) error {
	selp, err := signAndConfirm(elp, signer, false)
	if err != nil && errors.Cause(err) == notConfirmed {
		return nil
	}
	if err != nil {
		return err
	}
	return sendRaw(selp)
}

func estimateGas(contract string, amount *big.Int, bytecode []byte) (err error) {
	elp, signer, err := makeExecution(contract, amount, bytecode)
	if err != nil {
		return
	}
	selp, err := signAndConfirm(elp, signer, true)
	if err != nil && errors.Cause(err) == notConfirmed {
		return nil
	}
	if err != nil {
		return
	}
	request := &iotexapi.EstimateGasForActionRequest{Action: selp}
	err = sendToChain(request)
	return
}

func execute(contract string, amount *big.Int, bytecode []byte) (err error) {
	elp, signer, err := makeExecution(contract, amount, bytecode)
	return sendAction(elp, signer)
}
