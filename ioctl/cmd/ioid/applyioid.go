package ioid

import (
	"encoding/hex"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/ioctl/cmd/action"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/spf13/cobra"
)

// Multi-language support
var (
	_applyUsages = map[config.Language]string{
		config.English: "apply",
		config.Chinese: "apply",
	}
	_applyShorts = map[config.Language]string{
		config.English: "Apply ioIDs",
		config.Chinese: "申请 ioID",
	}
	_ioIDStoreUsages = map[config.Language]string{
		config.English: "ioID store contract address",
		config.Chinese: "ioID Store 合约地址",
	}
	_projectIdUsages = map[config.Language]string{
		config.English: "project id",
		config.Chinese: "项目Id",
	}
	_amountUsages = map[config.Language]string{
		config.English: "amount",
		config.Chinese: "数量",
	}
)

// _applyCmd represents the ioID apply command
var _applyCmd = &cobra.Command{
	Use:   config.TranslateInLang(_applyUsages, config.UILanguage),
	Short: config.TranslateInLang(_applyShorts, config.UILanguage),
	Args:  cobra.MinimumNArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		err := apply()
		return output.PrintError(err)
	},
}

var (
	ioIDStore    string
	projectId    uint64
	amount       uint64
	ioIDStoreABI = `[
		{
			"inputs": [],
			"name": "price",
			"outputs": [
				{
					"internalType": "uint256",
					"name": "",
					"type": "uint256"
				}
			],
			"stateMutability": "view",
			"type": "function"
		},
		{
			"inputs": [
				{
					"internalType": "uint256",
					"name": "_projectId",
					"type": "uint256"
				},
				{
					"internalType": "uint256",
					"name": "_amount",
					"type": "uint256"
				}
			],
			"name": "applyIoIDs",
			"outputs": [],
			"stateMutability": "payable",
			"type": "function"
		}
	]`
)

func init() {
	_applyCmd.Flags().StringVarP(
		&ioIDStore, "ioIDStore", "i",
		"0xA0C9f9A884cdAE649a42F16b057735Bc4fE786CD",
		config.TranslateInLang(_ioIDStoreUsages, config.UILanguage),
	)
	_applyCmd.Flags().Uint64VarP(
		&projectId, "projectId", "p", 0,
		config.TranslateInLang(_projectIdUsages, config.UILanguage),
	)
	_applyCmd.Flags().Uint64VarP(
		&amount, "amount", "a", 1,
		config.TranslateInLang(_amountUsages, config.UILanguage),
	)
}

func apply() error {
	ioioIDStore, err := address.FromHex(ioIDStore)
	if err != nil {
		return output.NewError(output.AddressError, "failed to convert ioIDStore address", err)
	}

	ioIDStoreAbi, err := abi.JSON(strings.NewReader(ioIDStoreABI))
	if err != nil {
		return output.NewError(output.SerializationError, "failed to unmarshal abi", err)
	}

	data, err := ioIDStoreAbi.Pack("price")
	if err != nil {
		return output.NewError(output.ConvertError, "failed to pack price arguments", err)
	}
	priceHex, err := action.Read(ioioIDStore, "0", data)
	if err != nil {
		return output.NewError(output.APIError, "failed to read contract", err)
	}
	price, _ := hex.DecodeString(priceHex)
	total := new(big.Int).Mul(new(big.Int).SetBytes(price), new(big.Int).SetUint64(amount))

	data, err = ioIDStoreAbi.Pack("applyIoIDs", new(big.Int).SetUint64(projectId), new(big.Int).SetUint64(amount))
	if err != nil {
		return output.NewError(output.ConvertError, "failed to pack apply arguments", err)
	}

	return action.Execute(ioioIDStore.String(), total, data)
}
