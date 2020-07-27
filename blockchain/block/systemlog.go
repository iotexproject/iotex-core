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

// transaction log definitions
type (
	// TokenTxRecord is a token transaction record
	TokenTxRecord struct {
		topic     []byte
		amount    string
		sender    string
		recipient string
	}

	// TransactionLog is transaction log in one action
	TransactionLog struct {
		actHash   hash.Hash256
		numTxs    uint64
		txRecords []*TokenTxRecord
	}

	// BlkTransactionLog is transaction log in one block
	BlkTransactionLog struct {
		numActions uint64
		actionLogs []*TransactionLog
	}
)

// NewImplictTransferLog creates a new TransactionLog
func NewImplictTransferLog(actHash hash.Hash256, records []*TokenTxRecord) *TransactionLog {
	if len(records) == 0 {
		return nil
	}

	return &TransactionLog{
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

// DeserializeSystemLogPb parse the byte stream into BlkTransactionLog Pb message
func DeserializeSystemLogPb(buf []byte) (*iotextypes.BlockTransactionLog, error) {
	pb := &iotextypes.BlockTransactionLog{}
	if err := proto.Unmarshal(buf, pb); err != nil {
		return nil, err
	}
	return pb, nil
}

// Serialize returns a serialized byte stream for BlkTransactionLog
func (log *BlkTransactionLog) Serialize() []byte {
	return byteutil.Must(proto.Marshal(log.toProto()))
}

func (log *BlkTransactionLog) toProto() *iotextypes.BlockTransactionLog {
	if len(log.actionLogs) == 0 {
		return nil
	}

	sysLog := iotextypes.BlockTransactionLog{
		TransactionLog: []*iotextypes.TransactionLog{},
	}
	for _, l := range log.actionLogs {
		if log := l.Proto(); log != nil {
			sysLog.TransactionLog = append(sysLog.TransactionLog, log)
			sysLog.NumTransactions++
		}
	}

	if len(sysLog.TransactionLog) == 0 {
		return nil
	}
	return &sysLog
}

// Proto returns the pb message
func (log *TransactionLog) Proto() *iotextypes.TransactionLog {
	if len(log.txRecords) == 0 {
		return nil
	}

	actionLog := iotextypes.TransactionLog{
		ActionHash:   log.actHash[:],
		Transactions: []*iotextypes.TransactionLog_Transaction{},
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

func (log *TokenTxRecord) toProto() *iotextypes.TransactionLog_Transaction {
	return &iotextypes.TransactionLog_Transaction{
		Topic:     log.topic,
		Amount:    log.amount,
		Sender:    log.sender,
		Recipient: log.recipient,
	}
}

// ReceiptTransactionLog generates transaction log from receipt
func ReceiptTransactionLog(r *action.Receipt) *TransactionLog {
	if r == nil || len(r.Logs) == 0 || r.Status != uint64(iotextypes.ReceiptStatus_Success) {
		return nil
	}

	actionLog := TransactionLog{
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
	if log == nil || !log.IsTransactionLog() {
		return nil
	}

	txRecord := TokenTxRecord{}
	txRecord.topic = make([]byte, len(log.Topics[0]))
	copy(txRecord.topic, log.Topics[0][:])
	txRecord.amount = new(big.Int).SetBytes(log.Data).String()
	from, _ := address.FromBytes(log.Topics[1][12:])
	txRecord.sender = from.String()
	to, _ := address.FromBytes(log.Topics[2][12:])
	txRecord.recipient = to.String()

	if log.Sender != "" {
		txRecord.sender = log.Sender
	}
	if log.Recipient != "" {
		txRecord.recipient = log.Recipient
	}
	return &txRecord
}
