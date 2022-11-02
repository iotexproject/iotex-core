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
	ErrInvalidCallData  = errors.New("invalid call binary data")
	ErrInvalidCallSig   = errors.New("invalid call sig")
	ErrDecodeFailure    = errors.New("decode call data error")
	ErrConvertBigNumber = errors.New("convert big number error")
)

type (
	Parameters struct {
		MethodName []byte
		Arguments  [][]byte
	}

	StateContext interface {
		Parameters() *Parameters
		EncodeToEth(*iotexapi.ReadStateResponse) (string, error)
	}

	baseStateContext struct {
		parameters *Parameters
	}

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
			return "", ErrConvertBigNumber
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

func CallDataToStakeStateContext(data []byte) (StateContext, error) {
	if len(data) < 4 {
		return nil, ErrInvalidCallData
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
	default:
		return nil, ErrInvalidCallSig
	}
}
