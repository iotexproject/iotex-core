package ethabi

import (
	"encoding/hex"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/iotex-core/pkg/util/addrutil"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

var (
	errInvalidCallData  = errors.New("invalid call binary data")
	errInvalidCallSig   = errors.New("invalid call sig")
	errConvertBigNumber = errors.New("convert big number error")
	errDecodeFailure    = errors.New("decode data error")
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

	// BucketEth struct for eth
	BucketEth struct {
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

	// CandidateEth struct for eth
	CandidateEth struct {
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

func encodeVoteBucketListToEth(outputs abi.Arguments, buckets iotextypes.VoteBucketList) (string, error) {
	args := make([]BucketEth, len(buckets.Buckets))
	for i, bucket := range buckets.Buckets {
		args[i] = BucketEth{}
		args[i].Index = bucket.Index
		addr, err := addrutil.IoAddrToEvmAddr(bucket.CandidateAddress)
		if err != nil {
			return "", err
		}
		args[i].CandidateAddress = addr
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
		addr, err = addrutil.IoAddrToEvmAddr(bucket.Owner)
		if err != nil {
			return "", err
		}
		args[i].Owner = addr
	}

	data, err := outputs.Pack(args)
	if err != nil {
		return "", nil
	}
	return hex.EncodeToString(data), nil
}

func encodeCandidateToEth(candidate *iotextypes.CandidateV2) (*CandidateEth, error) {
	result := &CandidateEth{}
	addr, err := addrutil.IoAddrToEvmAddr(candidate.OwnerAddress)
	if err != nil {
		return nil, err
	}
	result.OwnerAddress = addr
	addr, err = addrutil.IoAddrToEvmAddr(candidate.OperatorAddress)
	if err != nil {
		return nil, err
	}
	result.OperatorAddress = addr
	addr, err = addrutil.IoAddrToEvmAddr(candidate.RewardAddress)
	if err != nil {
		return nil, err
	}
	result.RewardAddress = addr
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

// BuildReadStateRequest decode eth_call data to StateContext
func BuildReadStateRequest(data []byte) (StateContext, error) {
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
