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
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/protogen/iotexapi"
	"github.com/iotexproject/iotex-core/protogen/iotextypes"
)

// actionInvokeCmd invokes smart contract on IoTeX blockchain
var actionInvokeCmd = &cobra.Command{
	Use:   "invoke contract [amount]",
	Short: "Invoke smart contract on IoTeX blockchain",
	Args:  cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(invoke(args))
	},
}

func init() {
	actionInvokeCmd.Flags().Uint64VarP(&gasLimit, "gas-limit", "l", 0, "set gas limit")
	actionInvokeCmd.Flags().Int64VarP(&gasPrice, "gas-price", "p", 0, "set gas prize")
	actionInvokeCmd.Flags().StringVarP(&alias, "alias", "a", "", "choose signing key")
	actionInvokeCmd.Flags().BytesHexVarP(&bytecode, "bytecode", "b", nil, "set the byte code")
	actionInvokeCmd.MarkFlagRequired("gas-limit")
	actionInvokeCmd.MarkFlagRequired("gas-price")
	actionInvokeCmd.MarkFlagRequired("alias")
	actionInvokeCmd.MarkFlagRequired("bytecode")
}

// invoke invokes smart contract on IoTeX blockchain
func invoke(args []string) string {
	contract := args[0]
	amount := int64(0)
	var err error
	if len(args) == 2 {
		amount, err = strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			log.L().Error("cannot convert "+args[1]+" into int64", zap.Error(err))
			return err.Error()
		}
	}
	executor, err := account.AliasToAddress(alias)
	if err != nil {
		return err.Error()
	}
	accountMeta, err := account.GetAccountMeta(executor)
	if err != nil {
		return err.Error()
	}
	tx, err := action.NewExecution(contract, accountMeta.PendingNonce, big.NewInt(amount),
		gasLimit, big.NewInt(gasPrice), bytecode)
	if err != nil {
		log.L().Error("cannot make a Execution instance", zap.Error(err))
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
		SenderPubKey: keypair.PublicKeyToBytes(pubKey),
		Signature:    sig,
	}
	request := &iotexapi.SendActionRequest{Action: selp}
	return sendAction(request)
}
