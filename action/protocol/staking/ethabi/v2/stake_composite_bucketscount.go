package v2

import (
	"github.com/ethereum/go-ethereum/accounts/abi"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"

	"github.com/iotexproject/iotex-core/v2/action/protocol/abiutil"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/ethabi/common"
)

const _compositeBucketsCountInterfaceABI = `[
	{
		"inputs": [],
		"name": "compositeBucketsCount",
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

var _compositeBucketsCountMethod abi.Method

func init() {
	_compositeBucketsCountMethod = abiutil.MustLoadMethod(_compositeBucketsCountInterfaceABI, "compositeBucketsCount")
}

func newCompositeBucketsCountStateContext() (*common.BucketsCountStateContext, error) {
	return common.NewBucketsCountStateContext(nil, &_compositeBucketsCountMethod, iotexapi.ReadStakingDataMethod_COMPOSITE_BUCKETS_COUNT)
}
