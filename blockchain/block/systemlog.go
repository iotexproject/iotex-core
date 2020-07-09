// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package block

import (
	"math/big"

	"github.com/golang/protobuf/proto"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

// implicit transfer log definitions
type (
	// TokenTxRecord is a token transaction record
	TokenTxRecord struct {
		topic     []byte
		amount    string
		sender    string
		recipient string
	}

	// ImplictTransferLog is implicit transfer log in one action
	ImplictTransferLog struct {
		actHash   hash.Hash256
		numTxs    uint64
		txRecords []*TokenTxRecord
	}

	// BlkImplictTransferLog is implicit transfer log in one block
	BlkImplictTransferLog struct {
		numActions uint64
		actionLogs []*ImplictTransferLog
	}
)

// NewImplictTransferLog creates a new ImplictTransferLog
func NewImplictTransferLog(actHash hash.Hash256, records []*TokenTxRecord) *ImplictTransferLog {
	if len(records) == 0 {
		return nil
	}

	return &ImplictTransferLog{
		actHash:   actHash,
		numTxs:    uint64(len(records)),
		txRecords: records,
	}
}

// NewTokenTxRecord creates a new TokenTxRecord
func NewTokenTxRecord(topic []byte, amount, sender, recipient string) *TokenTxRecord {
	rec := TokenTxRecord{
		topic:     make([]byte, len(topic)),
		amount:    amount,
		sender:    sender,
		recipient: recipient,
	}
	copy(rec.topic, topic)
	return &rec
}

// DeserializeSystemLogPb parse the byte stream into BlkImplictTransferLog Pb message
func DeserializeSystemLogPb(buf []byte) (*iotextypes.BlockImplicitTransferLog, error) {
	pb := &iotextypes.BlockImplicitTransferLog{}
	if err := proto.Unmarshal(buf, pb); err != nil {
		return nil, err
	}
	return pb, nil
}

// Serialize returns a serialized byte stream for BlkImplictTransferLog
func (log *BlkImplictTransferLog) Serialize() []byte {
	return byteutil.Must(proto.Marshal(log.toProto()))
}

func (log *BlkImplictTransferLog) toProto() *iotextypes.BlockImplicitTransferLog {
	if len(log.actionLogs) == 0 {
		return nil
	}

	sysLog := iotextypes.BlockImplicitTransferLog{
		ImplicitTransferLog: []*iotextypes.ImplicitTransferLog{},
	}
	for _, l := range log.actionLogs {
		if log := l.Proto(); log != nil {
			sysLog.ImplicitTransferLog = append(sysLog.ImplicitTransferLog, log)
			sysLog.NumTransactions++
		}
	}

	if len(sysLog.ImplicitTransferLog) == 0 {
		return nil
	}
	return &sysLog
}

// Proto returns the pb message
func (log *ImplictTransferLog) Proto() *iotextypes.ImplicitTransferLog {
	if len(log.txRecords) == 0 {
		return nil
	}

	actionLog := iotextypes.ImplicitTransferLog{
		ActionHash:   log.actHash[:],
		Transactions: []*iotextypes.ImplicitTransferLog_Transaction{},
	}
	for _, l := range log.txRecords {
		if record := l.toProto(); record != nil {
			actionLog.Transactions = append(actionLog.Transactions, record)
			actionLog.NumTransactions++
		}
	}

	if actionLog.NumTransactions == 0 {
		return nil
	}
	return &actionLog
}

func (log *TokenTxRecord) toProto() *iotextypes.ImplicitTransferLog_Transaction {
	return &iotextypes.ImplicitTransferLog_Transaction{
		Topic:     log.topic,
		Amount:    log.amount,
		Sender:    log.sender,
		Recipient: log.recipient,
	}
}

// ReceiptImplicitTransferLog generates implicit transfer log from receipt
func ReceiptImplicitTransferLog(r *action.Receipt) *ImplictTransferLog {
	if r == nil || len(r.Logs) == 0 || r.Status != uint64(iotextypes.ReceiptStatus_Success) {
		return nil
	}

	actionLog := ImplictTransferLog{
		actHash:   r.ActionHash,
		txRecords: []*TokenTxRecord{},
	}
	for _, log := range r.Logs {
		if record := LogTokenTxRecord(log); record != nil {
			actionLog.txRecords = append(actionLog.txRecords, record)
			actionLog.numTxs++
		}
	}

	if actionLog.numTxs == 0 {
		return nil
	}
	return &actionLog
}

// LogTokenTxRecord generates token transaction record from log
func LogTokenTxRecord(log *action.Log) *TokenTxRecord {
	txRecord := TokenTxRecord{}

	switch {
	case log.IsEvmTransfer():
		txRecord.topic = make([]byte, len(log.Topics[0]))
		copy(txRecord.topic, log.Topics[0][:])
		txRecord.amount = new(big.Int).SetBytes(log.Data).String()
		from, _ := address.FromBytes(log.Topics[1][12:])
		txRecord.sender = from.String()
		to, _ := address.FromBytes(log.Topics[2][12:])
		txRecord.recipient = to.String()
		return &txRecord
	case log.IsWithdrawBucket():
		txRecord.topic = make([]byte, len(log.Topics[0]))
		copy(txRecord.topic, log.Topics[0][:])
		txRecord.amount = new(big.Int).SetBytes(log.Data).String()
		txRecord.sender = log.Address
		to, _ := address.FromBytes(log.Topics[2][12:])
		txRecord.recipient = to.String()
		return &txRecord
	default:
		return nil
	}
}
