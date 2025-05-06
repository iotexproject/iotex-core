package common

import (
	"encoding/hex"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/v2/pkg/util/addrutil"
)

type (
	// BucketEth struct for eth
	BucketEth struct {
		Index                        uint64
		CandidateAddress             common.Address
		StakedAmount                 *big.Int
		StakedDuration               uint32
		CreateTime                   int64
		StakeStartTime               int64
		UnstakeStartTime             int64
		AutoStake                    bool
		Owner                        common.Address
		ContractAddress              common.Address
		StakedDurationBlockNumber    uint64
		CreateBlockHeight            uint64
		StakeStartBlockHeight        uint64
		UnstakeStartBlockHeight      uint64
		EndorsementExpireBlockHeight uint64
	}

	// BucketTypeEth struct for eth
	BucketTypeEth struct {
		StakedAmount   *big.Int
		StakedDuration uint32
	}

	// CandidateEth struct for eth
	CandidateEth struct {
		Id                 common.Address
		OwnerAddress       common.Address
		OperatorAddress    common.Address
		RewardAddress      common.Address
		Name               string
		TotalWeightedVotes *big.Int
		SelfStakeBucketIdx uint64
		SelfStakingTokens  *big.Int
	}
)

func EncodeVoteBucketListToEth(outputs abi.Arguments, buckets *iotextypes.VoteBucketList) (string, error) {
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
			return "", ErrConvertBigNumber
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
			args[i].EndorsementExpireBlockHeight = bucket.EndorsementExpireBlockHeight
		} else {
			addr, err = addrutil.IoAddrToEvmAddr(bucket.ContractAddress)
			if err != nil {
				return "", err
			}
			args[i].ContractAddress = addr
		}
		if bucket.CreateBlockHeight == 0 {
			args[i].StakedDuration = bucket.StakedDuration
			args[i].CreateTime = bucket.CreateTime.Seconds
			args[i].StakeStartTime = bucket.StakeStartTime.Seconds
			args[i].UnstakeStartTime = bucket.UnstakeStartTime.Seconds
		} else {
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

func EncodeCandidateToEth(candidate *iotextypes.CandidateV2) (*CandidateEth, error) {
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
		return nil, ErrConvertBigNumber
	}
	result.SelfStakeBucketIdx = candidate.SelfStakeBucketIdx
	if amount, ok := new(big.Int).SetString(candidate.SelfStakingTokens, 10); ok {
		result.SelfStakingTokens = amount
	} else {
		return nil, ErrConvertBigNumber
	}
	addr, err = addrutil.IoAddrToEvmAddr(candidate.Id)
	if err != nil {
		return nil, err
	}
	result.Id = addr
	return result, nil
}

func EncodeBucketTypeListToEth(outputs abi.Arguments, bucketTypes *iotextypes.ContractStakingBucketTypeList) (string, error) {
	args := make([]BucketTypeEth, len(bucketTypes.BucketTypes))
	for i, bt := range bucketTypes.BucketTypes {
		args[i] = BucketTypeEth{}
		if amount, ok := new(big.Int).SetString(bt.StakedAmount, 10); ok {
			args[i].StakedAmount = amount
		} else {
			return "", ErrConvertBigNumber
		}
		args[i].StakedDuration = bt.StakedDuration
	}

	data, err := outputs.Pack(args)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}
