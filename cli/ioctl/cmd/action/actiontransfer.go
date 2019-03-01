// Copyright (c) 2019 IoTeX
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

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/howeyc/gopass"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/account"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/config"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/protogen/iotexapi"
	"github.com/iotexproject/iotex-core/protogen/iotextypes"
)

// actionTransferCmd transfers tokens on IoTex blockchain
var actionTransferCmd = &cobra.Command{
	Use:   "transfer recipient amount data",
	Short: "Transfer tokens on IoTex blokchain",
	Args:  cobra.ExactArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(transfer(args))
	},
}

func init() {
	actionTransferCmd.Flags().Uint64VarP(&gasLimit, "gas-limit", "l", 0, "set gas limit")
	actionTransferCmd.Flags().Int64VarP(&gasPrice, "gas-price", "p", 0, "set gas prize")
	actionTransferCmd.Flags().StringVarP(&alias, "alias", "a", "", "choose signing key")
	actionTransferCmd.MarkFlagRequired("gas-limit")
	actionTransferCmd.MarkFlagRequired("gas-price")
	actionTransferCmd.MarkFlagRequired("alias")
}

// transfer transfers tokens on IoTex blockchain
func transfer(args []string) string {
	recipient := args[0]
	amount, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		log.L().Error("cannot transfer "+args[1]+" into int64", zap.Error(err))
		return err.Error()
	}
	payload := args[2]
	// TODO: Check the validity of args

	sender, err := account.AliasToAddress(alias)
	if err != nil {
		return err.Error()
	}
	accountMeta, err := account.GetAccountMeta(sender)
	if err != nil {
		log.L().Error("cannot get account from "+recipient, zap.Error(err))
		return err.Error()
	}
	tx, err := action.NewTransfer(accountMeta.PendingNonce, big.NewInt(amount),
		recipient, []byte(payload), gasLimit, big.NewInt(gasPrice))
	if err != nil {
		log.L().Error("cannot make a Transfer instance", zap.Error(err))
	}
	fmt.Printf("Enter password #%s:", alias)
	password, err := gopass.GetPasswd()
	if err != nil {
		log.L().Error("fail to get password", zap.Error(err))
		return err.Error()
	}
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(accountMeta.PendingNonce).
		SetGasPrice(big.NewInt(gasPrice)).
		SetGasLimit(gasLimit).
		SetAction(tx).Build()
	hash := elp.Hash()
	sig, err := account.Sign(alias, string(password), hash[:])
	if err != nil {
		log.L().Error("fail to sign", zap.Error(err))
		return err.Error()
	}
	pubKey, err := crypto.SigToPub(hash[:], sig)
	selp := &iotextypes.Action{
		Core:         elp.Proto(),
		SenderPubKey: keypair.PublicKeyToBytes(pubKey),
		Signature:    sig,
	}
	if err != nil {
		log.L().Error("fail to sign envelope", zap.Error(err))
		return err.Error()
	}
	request := iotexapi.SendActionRequest{Action: selp}

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
	_, err = cli.SendAction(ctx, &request)
	if err != nil {
		log.L().Error("server error", zap.Error(err))
		return err.Error()
	}
	return "Transfer action has been sent."
}
