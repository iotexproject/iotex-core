// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"github.com/golang/protobuf/proto"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/pkg/log"
)

var (
	// InContractTransfer is topic for implicit transfer log of evm transfer
	// 32 bytes with all zeros
	InContractTransfer = hash.BytesToHash256([]byte{byte(iotextypes.ImplicitTransferLogType_IN_CONTRACT_TRANSFER)})

	// BucketWithdrawAmount is topic for bucket withdraw
	BucketWithdrawAmount = hash.BytesToHash256([]byte{byte(iotextypes.ImplicitTransferLogType_BUCKET_WITHDRAW_AMOUNT)})

	// BucketCreateAmount is topic for bucket create
	BucketCreateAmount = hash.BytesToHash256([]byte{byte(iotextypes.ImplicitTransferLogType_BUCKET_CREATE_AMOUNT)})

	// BucketDepositAmount is topic for bucket deposit
	BucketDepositAmount = hash.BytesToHash256([]byte{byte(iotextypes.ImplicitTransferLogType_BUCKET_DEPOSIT_AMOUNT)})

	// CandidateSelfStake is topic for candidate self-stake
	CandidateSelfStake = hash.BytesToHash256([]byte{byte(iotextypes.ImplicitTransferLogType_CANDIDATE_SELF_STAKE)})

	// CandidateRegistrationFee is topic for candidate register
	CandidateRegistrationFee = hash.BytesToHash256([]byte{byte(iotextypes.ImplicitTransferLogType_CANDIDATE_REGISTRATION_FEE)})

	// StakingBucketPoolTopic is topic for staking bucket pool
	StakingBucketPoolTopic = hash.BytesToHash256(address.StakingProtocolAddrHash[:])

	// RewardingPoolTopic is topic for rewarding pool
	RewardingPoolTopic = hash.BytesToHash256(address.RewardingProtocolAddrHash[:])
)

type (
	// Topics are data items of a transaction, such as send/recipient address
	Topics []hash.Hash256

	// Receipt represents the result of a contract
	Receipt struct {
		Status          uint64
		BlockHeight     uint64
		ActionHash      hash.Hash256
		GasConsumed     uint64
		ContractAddress string
		Logs            []*Log
	}

	// Log stores an evm contract event
	Log struct {
		Address            string
		Topics             Topics
		Data               []byte
		BlockHeight        uint64
		ActionHash         hash.Hash256
		Index              uint
		NotFixTopicCopyBug bool
		Sender             string
		Recipient          string
	}
)

// ConvertToReceiptPb converts a Receipt to protobuf's Receipt
func (receipt *Receipt) ConvertToReceiptPb() *iotextypes.Receipt {
	r := &iotextypes.Receipt{}
	r.Status = receipt.Status
	r.BlkHeight = receipt.BlockHeight
	r.ActHash = receipt.ActionHash[:]
	r.GasConsumed = receipt.GasConsumed
	r.ContractAddress = receipt.ContractAddress
	r.Logs = []*iotextypes.Log{}
	for _, l := range receipt.Logs {
		// exclude implict transfer log when calculating receipts' hash or storing logs
		if !l.IsImplicitTransfer() {
			r.Logs = append(r.Logs, l.ConvertToLogPb())
		}
	}
	return r
}

// ConvertFromReceiptPb converts a protobuf's Receipt to Receipt
func (receipt *Receipt) ConvertFromReceiptPb(pbReceipt *iotextypes.Receipt) {
	receipt.Status = pbReceipt.GetStatus()
	receipt.BlockHeight = pbReceipt.GetBlkHeight()
	copy(receipt.ActionHash[:], pbReceipt.GetActHash())
	receipt.GasConsumed = pbReceipt.GetGasConsumed()
	receipt.ContractAddress = pbReceipt.GetContractAddress()
	logs := pbReceipt.GetLogs()
	receipt.Logs = make([]*Log, len(logs))
	for i, log := range logs {
		receipt.Logs[i] = &Log{}
		receipt.Logs[i].ConvertFromLogPb(log)
	}
}

// Serialize returns a serialized byte stream for the Receipt
func (receipt *Receipt) Serialize() ([]byte, error) {
	return proto.Marshal(receipt.ConvertToReceiptPb())
}

// Deserialize parse the byte stream into Receipt
func (receipt *Receipt) Deserialize(buf []byte) error {
	pbReceipt := &iotextypes.Receipt{}
	if err := proto.Unmarshal(buf, pbReceipt); err != nil {
		return err
	}
	receipt.ConvertFromReceiptPb(pbReceipt)
	return nil
}

// Hash returns the hash of receipt
func (receipt *Receipt) Hash() hash.Hash256 {
	data, err := receipt.Serialize()
	if err != nil {
		log.L().Panic("Error when serializing a receipt")
	}
	return hash.Hash256b(data)
}

// ConvertToLogPb converts a Log to protobuf's Log
func (log *Log) ConvertToLogPb() *iotextypes.Log {
	l := &iotextypes.Log{}
	l.ContractAddress = log.Address
	l.Topics = [][]byte{}
	for _, topic := range log.Topics {
		if log.NotFixTopicCopyBug {
			l.Topics = append(l.Topics, topic[:])
		} else {
			data := make([]byte, len(topic))
			copy(data, topic[:])
			l.Topics = append(l.Topics, data)
		}
	}
	l.Data = log.Data
	l.BlkHeight = log.BlockHeight
	l.ActHash = log.ActionHash[:]
	l.Index = uint32(log.Index)
	return l
}

// ConvertFromLogPb converts a protobuf's LogPb to Log
func (log *Log) ConvertFromLogPb(pbLog *iotextypes.Log) {
	log.Address = pbLog.GetContractAddress()
	pbLogs := pbLog.GetTopics()
	log.Topics = make([]hash.Hash256, len(pbLogs))
	for i, topic := range pbLogs {
		copy(log.Topics[i][:], topic)
	}
	log.Data = pbLog.GetData()
	log.BlockHeight = pbLog.GetBlkHeight()
	copy(log.ActionHash[:], pbLog.GetActHash())
	log.Index = uint(pbLog.GetIndex())
}

// Serialize returns a serialized byte stream for the Log
func (log *Log) Serialize() ([]byte, error) {
	return proto.Marshal(log.ConvertToLogPb())
}

// Deserialize parse the byte stream into Log
func (log *Log) Deserialize(buf []byte) error {
	pbLog := &iotextypes.Log{}
	if err := proto.Unmarshal(buf, pbLog); err != nil {
		return err
	}
	log.ConvertFromLogPb(pbLog)
	return nil
}

// IsImplicitTransfer checks whether a log is implicit transfer log
func (log *Log) IsImplicitTransfer() bool {
	return log.IsEvmTransfer() || log.IsCreateBucket() || log.IsDepositBucket() ||
		log.IsWithdrawBucket() || log.IsCandidateRegister() || log.IsCandidateSelfStake()
}

func (log *Log) isStakingImplicitLog(topic hash.Hash256) bool {
	if len(log.Topics) == 0 {
		return false
	}

	addr, _ := address.FromBytes(address.StakingProtocolAddrHash[:])
	if log.Address != addr.String() {
		return false
	}

	if log.Topics[0] != topic {
		return false
	}

	if len(log.Topics) < 4 {
		return false
	}

	switch {
	case topic == BucketCreateAmount || topic == BucketDepositAmount || topic == CandidateSelfStake:
		// amount goes into staking bucket pool
		return log.Topics[2] == StakingBucketPoolTopic
	case topic == BucketWithdrawAmount:
		// amount comes out of staking bucket pool
		return log.Topics[1] == StakingBucketPoolTopic
	case topic == CandidateRegistrationFee:
		// amount goes into rewarding pool
		return log.Topics[2] == RewardingPoolTopic
	default:
		return false
	}
}

// IsEvmTransfer checks evm transfer log
func (log *Log) IsEvmTransfer() bool {
	if log == nil || len(log.Topics) == 0 {
		return false
	}
	return log.Topics[0] == InContractTransfer
}

// IsWithdrawBucket checks withdraw bucket log
func (log *Log) IsWithdrawBucket() bool {
	return log.isStakingImplicitLog(BucketWithdrawAmount)
}

// IsCreateBucket checks create bucket log
func (log *Log) IsCreateBucket() bool {
	return log.isStakingImplicitLog(BucketCreateAmount)
}

// IsDepositBucket checks deposit bucket log
func (log *Log) IsDepositBucket() bool {
	return log.isStakingImplicitLog(BucketDepositAmount)
}

// IsCandidateRegister checks candidate register log
func (log *Log) IsCandidateRegister() bool {
	return log.isStakingImplicitLog(CandidateRegistrationFee)
}

// IsCandidateSelfStake checks candidate self-stake log
func (log *Log) IsCandidateSelfStake() bool {
	return log.isStakingImplicitLog(CandidateSelfStake)
}
