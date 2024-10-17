package v1

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"

	"github.com/iotexproject/iotex-core/v2/action/protocol/abiutil"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/ethabi/common"
)

const _bucketsByIndexesInterfaceABI = `[
	{
		"inputs": [
			{
				"internalType": "uint64[]",
				"name": "indexes",
				"type": "uint64[]"
			}
		],
		"name": "bucketsByIndexes",
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

var _bucketsByIndexesMethod abi.Method

func init() {
	_bucketsByIndexesMethod = abiutil.MustLoadMethod(_bucketsByIndexesInterfaceABI, "bucketsByIndexes")
}

func newBucketsByIndexesStateContext(data []byte) (*common.BucketsByIndexesStateContext, error) {
	return common.NewBucketsByIndexesStateContext(data, &_bucketsByIndexesMethod, iotexapi.ReadStakingDataMethod_BUCKETS_BY_INDEXES)
}
