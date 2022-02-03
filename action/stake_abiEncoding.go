package action

import (
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
)

// StakingInterface is the interface of the abi encoding of stake action
var StakingInterface *abi.ABI

const stakingInterfaceABI = `[{"inputs":[{"internalType":"string","name":"candName","type":"string"},{"internalType":"uint256","name":"amount","type":"uint256"},{"internalType":"uint32","name":"duration","type":"uint32"},{"internalType":"bool","name":"autoStake","type":"bool"},{"internalType":"uint8[]","name":"data","type":"uint8[]"}],"name":"createStake","outputs":[],"stateMutability":"nonpayable","type":"function"}]`

func init() {
	parsedAbi, err := abi.JSON(strings.NewReader(stakingInterfaceABI))
	if err != nil {
		panic(err)
	}
	StakingInterface = &parsedAbi
}
