// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package block

import (
	"github.com/golang/protobuf/proto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

// transaction log definitions
type (
	// TokenTxRecord is a token transaction record
	TokenTxRecord struct {
		amount    string
		sender    string
		recipient string
		typ       iotextypes.TransactionLogType
	}

	// TransactionLog is transaction log in one action
	TransactionLog struct {
		actHash   hash.Hash256
		txRecords []*TokenTxRecord
	}

	// BlkTransactionLog is transaction log in one block
	BlkTransactionLog struct {
		actionLogs []*TransactionLog
	}
)

// NewTransactionLog creates a new TransactionLog
func NewTransactionLog(actHash hash.Hash256, records []*TokenTxRecord) *TransactionLog {
	if len(records) == 0 {
		return nil
	}

	return &TransactionLog{
		actHash:   actHash,
		txRecords: records,
	}
}

// NewTokenTxRecord creates a new TokenTxRecord
func NewTokenTxRecord(typ iotextypes.TransactionLogType, amount, sender, recipient string) *TokenTxRecord {
	rec := TokenTxRecord{
		amount:    amount,
		sender:    sender,
		recipient: recipient,
		typ:       typ,
	}
	return &rec
}

// DeserializeSystemLogPb parse the byte stream into TransactionLogs Pb message
func DeserializeSystemLogPb(buf []byte) (*iotextypes.TransactionLogs, error) {
	pb := &iotextypes.TransactionLogs{}
	if err := proto.Unmarshal(buf, pb); err != nil {
		return nil, err
	}
	return pb, nil
}

// Serialize returns a serialized byte stream for BlkTransactionLog
func (log *BlkTransactionLog) Serialize() []byte {
	return byteutil.Must(proto.Marshal(log.toProto()))
}

func (log *BlkTransactionLog) toProto() *iotextypes.TransactionLogs {
	if len(log.actionLogs) == 0 {
		return &iotextypes.TransactionLogs{}
	}

	sysLog := iotextypes.TransactionLogs{}
	for _, l := range log.actionLogs {
		if log := l.Proto(); log != nil {
			sysLog.Logs = append(sysLog.Logs, log)
		}
	}

	if len(sysLog.Logs) == 0 {
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
		Amount:    log.amount,
		Sender:    log.sender,
		Recipient: log.recipient,
		Type:      log.typ,
	}
}

// ReceiptTransactionLog generates transaction log from receipt
func ReceiptTransactionLog(r *action.Receipt) *TransactionLog {
	if r == nil || len(r.TransactionLogs()) == 0 {
		return nil
	}

	actionLog := TransactionLog{
		actHash:   r.ActionHash,
		txRecords: []*TokenTxRecord{},
	}
	for _, log := range r.TransactionLogs() {
		if record := LogTokenTxRecord(log); record != nil {
			actionLog.txRecords = append(actionLog.txRecords, record)
		}
	}

	if len(actionLog.txRecords) == 0 {
		return nil
	}
	return &actionLog
}

// LogTokenTxRecord generates token transaction record from log
func LogTokenTxRecord(l *action.TransactionLog) *TokenTxRecord {
	if l == nil {
		return nil
	}
	if l.Amount.Sign() < 0 {
		log.L().Panic("Negative amount transaction log.", zap.Any("TransactionLog", l))
	}

	return &TokenTxRecord{
		sender:    l.Sender,
		recipient: l.Recipient,
		amount:    l.Amount.String(),
		typ:       l.Type,
	}
}
