package v1

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"

	"github.com/iotexproject/iotex-core/action/protocol/abiutil"
	stakingComm "github.com/iotexproject/iotex-core/action/protocol/staking/ethabi/common"
)

const _candidatesInterfaceABI = `[
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
		"name": "candidates",
		"outputs": [
			{
				"components": [
					{
						"internalType": "address",
						"name": "ownerAddress",
						"type": "address"
					},
					{
						"internalType": "address",
						"name": "operatorAddress",
						"type": "address"
					},
					{
						"internalType": "address",
						"name": "rewardAddress",
						"type": "address"
					},
					{
						"internalType": "string",
						"name": "name",
						"type": "string"
					},
					{
						"internalType": "uint256",
						"name": "totalWeightedVotes",
						"type": "uint256"
					},
					{
						"internalType": "uint64",
						"name": "selfStakeBucketIdx",
						"type": "uint64"
					},
					{
						"internalType": "uint256",
						"name": "selfStakingTokens",
						"type": "uint256"
					}
				],
				"internalType": "struct IStaking.Candidate[]",
				"name": "",
				"type": "tuple[]"
			}
		],
		"stateMutability": "view",
		"type": "function"
	}
]`

var _candidatesMethod abi.Method

func init() {
	_candidatesMethod = abiutil.MustLoadMethod(_candidatesInterfaceABI, "candidates")
}

func newCandidatesStateContext(data []byte) (*stakingComm.CandidatesStateContext, error) {
	return stakingComm.NewCandidatesStateContext(data, &_candidatesMethod, iotexapi.ReadStakingDataMethod_CANDIDATES, stakingComm.WithNoSelfStakeBucketIndexAsMaxUint32())
}
