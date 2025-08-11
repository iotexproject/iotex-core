package action

import (
	_ "embed"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/accounts/abi"
)

var (
	// NativeStakingContractJSONABI is the JSON ABI of the native staking contract
	//go:embed native_staking_contract_abi.json
	NativeStakingContractJSONABI string
	// nativeStakingContractABI is the parsed ABI of the native staking contract
	nativeStakingContractABI abi.ABI

	once sync.Once
)

func initNativeStakingContractABI() {
	once.Do(func() {
		var err error
		nativeStakingContractABI, err = abi.JSON(strings.NewReader(NativeStakingContractJSONABI))
		if err != nil {
			panic("failed to parse native staking contract ABI: " + err.Error())
		}
	})
}

func NativeStakingContractABI() *abi.ABI {
	// Ensure the ABI is initialized before returning
	initNativeStakingContractABI()
	return &nativeStakingContractABI
}
