// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"context"
	"fmt"
	"math/big"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/status"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/ioctl/cmd/alias"
	"github.com/iotexproject/iotex-core/ioctl/cmd/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/pkg/unit"
)

const defaultGasLimit = uint64(20000000)

var defaultGasPrice = big.NewInt(unit.Qev)

// actionReadCmd represents the action read command
var actionReadCmd = &cobra.Command{
	Use:   "read (ALIAS|CONTRACT_ADDRESS) -b BYTE_CODE [-s SIGNER]",
	Short: "Read smart contract on IoTeX blockchain",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		contract, err := alias.IOAddress(args[0])
		if err != nil {
			return output.PrintError(output.AddressError, err.Error())
		}
		bytecode, err := decodeBytecode()
		if err != nil {
			return output.PrintError(output.ConvertError, err.Error())
		}
		result, err := read(contract, bytecode)
		if err != nil {
			return output.PrintError(0, err.Error()) // TODO: undefined error
		}
		output.PrintResult(result)
		return err
	},
}

func init() {
	signerFlag.RegisterCommand(actionReadCmd)
	bytecodeFlag.RegisterCommand(actionReadCmd)
	bytecodeFlag.MarkFlagRequired(actionReadCmd)
}

// read reads smart contract on IoTeX blockchain
func read(contract address.Address, bytecode []byte) (string, error) {
	caller, err := signer()
	if err != nil {
		return "", err
	}
	exec, err := action.NewExecution(contract.String(), 0, big.NewInt(0), defaultGasLimit, defaultGasPrice, bytecode)
	if err != nil {
		return "", fmt.Errorf("cannot make an Execution instance" + err.Error())
	}
	conn, err := util.ConnectToEndpoint(config.ReadConfig.SecureConnect && !config.Insecure)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	res, err := iotexapi.NewAPIServiceClient(conn).ReadContract(
		context.Background(),
		&iotexapi.ReadContractRequest{
			Execution:     exec.Proto(),
			CallerAddress: caller,
		},
	)
	if err == nil {
		return res.Data, nil
	}
	if sta, ok := status.FromError(err); ok {
		return "", fmt.Errorf(sta.Message())
	}
	return "", err
}
