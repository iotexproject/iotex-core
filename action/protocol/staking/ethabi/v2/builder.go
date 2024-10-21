package v2

import (
	"encoding/hex"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	stakingComm "github.com/iotexproject/iotex-core/v2/action/protocol/staking/ethabi/common"
)

func BuildReadStateRequest(data []byte) (protocol.StateContext, error) {
	switch methodSig := hex.EncodeToString(data[:4]); methodSig {
	case hex.EncodeToString(_compositeBucketsMethod.ID):
		return newCompositeBucketsStateContext(data[4:])
	case hex.EncodeToString(_compositeBucketsByCandidateMethod.ID):
		return newCompositeBucketsByCandidateStateContext(data[4:])
	case hex.EncodeToString(_compositeBucketsByIndexesMethod.ID):
		return newCompositeBucketsByIndexesStateContext(data[4:])
	case hex.EncodeToString(_compositeBucketsByVoterMethod.ID):
		return newCompositeBucketsByVoterStateContext(data[4:])
	case hex.EncodeToString(_compositeBucketsCountMethod.ID):
		return newCompositeBucketsCountStateContext()
	case hex.EncodeToString(_compositeTotalStakingAmountMethod.ID):
		return newCompositeTotalStakingAmountContext()
	case hex.EncodeToString(_contractBucketTypesMethod.ID):
		return newContractBucketTypesStateContext(data[4:])
	default:
		return nil, stakingComm.ErrInvalidCallSig
	}
}
