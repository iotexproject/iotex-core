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
	"google.golang.org/grpc"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/account"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/config"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/protogen/iotexapi"
	"github.com/iotexproject/iotex-core/protogen/iotextypes"
)

var (
	alias    string
	bytecode []byte
	gasLimit uint64
	gasPrice int64
	nonce    uint64
)

// ActionCmd represents the account command
var ActionCmd = &cobra.Command{
	Use:   "action",
	Short: "Deal with actions of IoTeX blockchain",
	Args:  cobra.MinimumNArgs(1),
}

func init() {
	ActionCmd.AddCommand(actionHashCmd)
	ActionCmd.AddCommand(actionTransferCmd)
	ActionCmd.AddCommand(actionDeployCmd)
	ActionCmd.AddCommand(actionInvokeCmd)
	setActionFlags(actionTransferCmd, actionDeployCmd, actionInvokeCmd)
}

func setActionFlags(cmds ...*cobra.Command) {
	for _, cmd := range cmds {
		cmd.Flags().Uint64VarP(&gasLimit, "gas-limit", "l", 0, "set gas limit")
		cmd.Flags().Int64VarP(&gasPrice, "gas-price", "p", 0, "set gas prize")
		cmd.Flags().StringVarP(&alias, "alias", "a", "", "choose signing key")
		cmd.Flags().Uint64VarP(&nonce, "nonce", "n", 0, "set nonce")
		cmd.MarkFlagRequired("gas-limit")
		cmd.MarkFlagRequired("gas-price")
		cmd.MarkFlagRequired("alias")
		if cmd == actionDeployCmd || cmd == actionInvokeCmd {
			cmd.Flags().BytesHexVarP(&bytecode, "bytecode", "b", nil, "set the byte code")
			actionInvokeCmd.MarkFlagRequired("bytecode")
		}
	}
}

func sendAction(elp action.Envelope) string {
	fmt.Printf("Enter password #%s:\n", alias)
	bytePassword, err := terminal.ReadPassword(syscall.Stdin)
	if err != nil {
		log.L().Error("fail to get password", zap.Error(err))
		return err.Error()
	}
	password := string(bytePassword)
	ehash := elp.Hash()
	sig, err := account.Sign(alias, password, ehash[:])
	if err != nil {
		log.L().Error("fail to sign", zap.Error(err))
		return err.Error()
	}
	pubKey, err := keypair.SigToPublicKey(ehash[:], sig)
	if err != nil {
		log.L().Error("fail to get public key", zap.Error(err))
		return err.Error()
	}
	selp := &iotextypes.Action{
		Core:         elp.Proto(),
		SenderPubKey: pubKey.Bytes(),
		Signature:    sig,
	}
	request := &iotexapi.SendActionRequest{Action: selp}

	endpoint := config.Get("endpoint")
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
	_, err = cli.SendAction(ctx, request)
	if err != nil {
		log.L().Error("server error", zap.Error(err))
		return err.Error()
	}
	shash := hash.Hash256b(byteutil.Must(proto.Marshal(selp)))
	return "Action has been sent to blockchain.\n" +
		"Wait for several seconds and query this action by hash:\n" +
		hex.EncodeToString(shash[:])
}
