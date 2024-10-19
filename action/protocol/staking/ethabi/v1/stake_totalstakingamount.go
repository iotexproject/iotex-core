package v1

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"

	"github.com/iotexproject/iotex-core/v2/action/protocol/abiutil"
	stakingComm "github.com/iotexproject/iotex-core/v2/action/protocol/staking/ethabi/common"
)

var _totalStakingAmountInterfaceABI = `[
	{
		"inputs": [],
		"name": "totalStakingAmount",
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

var _totalStakingAmountMethod abi.Method

func init() {
	_totalStakingAmountMethod = abiutil.MustLoadMethod(_totalStakingAmountInterfaceABI, "totalStakingAmount")
}

func newTotalStakingAmountContext() (*stakingComm.TotalStakingAmountStateContext, error) {
	return stakingComm.NewTotalStakingAmountContext(nil, &_totalStakingAmountMethod, iotexapi.ReadStakingDataMethod_TOTAL_STAKING_AMOUNT)
}
