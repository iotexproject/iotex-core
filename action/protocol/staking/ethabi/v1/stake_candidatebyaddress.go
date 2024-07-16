package v1

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"

	"github.com/iotexproject/iotex-core/action/protocol/abiutil"
	stakingComm "github.com/iotexproject/iotex-core/action/protocol/staking/ethabi/common"
)

const _candidateByAddressInterfaceABI = `[
	{
		"inputs": [
			{
				"internalType": "address",
				"name": "ownerAddress",
				"type": "address"
			}
		],
		"name": "candidateByAddress",
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
				"internalType": "struct IStaking.Candidate",
				"name": "",
				"type": "tuple"
			}
		],
		"stateMutability": "view",
		"type": "function"
	}
]`

var _candidateByAddressMethod abi.Method

func init() {
	_candidateByAddressMethod = abiutil.MustLoadMethod(_candidateByAddressInterfaceABI, "candidateByAddress")
}

func newCandidateByAddressStateContext(data []byte) (*stakingComm.CandidateByAddressStateContext, error) {
	return stakingComm.NewCandidateByAddressStateContext(data, &_candidateByAddressMethod, iotexapi.ReadStakingDataMethod_CANDIDATE_BY_ADDRESS, stakingComm.WithNoSelfStakeBucketIndexAsMaxUint32())

}
