package ioid

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/v2/ioctl/cmd/ws"
	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
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
		config.ReadConfig.IoidProjectStoreContract,
		config.TranslateInLang(_ioIDStoreUsages, config.UILanguage),
	)
	_deviceCmd.Flags().Uint64VarP(
		&projectId, "projectId", "p", 0,
		config.TranslateInLang(_projectIdUsages, config.UILanguage),
	)
}

func device(args []string) error {
	contract := common.HexToAddress(args[0])

	caller, err := ws.NewContractCaller(ioIDStoreABI, ioIDStore)
	if err != nil {
		return output.NewError(output.SerializationError, "failed to create contract caller", err)
	}

	tx, err := caller.CallAndRetrieveResult("setDeviceContract", []any{
		new(big.Int).SetUint64(projectId),
		contract},
	)
	if err != nil {
		return output.NewError(output.SerializationError, "failed to call contract", err)
	}

	fmt.Printf("Set device NFT contract address txHash: %s\n", tx)

	return nil
}
