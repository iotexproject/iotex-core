// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package did

import (
	"encoding/hex"
	"strings"

	"github.com/pkg/errors"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/ioctl/cmd/action"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

// Multi-language support
var (
	getHashCmdUses = map[config.Language]string{
		config.English: "gethash (CONTRACT_ADDRESS|ALIAS) DID",
		config.Chinese: "gethash (合约地址|别名) DID",
	}
	getHashCmdShorts = map[config.Language]string{
		config.English: "Gethash get DID doc's hash on IoTeX blockchain",
		config.Chinese: "Gethash 在IoTeX链上获取相应DID的doc hash",
	}
)

// didGetHashCmd represents the contract invoke getHash command
var didGetHashCmd = &cobra.Command{
	Use:   config.TranslateInLang(getHashCmdUses, config.UILanguage),
	Short: config.TranslateInLang(getHashCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		return output.PrintError(getHash(args))

	},
}

func getHash(args []string) (err error) {
	contract, err := util.Address(args[0])
	if err != nil {
		return output.NewError(output.AddressError, "failed to get contract address", err)
	}
	addr, err := address.FromString(contract)
	if err != nil {
		return output.NewError(output.ConvertError, "invalid contract address", err)
	}

	abi, err := abi.JSON(strings.NewReader(DIDABI))
	if err != nil {
		return
	}
	bytecode, err := encodeGet(abi, getHashName, args[1])
	if err != nil {
		return output.NewError(output.ConvertError, "invalid bytecode", err)
	}

	result, err := action.Read(addr, "0", bytecode)
	if err != nil {
		return
	}
	ret, err := hex.DecodeString(result)
	if err != nil {
		return
	}
	var out [32]byte
	err = abi.Unpack(&out, getHashName, ret)
	if err != nil {
		return errors.New("DID does not exist")
	}
	output.PrintResult(hex.EncodeToString(out[:]))
	return
}

func encodeGet(abi abi.ABI, method, did string) (ret []byte, err error) {
	_, exist := abi.Methods[method]
	if !exist {
		return nil, errors.New("method is not found")
	}
	return abi.Pack(method, []byte(did))
}
