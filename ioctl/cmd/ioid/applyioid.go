package ioid

import (
	"bytes"
	_ "embed" // used to embed contract abi
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/v2/ioctl/cmd/ws"
	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
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
	ioIDStore string
	projectId uint64
	amount    uint64
	//go:embed contracts/abis/ioIDStore.json
	ioIDStoreJSON []byte
	ioIDStoreABI  abi.ABI
)

func init() {
	var err error
	ioIDStoreABI, err = abi.JSON(bytes.NewReader(ioIDStoreJSON))
	if err != nil {
		panic(err)
	}

	_applyCmd.Flags().StringVarP(
		&ioIDStore, "ioIDStore", "i",
		config.ReadConfig.IoidProjectStoreContract,
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
	caller, err := ws.NewContractCaller(ioIDStoreABI, ioIDStore)
	if err != nil {
		return output.NewError(output.SerializationError, "failed to create contract caller", err)
	}

	result := ws.NewContractResult(&ioIDStoreABI, "price", new(big.Int))
	if err = caller.Read("price", nil, result); err != nil {
		return output.NewError(output.SerializationError, "failed to read contract", err)
	}
	price, err := result.Result()
	if err != nil {
		return output.NewError(output.SerializationError, "failed parse price", err)
	}
	total := new(big.Int).Mul(price.(*big.Int), new(big.Int).SetUint64(amount))
	fmt.Printf(
		"Apply %d ioIDs need %s IOTX\n", amount,
		new(big.Float).Quo(
			new(big.Float).SetInt(total),
			new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))).String(),
	)

	caller.SetAmount(total)
	tx, err := caller.CallAndRetrieveResult("applyIoIDs", []any{
		new(big.Int).SetUint64(projectId),
		new(big.Int).SetUint64(amount)},
	)
	if err != nil {
		return output.NewError(output.SerializationError, "failed to call contract", err)
	}

	fmt.Printf("Apply ioID txHash: %s\n", tx)

	return nil
}
