// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"github.com/golang/protobuf/proto"

	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/proto"
)

// Receipt represents the result of a contract
type Receipt struct {
	ReturnValue     []byte
	Status          uint64
	ActHash         hash.Hash256
	GasConsumed     uint64
	ContractAddress string
	Logs            []*Log
}

// Log stores an evm contract event
type Log struct {
	Address     string
	Topics      []hash.Hash256
	Data        []byte
	BlockNumber uint64
	TxnHash     hash.Hash256
	// TODO: in our case, BlockHash is actually txRoot, we need to revisit this field later
	BlockHash hash.Hash256
	Index     uint
}

// ConvertToReceiptPb converts a Receipt to protobuf's ReceiptPb
func (receipt *Receipt) ConvertToReceiptPb() *iproto.ReceiptPb {
	r := &iproto.ReceiptPb{}
	r.ReturnValue = receipt.ReturnValue
	r.Status = receipt.Status
	r.ActHash = receipt.ActHash[:]
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
	copy(receipt.ActHash[:], pbReceipt.GetActHash())
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

// Hash returns the hash of receipt
func (receipt *Receipt) Hash() hash.Hash256 {
	data, err := receipt.Serialize()
	if err != nil {
		log.L().Panic("Error when serializing a receipt")
	}
	return byteutil.BytesTo32B(hash.Hash256b(data))
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
	l.BlockHash = log.BlockHash[:]
	l.Index = uint32(log.Index)
	return l
}

// ConvertFromLogPb converts a protobuf's LogPb to Log
func (log *Log) ConvertFromLogPb(pbLog *iproto.LogPb) {
	log.Address = pbLog.GetAddress()
	pbLogs := pbLog.GetTopics()
	log.Topics = make([]hash.Hash256, len(pbLogs))
	for i, topic := range pbLogs {
		copy(log.Topics[i][:], topic)
	}
	log.Data = pbLog.GetData()
	log.BlockNumber = pbLog.GetBlockNumber()
	copy(log.TxnHash[:], pbLog.GetTxnHash())
	copy(log.BlockHash[:], pbLog.GetBlockHash())
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
