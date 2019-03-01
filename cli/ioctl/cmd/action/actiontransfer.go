// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"syscall"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh/terminal"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/account"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/protogen/iotexapi"
	"github.com/iotexproject/iotex-core/protogen/iotextypes"
)

// actionTransferCmd transfers tokens on IoTeX blockchain
var actionTransferCmd = &cobra.Command{
	Use:   "transfer recipient amount data",
	Short: "Transfer tokens on IoTeX blokchain",
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

// transfer transfers tokens on IoTeX blockchain
func transfer(args []string) string {
	recipient := args[0]
	amount, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		log.L().Error("cannot convert "+args[1]+" into int64", zap.Error(err))
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
		return err.Error()
	}
	tx, err := action.NewTransfer(accountMeta.PendingNonce, big.NewInt(amount),
		recipient, []byte(payload), gasLimit, big.NewInt(gasPrice))
	if err != nil {
		log.L().Error("cannot make a Transfer instance", zap.Error(err))
	}
	fmt.Printf("Enter password #%s:\n", alias)
	bytePassword, err := terminal.ReadPassword(syscall.Stdin)
	if err != nil {
		log.L().Error("fail to get password", zap.Error(err))
		return err.Error()
	}
	password := strings.TrimSpace(string(bytePassword))
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(accountMeta.PendingNonce).
		SetGasPrice(big.NewInt(gasPrice)).
		SetGasLimit(gasLimit).
		SetAction(tx).Build()
	hash := elp.Hash()
	sig, err := account.Sign(alias, password, hash[:])
	if err != nil {
		log.L().Error("fail to sign", zap.Error(err))
		return err.Error()
	}
	pubKey, err := crypto.SigToPub(hash[:], sig)
	if err != nil {
		log.L().Error("fail to get public key", zap.Error(err))
		return err.Error()
	}
	selp := &iotextypes.Action{
		Core:         elp.Proto(),
		SenderPubKey: crypto.FromECDSAPub(pubKey),
		Signature:    sig,
	}
	request := &iotexapi.SendActionRequest{Action: selp}
	return sendAction(request)
}
