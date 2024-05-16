package v1

import (
	"encoding/hex"

	"github.com/iotexproject/iotex-core/action/protocol"
	stakingComm "github.com/iotexproject/iotex-core/action/protocol/staking/ethabi/common"
)

func BuildReadStateRequest(data []byte) (protocol.StateContext, error) {
	if len(data) < 4 {
		return nil, stakingComm.ErrInvalidCallData
	}

	switch methodSig := hex.EncodeToString(data[:4]); methodSig {
	case hex.EncodeToString(_bucketsMethod.ID):
		return newBucketsStateContext(data[4:])
	case hex.EncodeToString(_bucketsByCandidateMethod.ID):
		return newBucketsByCandidateStateContext(data[4:])
	case hex.EncodeToString(_bucketsByIndexesMethod.ID):
		return newBucketsByIndexesStateContext(data[4:])
	case hex.EncodeToString(_bucketsByVoterMethod.ID):
		return newBucketsByVoterStateContext(data[4:])
	case hex.EncodeToString(_bucketsCountMethod.ID):
		return newBucketsCountStateContext()
	case hex.EncodeToString(_candidatesMethod.ID):
		return newCandidatesStateContext(data[4:])
	case hex.EncodeToString(_candidateByNameMethod.ID):
		return newCandidateByNameStateContext(data[4:])
	case hex.EncodeToString(_candidateByAddressMethod.ID):
		return newCandidateByAddressStateContext(data[4:])
	case hex.EncodeToString(_totalStakingAmountMethod.ID):
		return newTotalStakingAmountContext()
	default:
		return nil, stakingComm.ErrInvalidCallSig
	}
}
