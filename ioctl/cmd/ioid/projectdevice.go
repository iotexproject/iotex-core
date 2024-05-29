package ioid

import (
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/ioctl/cmd/action"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/spf13/cobra"
)

// Multi-language support
var (
	_deviceUsages = map[config.Language]string{
		config.English: "device [DEVICE_NFT_CONTRACT_ADDRESS]",
		config.Chinese: "device [设备NFT合约地址]",
	}
	_deviceShorts = map[config.Language]string{
		config.English: "Set device NFT contract address",
		config.Chinese: "设置设备NFT合约地址",
	}
)

// _deviceCmd represents the ioID device command
var _deviceCmd = &cobra.Command{
	Use:   config.TranslateInLang(_deviceUsages, config.UILanguage),
	Short: config.TranslateInLang(_deviceShorts, config.UILanguage),
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		err := device(args)
		return output.PrintError(err)
	},
}

func init() {
	_deviceCmd.Flags().StringVarP(
		&ioIDStore, "ioIDStore", "i",
		"0xA0C9f9A884cdAE649a42F16b057735Bc4fE786CD",
		config.TranslateInLang(_ioIDStoreUsages, config.UILanguage),
	)
	_deviceCmd.Flags().Uint64VarP(
		&projectId, "projectId", "p", 0,
		config.TranslateInLang(_projectIdUsages, config.UILanguage),
	)
}

func device(args []string) error {
	nft := common.HexToAddress(args[0])

	ioioIDStore, err := address.FromHex(ioIDStore)
	if err != nil {
		return output.NewError(output.AddressError, "failed to convert ioIDStore address", err)
	}

	ioIDStoreAbi, err := abi.JSON(strings.NewReader(ioIDStoreABI))
	if err != nil {
		return output.NewError(output.SerializationError, "failed to unmarshal abi", err)
	}

	data, err := ioIDStoreAbi.Pack("setDeviceContract", new(big.Int).SetUint64(projectId), nft)
	if err != nil {
		return output.NewError(output.ConvertError, "failed to pack arguments", err)
	}

	return action.Execute(ioioIDStore.String(), big.NewInt(0), data)
}
