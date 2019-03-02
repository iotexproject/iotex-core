// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"fmt"
	"math/big"
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

// actionDeployCmd deploys smart contract on IoTeX blockchain
var actionDeployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy smart contract on IoTeX blockchain",
	Args:  cobra.MaximumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(deploy())
	},
}

func init() {
	actionDeployCmd.Flags().Uint64VarP(&gasLimit, "gas-limit", "l", 0, "set gas limit")
	actionDeployCmd.Flags().Int64VarP(&gasPrice, "gas-price", "p", 0, "set gas prize")
	actionDeployCmd.Flags().StringVarP(&alias, "alias", "a", "", "choose signing key")
	actionDeployCmd.Flags().BytesHexVarP(&bytecode, "bytecode", "b", nil, "set the byte code")
	actionDeployCmd.MarkFlagRequired("gas-limit")
	actionDeployCmd.MarkFlagRequired("gas-price")
	actionDeployCmd.MarkFlagRequired("alias")
	actionDeployCmd.MarkFlagRequired("bytecode")
}

// deploy deploys smart contract on IoTeX blockchain
func deploy() string {
	executor, err := account.AliasToAddress(alias)
	if err != nil {
		return err.Error()
	}
	accountMeta, err := account.GetAccountMeta(executor)
	if err != nil {
		return err.Error()
	}
	tx, err := action.NewExecution("", accountMeta.PendingNonce, big.NewInt(0),
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
