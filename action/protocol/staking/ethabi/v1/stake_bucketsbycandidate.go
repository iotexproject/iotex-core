package v1

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"

	"github.com/iotexproject/iotex-core/action/protocol/abiutil"
	"github.com/iotexproject/iotex-core/action/protocol/staking/ethabi/common"
)

const _bucketsByCandidateInterfaceABI = `[
	{
		"inputs": [
			{
				"internalType": "string",
				"name": "candName",
				"type": "string"
			},
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
		"name": "bucketsByCandidate",
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

var _bucketsByCandidateMethod abi.Method

func init() {
	_bucketsByCandidateMethod = abiutil.MustLoadMethod(_bucketsByCandidateInterfaceABI, "bucketsByCandidate")
}

func newBucketsByCandidateStateContext(data []byte) (*common.BucketsByCandidateStateContext, error) {
	return common.NewBucketsByCandidateStateContext(data, &_bucketsByCandidateMethod, iotexapi.ReadStakingDataMethod_BUCKETS_BY_CANDIDATE)
}
