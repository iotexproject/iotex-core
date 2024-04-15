package ethabi

import (
	"encoding/hex"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/pkg/util/addrutil"
)

var (
	errInvalidCallData  = errors.New("invalid call binary data")
	errInvalidCallSig   = errors.New("invalid call sig")
	errConvertBigNumber = errors.New("convert big number error")
	errDecodeFailure    = errors.New("decode data error")
)

type (
	// BucketEth struct for eth
	BucketEth struct {
		Index                     uint64
		CandidateAddress          common.Address
		StakedAmount              *big.Int
		StakedDuration            uint32
		CreateTime                int64
		StakeStartTime            int64
		UnstakeStartTime          int64
		AutoStake                 bool
		Owner                     common.Address
		ContractAddress           common.Address
		StakedDurationBlockNumber uint64
		CreateBlockHeight         uint64
		StakeStartBlockHeight     uint64
		UnstakeStartBlockHeight   uint64
	}

	// BucketTypeEth struct for eth
	BucketTypeEth struct {
		StakedAmount   *big.Int
		StakedDuration uint32
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

func encodeVoteBucketListToEth(outputs abi.Arguments, buckets *iotextypes.VoteBucketList) (string, error) {
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

		args[i].AutoStake = bucket.AutoStake
		addr, err = addrutil.IoAddrToEvmAddr(bucket.Owner)
		if err != nil {
			return "", err
		}
		args[i].Owner = addr

		if bucket.ContractAddress == "" {
			// native bucket contract address is 0x0000000000000000000000000000000000000000
			args[i].ContractAddress = common.Address{}
			args[i].StakedDuration = bucket.StakedDuration
			args[i].CreateTime = bucket.CreateTime.Seconds
			args[i].StakeStartTime = bucket.StakeStartTime.Seconds
			args[i].UnstakeStartTime = bucket.UnstakeStartTime.Seconds
		} else {
			addr, err = addrutil.IoAddrToEvmAddr(bucket.ContractAddress)
			if err != nil {
				return "", err
			}
			args[i].ContractAddress = addr
			args[i].StakedDurationBlockNumber = bucket.StakedDurationBlockNumber
			args[i].CreateBlockHeight = bucket.CreateBlockHeight
			args[i].StakeStartBlockHeight = bucket.StakeStartBlockHeight
			args[i].UnstakeStartBlockHeight = bucket.UnstakeStartBlockHeight
		}
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

func encodeBucketTypeListToEth(outputs abi.Arguments, bucketTypes *iotextypes.ContractStakingBucketTypeList) (string, error) {
	args := make([]BucketTypeEth, len(bucketTypes.BucketTypes))
	for i, bt := range bucketTypes.BucketTypes {
		args[i] = BucketTypeEth{}
		if amount, ok := new(big.Int).SetString(bt.StakedAmount, 10); ok {
			args[i].StakedAmount = amount
		} else {
			return "", errConvertBigNumber
		}
		args[i].StakedDuration = bt.StakedDuration
	}

	data, err := outputs.Pack(args)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}

// BuildReadStateRequest decode eth_call data to StateContext
func BuildReadStateRequest(data []byte) (protocol.StateContext, error) {
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
		return nil, errInvalidCallSig
	}
}
