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
	"syscall"

	"github.com/golang/protobuf/proto"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh/terminal"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/account"
	"github.com/iotexproject/iotex-core/cli/ioctl/util"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/protogen/iotexapi"
)

// Flags
var (
	gasLimit uint64
	gasPrice string
	nonce    uint64
	signer   string
	bytecode []byte
)

// ActionCmd represents the account command
var ActionCmd = &cobra.Command{
	Use:   "action",
	Short: "Manage actions of IoTeX blockchain",
	Args:  cobra.MinimumNArgs(1),
}

func init() {
	ActionCmd.AddCommand(actionHashCmd)
	ActionCmd.AddCommand(actionTransferCmd)
	ActionCmd.AddCommand(actionDeployCmd)
	ActionCmd.AddCommand(actionInvokeCmd)
	ActionCmd.AddCommand(actionClaimCmd)
	setActionFlags(actionTransferCmd, actionDeployCmd, actionInvokeCmd, actionClaimCmd)
}

func setActionFlags(cmds ...*cobra.Command) {
	for _, cmd := range cmds {
		cmd.Flags().Uint64VarP(&gasLimit, "gas-limit", "l", 0, "set gas limit")
		cmd.Flags().StringVarP(&gasPrice, "gas-price", "p", "",
			"set gas price (unit: 10^(-6)Iotx)")
		cmd.Flags().StringVarP(&signer, "signer", "s", "", "choose a signing account")
		cmd.Flags().Uint64VarP(&nonce, "nonce", "n", 0, "set nonce")
		cmd.MarkFlagRequired("gas-limit")
		cmd.MarkFlagRequired("gas-price")
		cmd.MarkFlagRequired("signer")
		if cmd == actionDeployCmd || cmd == actionInvokeCmd {
			cmd.Flags().BytesHexVarP(&bytecode, "bytecode", "b", nil, "set the byte code")
			actionInvokeCmd.MarkFlagRequired("bytecode")
		}
	}
}

func sendAction(elp action.Envelope) string {
	fmt.Printf("Enter password #%s:\n", signer)
	bytePassword, err := terminal.ReadPassword(int(syscall.Stdin))
	if err != nil {
		log.L().Error("failed to get password", zap.Error(err))
		return err.Error()
	}
	prvKey, err := account.KsAccountToPrivateKey(signer, string(bytePassword))
	if err != nil {
		return err.Error()
	}
	defer prvKey.Zero()
	sealed, err := action.Sign(elp, prvKey)
	prvKey.Zero()
	if err != nil {
		log.L().Error("failed to sign action", zap.Error(err))
		return err.Error()
	}
	selp := sealed.Proto()

	actionInfo, err := printActionProto(selp)
	if err != nil {
		return err.Error()
	}
	var confirm string
	fmt.Println("\n" + actionInfo + "\n" +
		"Please confirm your action.\n" +
		"Type 'YES' to continue, quit for anything else.")
	fmt.Scanf("%s", &confirm)
	if confirm != "YES" && confirm != "yes" {
		return "Quit"
	}
	fmt.Println()

	request := &iotexapi.SendActionRequest{Action: selp}
	conn, err := util.ConnectToEndpoint()
	if err != nil {
		return err.Error()
	}
	defer conn.Close()
	cli := iotexapi.NewAPIServiceClient(conn)
	ctx := context.Background()
	_, err = cli.SendAction(ctx, request)
	if err != nil {
		return err.Error()
	}
	shash := hash.Hash256b(byteutil.Must(proto.Marshal(selp)))
	return "Action has been sent to blockchain.\n" +
		"Wait for several seconds and query this action by hash:\n" +
		hex.EncodeToString(shash[:])
}
