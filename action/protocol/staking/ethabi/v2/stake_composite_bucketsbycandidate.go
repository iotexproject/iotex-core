package v2

import (
	"github.com/ethereum/go-ethereum/accounts/abi"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"

	"github.com/iotexproject/iotex-core/v2/action/protocol/abiutil"
	stakingComm "github.com/iotexproject/iotex-core/v2/action/protocol/staking/ethabi/common"
)

const _compositeBucketsByCandidateInterfaceABI = `[
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
		"name": "compositeBucketsByCandidate",
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
					},
					{
						"internalType": "address",
						"name": "contractAddress",
						"type": "address"
					},
					{
						"internalType": "uint64",
						"name": "stakedDurationBlockNumber",
						"type": "uint64"
					},
					{
						"internalType": "uint64",
						"name": "createBlockHeight",
						"type": "uint64"
					},
					{
						"internalType": "uint64",
						"name": "stakeStartBlockHeight",
						"type": "uint64"
					},
					{
						"internalType": "uint64",
						"name": "unstakeStartBlockHeight",
						"type": "uint64"
					}
				],
				"internalType": "struct IStaking.CompositeVoteBucket[]",
				"name": "",
				"type": "tuple[]"
			}
		],
		"stateMutability": "view",
		"type": "function"
	}
]`

var _compositeBucketsByCandidateMethod abi.Method

func init() {
	_compositeBucketsByCandidateMethod = abiutil.MustLoadMethod(_compositeBucketsByCandidateInterfaceABI, "compositeBucketsByCandidate")
}

func newCompositeBucketsByCandidateStateContext(data []byte) (*stakingComm.BucketsByCandidateStateContext, error) {
	return stakingComm.NewBucketsByCandidateStateContext(data, &_compositeBucketsByCandidateMethod, iotexapi.ReadStakingDataMethod_COMPOSITE_BUCKETS_BY_CANDIDATE)
}
