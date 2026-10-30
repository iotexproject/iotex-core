package action

import (
	_ "embed"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/accounts/abi"
)

// IMPORTANT: native_staking_contract_abi.json and
// native_staking_contract_interface.sol are NOT auto-generated from each
// other. The JSON is what iotex-core actually loads at runtime; the .sol
// file is the human-readable source of truth for clients / docs. Adding a
// new method, parameter, or event requires editing BOTH files by hand,
// otherwise the web3 path silently misencodes: clients embed one signature
// while the node decodes another. There is no Makefile target that catches
// the drift — keep an eye on it during review.

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
