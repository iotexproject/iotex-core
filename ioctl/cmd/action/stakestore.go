package action

import (
	"encoding/hex"
	"math/big"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

// stakeStoreCmd represents the stake store command
var stakeStoreCmd = &cobra.Command{
	// PYGG_INDEX is  BUCKET_INDEX
	Use: "store IOTX_AMOUNT PYGG_INDEX [DATA] [-c ALIAS|CONTRACT_ADDRESS]" +
		" [-s SIGNER] [-n NONCE] [-l GAS_LIMIT] [-p GASPRICE] [-P PASSWORD] [-y]",
	Short: "Store IOTX to pygg on IoTeX blockchain",
	Args:  cobra.RangeArgs(2, 3),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := store(args)
		return output.PrintError(err)
	},
}

func init() {
	registerWriteCommand(stakeStoreCmd)
}

func store(args []string) error {
	amount, err := util.StringToRau(args[0], util.IotxDecimalNum)
	if err != nil {
		return output.NewError(output.ConvertError, "invalid IOTX amount", err)
	}

	pyggIndex, ok := new(big.Int).SetString(args[1], 10)
	if !ok {
		return output.NewError(output.ConvertError, "failed to convert pygg index", nil)
	}

	data := []byte{}
	if len(args) == 3 {
		data = make([]byte, 2*len([]byte(args[2])))
		hex.Encode(data, []byte(args[2]))
	}

	contract, err := stakingContract()
	if err != nil {
		return output.NewError(output.AddressError, "failed to get contract address", err)
	}

	bytecode, err := stakeABI.Pack("storeToPygg", pyggIndex, data)
	if err != nil {
		return output.NewError(output.ConvertError, "cannot generate bytecode from given command", err)
	}

	return Execute(contract.String(), amount, bytecode)
}
