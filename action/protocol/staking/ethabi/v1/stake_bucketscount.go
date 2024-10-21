package v1

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"

	"github.com/iotexproject/iotex-core/v2/action/protocol/abiutil"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/ethabi/common"
)

var _bucketsCountInterfaceABI = `[
	{
		"inputs": [],
		"name": "bucketsCount",
		"outputs": [
			{
				"internalType": "uint64",
				"name": "total",
				"type": "uint64"
			},
			{
				"internalType": "uint64",
				"name": "active",
				"type": "uint64"
			}
		],
		"stateMutability": "view",
		"type": "function"
	}
]`

var _bucketsCountMethod abi.Method

func init() {
	_bucketsCountMethod = abiutil.MustLoadMethod(_bucketsCountInterfaceABI, "bucketsCount")
}

func newBucketsCountStateContext() (*common.BucketsCountStateContext, error) {
	return common.NewBucketsCountStateContext(nil, &_bucketsCountMethod, iotexapi.ReadStakingDataMethod_BUCKETS_COUNT)
}
