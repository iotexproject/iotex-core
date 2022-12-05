// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package did

import (
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
)

// Multi-language support
var (
	_dIDCmdShorts = map[config.Language]string{
		config.English: "Manage Decentralized Identity of IoTeX blockchain",
		config.Chinese: "管理IoTeX区块链上的去中心化数字身份",
	}
	// _didABI is the interface of the abi encoding of did
	_didABI abi.ABI
	err     error
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

func init() {
	_didABI, err = abi.JSON(strings.NewReader(DIDABI))
	if err != nil {
		panic(err)
	}
}

// NewDidCmd represents the did command
func NewDidCmd(client ioctl.Client) *cobra.Command {
	short, _ := client.SelectTranslation(_dIDCmdShorts)
	cmd := &cobra.Command{
		Use:   "did",
		Short: short,
	}
	cmd.AddCommand(NewDidRegisterCmd(client))
	client.SetEndpointWithFlag(cmd.PersistentFlags().StringVar)
	client.SetInsecureWithFlag(cmd.PersistentFlags().BoolVar)
	return cmd
}
