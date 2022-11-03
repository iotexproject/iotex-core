package action

import (
	"encoding/hex"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/iotex-core/pkg/util/addrutil"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

var (
	errInvalidCallData  = errors.New("invalid call binary data")
	errInvalidCallSig   = errors.New("invalid call sig")
	errConvertBigNumber = errors.New("convert big number error")
)

type (
	// Parameters state request parameters
	Parameters struct {
		MethodName []byte
		Arguments  [][]byte
	}

	// StateContext context for ReadState
	StateContext interface {
		Parameters() *Parameters
		EncodeToEth(*iotexapi.ReadStateResponse) (string, error)
	}

	baseStateContext struct {
		parameters *Parameters
	}

	// Bucket struct for eth
	Bucket struct {
		Index            uint64
		CandidateAddress common.Address
		StakedAmount     *big.Int
		StakedDuration   uint32
		CreateTime       int64
		StakeStartTime   int64
		UnstakeStartTime int64
		AutoStake        bool
		Owner            common.Address
	}

	// Candidate struct for eth
	Candidate struct {
		OwnerAddress       common.Address
		OperatorAddress    common.Address
		RewardAddress      common.Address
		Name               string
		TotalWeightedVotes *big.Int
		SelfStakeBucketIdx uint64
		SelfStakingTokens  *big.Int
	}
)

func (r *baseStateContext) Parameters() *Parameters {
	return r.parameters
}

func encodeVoteBucketListToEth(buckets iotextypes.VoteBucketList) (string, error) {
	args := make([]Bucket, len(buckets.Buckets))
	for i, bucket := range buckets.Buckets {
		args[i] = Bucket{}
		args[i].Index = bucket.Index
		if addr, err := addrutil.IoAddrToEvmAddr(bucket.CandidateAddress); err == nil {
			args[i].CandidateAddress = addr
		} else {
			return "", err
		}
		if amount, ok := new(big.Int).SetString(bucket.StakedAmount, 10); ok {
			args[i].StakedAmount = amount
		} else {
			return "", errConvertBigNumber
		}
		args[i].StakedDuration = bucket.StakedDuration
		args[i].CreateTime = bucket.CreateTime.Seconds
		args[i].StakeStartTime = bucket.StakeStartTime.Seconds
		args[i].UnstakeStartTime = bucket.UnstakeStartTime.Seconds
		args[i].AutoStake = bucket.AutoStake
		if addr, err := addrutil.IoAddrToEvmAddr(bucket.Owner); err == nil {
			args[i].Owner = addr
		} else {
			return "", err
		}
	}

	data, err := _bucketsMethod.Outputs.Pack(args)
	if err != nil {
		return "", nil
	}
	return hex.EncodeToString(data), nil
}

func encodeCandidateToEth(candidate *iotextypes.CandidateV2) (*Candidate, error) {
	result := &Candidate{}
	if addr, err := addrutil.IoAddrToEvmAddr(candidate.OwnerAddress); err == nil {
		result.OwnerAddress = addr
	} else {
		return nil, err
	}
	if addr, err := addrutil.IoAddrToEvmAddr(candidate.OperatorAddress); err == nil {
		result.OperatorAddress = addr
	} else {
		return nil, err
	}
	if addr, err := addrutil.IoAddrToEvmAddr(candidate.RewardAddress); err == nil {
		result.RewardAddress = addr
	} else {
		return nil, err
	}
	result.Name = candidate.Name
	if amount, ok := new(big.Int).SetString(candidate.TotalWeightedVotes, 10); ok {
		result.TotalWeightedVotes = amount
	} else {
		return nil, errConvertBigNumber
	}
	result.SelfStakeBucketIdx = candidate.SelfStakeBucketIdx
	if amount, ok := new(big.Int).SetString(candidate.SelfStakingTokens, 10); ok {
		result.SelfStakingTokens = amount
	} else {
		return nil, errConvertBigNumber
	}
	return result, nil
}

// CallDataToStakeStateContext decode eth_call data to StateContext
func CallDataToStakeStateContext(data []byte) (StateContext, error) {
	if len(data) < 4 {
		return nil, errInvalidCallData
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
		return nil, errInvalidCallSig
	}
}
