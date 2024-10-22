package v2

import (
	"github.com/ethereum/go-ethereum/accounts/abi"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"

	"github.com/iotexproject/iotex-core/v2/action/protocol/abiutil"
	stakingComm "github.com/iotexproject/iotex-core/v2/action/protocol/staking/ethabi/common"
)

const _compositeTotalStakingAmountInterfaceABI = `[
	{
		"inputs": [],
		"name": "compositeTotalStakingAmount",
		"outputs": [
			{
				"internalType": "uint256",
				"name": "",
				"type": "uint256"
			}
		],
		"stateMutability": "view",
		"type": "function"
	}
]`

var _compositeTotalStakingAmountMethod abi.Method

func init() {
	_compositeTotalStakingAmountMethod = abiutil.MustLoadMethod(_compositeTotalStakingAmountInterfaceABI, "compositeTotalStakingAmount")
}

func newCompositeTotalStakingAmountContext() (*stakingComm.TotalStakingAmountStateContext, error) {
	return stakingComm.NewTotalStakingAmountContext(nil, &_compositeTotalStakingAmountMethod, iotexapi.ReadStakingDataMethod_COMPOSITE_TOTAL_STAKING_AMOUNT)
}
