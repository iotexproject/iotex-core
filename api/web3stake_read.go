package api

import (
	"encoding/hex"
	"errors"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/iotex-core/pkg/util/addrutil"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"google.golang.org/protobuf/proto"
)

var (
	ErrInvalidCallData  = errors.New("invalid call binary data")
	ErrInvalidCallSig   = errors.New("invalid call sig")
	ErrDecodeFailure    = errors.New("decode call data error")
	ErrConvertBigNumber = errors.New("convert big number  error")
)

const (
	_bucketsInterfaceABI = `[
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
			"name": "buckets",
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
						}
					],
					"internalType": "struct IStaking.VoteBucket[]",
					"name": "",
					"type": "tuple[]"
				}
			],
			"stateMutability": "view",
			"type": "function"
		},
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
			"name": "bucketsByCandidate",
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
						}
					],
					"internalType": "struct IStaking.VoteBucket[]",
					"name": "",
					"type": "tuple[]"
				}
			],
			"stateMutability": "view",
			"type": "function"
		},
		{
			"inputs": [
				{
					"internalType": "uint64[]",
					"name": "indexes",
					"type": "uint64[]"
				}
			],
			"name": "bucketsByIndexes",
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
						}
					],
					"internalType": "struct IStaking.VoteBucket[]",
					"name": "",
					"type": "tuple[]"
				}
			],
			"stateMutability": "view",
			"type": "function"
		},
		{
			"inputs": [
				{
					"internalType": "address",
					"name": "voter",
					"type": "address"
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
			"name": "bucketsByVoter",
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
						}
					],
					"internalType": "struct IStaking.VoteBucket[]",
					"name": "",
					"type": "tuple[]"
				}
			],
			"stateMutability": "view",
			"type": "function"
		},
		{
			"inputs": [],
			"name": "bucketsCount",
			"outputs": [
				{
					"internalType": "uint64",
					"name": "total",
					"type": "uint64"
				},
				{
					"internalType": "uint64",
					"name": "active",
					"type": "uint64"
				}
			],
			"stateMutability": "view",
			"type": "function"
		}
	]`
)

var (
	_bucketsMethod            abi.Method
	_bucketsByVoterMethod     abi.Method
	_bucketsByCandidateMethod abi.Method
	_bucketsByIndexesMethod   abi.Method
	_bucketsCountMethod       abi.Method
)

func init() {
	stakeBucketsInterface, err := abi.JSON(strings.NewReader(_bucketsInterfaceABI))
	if err != nil {
		panic(err)
	}
	var ok bool
	_bucketsMethod, ok = stakeBucketsInterface.Methods["buckets"]
	if !ok {
		panic("fail to load the method")
	}
	_bucketsByVoterMethod, ok = stakeBucketsInterface.Methods["bucketsByVoter"]
	if !ok {
		panic("fail to load the method")
	}
	_bucketsByCandidateMethod, ok = stakeBucketsInterface.Methods["bucketsByCandidate"]
	if !ok {
		panic("fail to load the method")
	}
	_bucketsByIndexesMethod, ok = stakeBucketsInterface.Methods["bucketsByIndexes"]
	if !ok {
		panic("fail to load the method")
	}
	_bucketsCountMethod, ok = stakeBucketsInterface.Methods["bucketsCount"]
	if !ok {
		panic("fail to load the method")
	}
}

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

	BucketsStateContext struct {
		*baseStateContext
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

func newBucketsStateContext(data []byte) (*BucketsStateContext, error) {
	paramsMap := map[string]interface{}{}
	ok := false
	if err := _bucketsMethod.Inputs.UnpackIntoMap(paramsMap, data); err != nil {
		return nil, err
	}
	var offset, limit uint32
	if offset, ok = paramsMap["offset"].(uint32); !ok {
		return nil, ErrDecodeFailure
	}
	if limit, ok = paramsMap["limit"].(uint32); !ok {
		return nil, ErrDecodeFailure
	}

	method := &iotexapi.ReadStakingDataMethod{
		Method: iotexapi.ReadStakingDataMethod_BUCKETS_BY_VOTER,
	}
	methodBytes, err := proto.Marshal(method)
	if err != nil {
		return nil, err
	}
	arguments := &iotexapi.ReadStakingDataRequest{
		Request: &iotexapi.ReadStakingDataRequest_Buckets{
			Buckets: &iotexapi.ReadStakingDataRequest_VoteBuckets{
				Pagination: &iotexapi.PaginationParam{
					Offset: offset,
					Limit:  limit,
				},
			},
		},
	}
	argumentsBytes, err := proto.Marshal(arguments)
	if err != nil {
		return nil, err
	}
	return &BucketsStateContext{
		&baseStateContext{
			&Parameters{
				MethodName: methodBytes,
				Arguments:  [][]byte{argumentsBytes},
			},
		},
	}, nil
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

func (r *BucketsStateContext) EncodeToEth(resp *iotexapi.ReadStateResponse) (string, error) {
	var result iotextypes.VoteBucketList
	if err := proto.Unmarshal(resp.Data, &result); err != nil {
		return "", err
	}

	return encodeVoteBucketListToEth(result)
}

func CallDataToStakeStateContext(data []byte) (StateContext, error) {
	if len(data) <= 4 {
		return nil, ErrInvalidCallData
	}

	switch methodSig := hex.EncodeToString(data[:4]); methodSig {
	case hex.EncodeToString(_bucketsMethod.ID):
		return newBucketsStateContext(data[4:])
	default:
		return nil, ErrInvalidCallSig
	}
}
