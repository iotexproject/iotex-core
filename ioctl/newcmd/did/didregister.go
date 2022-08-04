// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package did

import (
	"encoding/hex"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/newcmd/action"
)

const (
	_registerDIDName   = "registerDID"
	_getHashName       = "getHash"
	_getURIName        = "getURI"
	_updateDIDName     = "updateDID"
	_deregisterDIDName = "deregisterDID"
	// DIDABI is the did abi
	DIDABI = `[{"constant": false,"inputs": [],"name": "deregisterDID","outputs": [],"payable": false,"stateMutability": "nonpayable","type": "function"},{"constant": true,"inputs": [{"internalType": "bytes","name": "did","type": "bytes"}],"name": "getHash","outputs": [{"internalType": "bytes32","name": "","type": "bytes32"}],"payable": false,"stateMutability": "view","type": "function"},   {"constant": true,"inputs": [{"internalType": "bytes","name": "did","type": "bytes"}],"name": "getURI","outputs": [{"internalType": "bytes","name": "","type": "bytes"}],"payable": false,"stateMutability": "view","type": "function"},{"constant": false,"inputs": [{"internalType": "bytes32","name": "h","type": "bytes32"},{"internalType": "bytes","name": "uri","type": "bytes"}],"name": "registerDID","outputs": [],"payable": false,"stateMutability": "nonpayable","type": "function"},{"constant": false,"inputs": [{"internalType": "bytes32","name": "h","type": "bytes32"},{"internalType": "bytes","name": "uri","type": "bytes"}],"name": "updateDID","outputs": [],"payable": false,"stateMutability": "nonpayable","type": "function"}]`
)

// Multi-language support
var (
	_registerCmdUses = map[config.Language]string{
		config.English: "register (CONTRACT_ADDRESS|ALIAS) hash uri",
		config.Chinese: "register (合约地址|别名) hash uri",
	}
	_registerCmdShorts = map[config.Language]string{
		config.English: "Register DID on IoTeX blockchain",
		config.Chinese: "Register 在IoTeX链上注册DID",
	}
)

// NewDidRegisterCmd represents the did register command
func NewDidRegisterCmd(client ioctl.Client) *cobra.Command {
	use, _ := client.SelectTranslation(_registerCmdUses)
	short, _ := client.SelectTranslation(_registerCmdShorts)

	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			contract, err := client.Address(args[0])
			if err != nil {
				return errors.Wrap(err, "failed to get contract address")
			}

			hashSlice, err := hex.DecodeString(args[1])
			if err != nil {
				return errors.Wrap(err, "failed to decode data")
			}
			var hashArray [32]byte
			copy(hashArray[:], hashSlice)
			abi, err := abi.JSON(strings.NewReader(DIDABI))
			if err != nil {
				return errors.Wrap(err, "falied to parse abi")
			}
			_, exist := abi.Methods[_registerDIDName]
			if !exist {
				return errors.New("method is not found")
			}
			bytecode, err := abi.Pack(_registerDIDName, hashArray, []byte(args[2]))
			if err != nil {
				return errors.Wrap(err, "invalid bytecode")
			}
			return action.Execute(client, cmd, contract, big.NewInt(0), bytecode)
		},
	}
	action.RegisterWriteCommand(client, cmd)
	return cmd
}
