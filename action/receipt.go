// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"

	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/pkg/log"
)

var (
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
		Status             uint64
		BlockHeight        uint64
		ActionHash         hash.Hash256
		GasConsumed        uint64
		ContractAddress    string
		TxIndex            uint32
		logs               []*Log
		transactionLogs    []*TransactionLog
		executionRevertMsg string
	}

	// Log stores an evm contract event
	Log struct {
		Address            string
		Topics             Topics
		Data               []byte
		BlockHeight        uint64
		ActionHash         hash.Hash256
		Index, TxIndex     uint32
		NotFixTopicCopyBug bool
	}

	// TransactionLog stores a transaction event
	TransactionLog struct {
		Type      iotextypes.TransactionLogType
		Amount    *big.Int
		Sender    string
		Recipient string
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
	r.TxIndex = receipt.TxIndex
	r.Logs = []*iotextypes.Log{}
	for _, l := range receipt.logs {
		r.Logs = append(r.Logs, l.ConvertToLogPb())
	}
	if receipt.executionRevertMsg != "" {
		r.ExecutionRevertMsg = receipt.executionRevertMsg
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
	receipt.TxIndex = pbReceipt.GetTxIndex()
	logs := pbReceipt.GetLogs()
	receipt.logs = make([]*Log, len(logs))
	for i, log := range logs {
		receipt.logs[i] = &Log{}
		receipt.logs[i].ConvertFromLogPb(log)
	}
	receipt.executionRevertMsg = pbReceipt.GetExecutionRevertMsg()
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

// Logs returns the list of logs stored in receipt
func (receipt *Receipt) Logs() []*Log {
	return receipt.logs
}

// AddLogs add log to receipt and filter out nil log.
func (receipt *Receipt) AddLogs(logs ...*Log) *Receipt {
	for _, l := range logs {
		if l != nil {
			receipt.logs = append(receipt.logs, l)
		}
	}
	return receipt
}

// TransactionLogs returns the list of transaction logs stored in receipt
func (receipt *Receipt) TransactionLogs() []*TransactionLog {
	return receipt.transactionLogs
}

// AddTransactionLogs add transaction logs to receipt and filter out nil log.
func (receipt *Receipt) AddTransactionLogs(logs ...*TransactionLog) *Receipt {
	for _, l := range logs {
		if l != nil {
			receipt.transactionLogs = append(receipt.transactionLogs, l)
		}
	}
	return receipt
}

// ExecutionRevertMsg returns the list of execution revert error logs stored in receipt.
func (receipt *Receipt) ExecutionRevertMsg() string {
	return receipt.executionRevertMsg
}

// SetExecutionRevertMsg sets executionerrorlogs to receipt.
func (receipt *Receipt) SetExecutionRevertMsg(revertReason string) *Receipt {
	if receipt.executionRevertMsg == "" && revertReason != "" {
		receipt.executionRevertMsg = revertReason
	}
	return receipt
}

// UpdateIndex updates the index of receipt and logs
func (receipt *Receipt) UpdateIndex(txIndex, logIndex uint32) (uint32, uint32) {
	if receipt == nil {
		return txIndex, logIndex
	}
	receipt.TxIndex = txIndex
	for _, l := range receipt.logs {
		l.TxIndex = txIndex
		l.Index = logIndex
		logIndex++
	}
	return txIndex + 1, logIndex
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
	l.Index = log.Index
	l.TxIndex = log.TxIndex
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
	log.Index = pbLog.GetIndex()
	log.TxIndex = pbLog.GetTxIndex()
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
