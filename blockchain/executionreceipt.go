// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"github.com/golang/protobuf/proto"

	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/proto"
)

// Receipt represents the result of a contract
type Receipt struct {
	ReturnValue     []byte
	Status          uint64
	Hash            hash.Hash32B
	GasConsumed     uint64
	ContractAddress string
	Logs            []*Log
}

// Log stores an evm contract event
type Log struct {
	Address     string
	Topics      []hash.Hash32B
	Data        []byte
	BlockNumber uint64
	TxnHash     hash.Hash32B
	BLockHash   hash.Hash32B
	Index       uint
}

// ConvertToReceiptPb converts a Receipt to protobuf's ReceiptPb
func (receipt *Receipt) ConvertToReceiptPb() *iproto.ReceiptPb {
	r := &iproto.ReceiptPb{}
	r.ReturnValue = receipt.ReturnValue
	r.Status = receipt.Status
	r.Hash = receipt.Hash[:]
	r.GasConsumed = receipt.GasConsumed
	r.ContractAddress = receipt.ContractAddress
	r.Logs = []*iproto.LogPb{}
	for _, log := range receipt.Logs {
		r.Logs = append(r.Logs, log.ConvertToLogPb())
	}
	return r
}

// ConvertFromReceiptPb converts a protobuf's ReceiptPb to Receipt
func (receipt *Receipt) ConvertFromReceiptPb(pbReceipt *iproto.ReceiptPb) {
	receipt.ReturnValue = pbReceipt.GetReturnValue()
	receipt.Status = pbReceipt.GetStatus()
	copy(receipt.Hash[:], pbReceipt.GetHash())
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
	pbReceipt := &iproto.ReceiptPb{}
	if err := proto.Unmarshal(buf, pbReceipt); err != nil {
		return err
	}
	receipt.ConvertFromReceiptPb(pbReceipt)
	return nil
}

// ConvertToLogPb converts a Log to protobuf's LogPb
func (log *Log) ConvertToLogPb() *iproto.LogPb {
	l := &iproto.LogPb{}
	l.Address = log.Address
	l.Topics = [][]byte{}
	for _, topic := range log.Topics {
		l.Topics = append(l.Topics, topic[:])
	}
	l.Data = log.Data
	l.BlockNumber = log.BlockNumber
	l.TxnHash = log.TxnHash[:]
	l.BlockHash = log.BLockHash[:]
	l.Index = uint32(log.Index)
	return l
}

// ConvertFromLogPb converts a protobuf's LogPb to Log
func (log *Log) ConvertFromLogPb(pbLog *iproto.LogPb) {
	log.Address = pbLog.GetAddress()
	pbLogs := pbLog.GetTopics()
	log.Topics = make([]hash.Hash32B, len(pbLogs))
	for i, topic := range pbLogs {
		copy(log.Topics[i][:], topic)
	}
	log.Data = pbLog.GetData()
	log.BlockNumber = pbLog.GetBlockNumber()
	copy(log.TxnHash[:], pbLog.GetTxnHash())
	copy(log.BLockHash[:], pbLog.GetBlockHash())
	log.Index = uint(pbLog.GetIndex())
}

// Serialize returns a serialized byte stream for the Log
func (log *Log) Serialize() ([]byte, error) {
	return proto.Marshal(log.ConvertToLogPb())
}

// Deserialize parse the byte stream into Log
func (log *Log) Deserialize(buf []byte) error {
	pbLog := &iproto.LogPb{}
	if err := proto.Unmarshal(buf, pbLog); err != nil {
		return err
	}
	log.ConvertFromLogPb(pbLog)
	return nil
}
