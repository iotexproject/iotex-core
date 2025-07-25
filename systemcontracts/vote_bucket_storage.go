package systemcontracts

import (
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/v2/pkg/log"
)

type SolidityVoteBucket struct {
	Index                        uint64
	CandidateAddress             string
	StakedAmount                 string
	StakedDuration               uint32
	CreateTime                   uint64 // Unix timestamp
	StakeStartTime               uint64 // Unix timestamp
	UnstakeStartTime             uint64 // Unix timestamp
	AutoStake                    bool
	Owner                        string
	ContractAddress              string
	StakedDurationBlockNumber    uint64
	CreateBlockHeight            uint64
	StakeStartBlockHeight        uint64
	UnstakeStartBlockHeight      uint64
	EndorsementExpireBlockHeight uint64
}

type (
	VoteBucketStorageContract struct {
		contractAddress common.Address
		backend         ContractBackend
		abi             abi.ABI
	}
)

// NewVoteBucketStorageContract creates a new VoteBucket storage contract instance
func NewVoteBucketStorageContract(contractAddress common.Address, backend ContractBackend) (*VoteBucketStorageContract, error) {
	abi, err := abi.JSON(strings.NewReader(VoteBucketStorageABI))
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse VoteBucketStorage ABI")
	}

	return &VoteBucketStorageContract{
		contractAddress: contractAddress,
		backend:         backend,
		abi:             abi,
	}, nil
}

// PutBuckets stores a complete vote bucket list in the contract
func (c *VoteBucketStorageContract) PutBuckets(buckets []*iotextypes.VoteBucket) error {
	// Convert iotextypes.VoteBucket to SolidityVoteBucket
	solidityBuckets := make([]SolidityVoteBucket, len(buckets))
	for i, bucket := range buckets {
		solidityBuckets[i] = SolidityVoteBucket{
			Index:                        bucket.Index,
			CandidateAddress:             bucket.CandidateAddress,
			StakedAmount:                 bucket.StakedAmount,
			StakedDuration:               bucket.StakedDuration,
			CreateTime:                   timestampToUnix(bucket.CreateTime),
			StakeStartTime:               timestampToUnix(bucket.StakeStartTime),
			UnstakeStartTime:             timestampToUnix(bucket.UnstakeStartTime),
			AutoStake:                    bucket.AutoStake,
			Owner:                        bucket.Owner,
			ContractAddress:              bucket.ContractAddress,
			StakedDurationBlockNumber:    bucket.StakedDurationBlockNumber,
			CreateBlockHeight:            bucket.CreateBlockHeight,
			StakeStartBlockHeight:        bucket.StakeStartBlockHeight,
			UnstakeStartBlockHeight:      bucket.UnstakeStartBlockHeight,
			EndorsementExpireBlockHeight: bucket.EndorsementExpireBlockHeight,
		}
	}

	// Pack the function call
	data, err := c.abi.Pack("putBuckets", solidityBuckets)
	if err != nil {
		return errors.Wrap(err, "failed to pack putBuckets call")
	}

	// Execute the transaction
	callMsg := &ethereum.CallMsg{
		To:    &c.contractAddress,
		Data:  data,
		Value: big.NewInt(0),
		Gas:   1000000,
	}

	if err := c.backend.Handle(callMsg); err != nil {
		return errors.Wrap(err, "failed to execute putBuckets")
	}

	log.L().Debug("Successfully stored vote buckets",
		zap.Int("count", len(buckets)))

	return nil
}

// GetBuckets retrieves vote buckets with pagination from the contract
func (c *VoteBucketStorageContract) GetBuckets(offset, limit uint64) (*iotextypes.VoteBucketList, error) {
	// Pack the function call
	data, err := c.abi.Pack("getBuckets", new(big.Int).SetUint64(offset), new(big.Int).SetUint64(limit))
	if err != nil {
		return nil, errors.Wrap(err, "failed to pack getBuckets call")
	}

	// Execute the call
	callMsg := &ethereum.CallMsg{
		To:    &c.contractAddress,
		Data:  data,
		Value: big.NewInt(0),
		Gas:   1000000,
	}

	result, err := c.backend.Call(callMsg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to call getBuckets")
	}

	// Unpack the result using UnpackIntoInterface for better type safety
	var getBucketsResult struct {
		BucketList []SolidityVoteBucket
		Total      *big.Int
	}

	err = c.abi.UnpackIntoInterface(&getBucketsResult, "getBuckets", result)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unpack getBuckets result into interface")
	}

	solidityBuckets := getBucketsResult.BucketList
	total := getBucketsResult.Total

	// Convert to iotextypes.VoteBucket
	buckets := make([]*iotextypes.VoteBucket, len(solidityBuckets))
	for i, sb := range solidityBuckets {
		buckets[i] = &iotextypes.VoteBucket{
			Index:                        sb.Index,
			CandidateAddress:             sb.CandidateAddress,
			StakedAmount:                 sb.StakedAmount,
			StakedDuration:               sb.StakedDuration,
			CreateTime:                   unixToTimestamp(sb.CreateTime),
			StakeStartTime:               unixToTimestamp(sb.StakeStartTime),
			UnstakeStartTime:             unixToTimestamp(sb.UnstakeStartTime),
			AutoStake:                    sb.AutoStake,
			Owner:                        sb.Owner,
			ContractAddress:              sb.ContractAddress,
			StakedDurationBlockNumber:    sb.StakedDurationBlockNumber,
			CreateBlockHeight:            sb.CreateBlockHeight,
			StakeStartBlockHeight:        sb.StakeStartBlockHeight,
			UnstakeStartBlockHeight:      sb.UnstakeStartBlockHeight,
			EndorsementExpireBlockHeight: sb.EndorsementExpireBlockHeight,
		}
	}

	log.L().Debug("Successfully retrieved vote buckets",
		zap.Uint64("offset", offset),
		zap.Uint64("limit", limit),
		zap.Int("returned", len(buckets)),
		zap.String("total", total.String()))

	return &iotextypes.VoteBucketList{
		Buckets: buckets,
	}, nil
}

// Helper functions to convert between protobuf Timestamp and Unix timestamp
func timestampToUnix(ts *timestamppb.Timestamp) uint64 {
	if ts == nil {
		return 0
	}
	return uint64(ts.AsTime().Unix())
}

func unixToTimestamp(unix uint64) *timestamppb.Timestamp {
	if unix == 0 {
		return nil
	}
	return timestamppb.New(time.Unix(int64(unix), 0))
}
