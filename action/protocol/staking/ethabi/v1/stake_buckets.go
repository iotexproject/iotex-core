package v1

import (
	"github.com/ethereum/go-ethereum/accounts/abi"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"

	"github.com/iotexproject/iotex-core/v2/action/protocol/abiutil"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/ethabi/common"
)

const _bucketsInterfaceABI = `[
	{
		"inputs": [
			{
				"internalType": "uint32",
				"name": "offset",
				"type": "uint32"
			},
			{
				"internalType": "uint32",
				"name": "limit",
				"type": "uint32"
			}
		],
		"name": "buckets",
		"outputs": [
			{
				"components": [
					{
						"internalType": "uint64",
						"name": "index",
						"type": "uint64"
					},
					{
						"internalType": "address",
						"name": "candidateAddress",
						"type": "address"
					},
					{
						"internalType": "uint256",
						"name": "stakedAmount",
						"type": "uint256"
					},
					{
						"internalType": "uint32",
						"name": "stakedDuration",
						"type": "uint32"
					},
					{
						"internalType": "int64",
						"name": "createTime",
						"type": "int64"
					},
					{
						"internalType": "int64",
						"name": "stakeStartTime",
						"type": "int64"
					},
					{
						"internalType": "int64",
						"name": "unstakeStartTime",
						"type": "int64"
					},
					{
						"internalType": "bool",
						"name": "autoStake",
						"type": "bool"
					},
					{
						"internalType": "address",
						"name": "owner",
						"type": "address"
					}
				],
				"internalType": "struct IStaking.VoteBucket[]",
				"name": "",
				"type": "tuple[]"
			}
		],
		"stateMutability": "view",
		"type": "function"
	}
]`

var _bucketsMethod abi.Method

func init() {
	_bucketsMethod = abiutil.MustLoadMethod(_bucketsInterfaceABI, "buckets")
}

func newBucketsStateContext(data []byte) (*common.BucketsStateContext, error) {
	return common.NewBucketsStateContext(data, &_bucketsMethod, iotexapi.ReadStakingDataMethod_BUCKETS)
}
