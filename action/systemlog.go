// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"

	"github.com/golang/protobuf/proto"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

// system log definitions
type (
	// TokenTxRecord is a token transaction record
	TokenTxRecord struct {
		amount    string
		sender    string
		recipient string
	}

	// SystemLog is system log in one action
	SystemLog struct {
		actHash   hash.Hash256
		numTxs    uint64
		txRecords []*TokenTxRecord
	}

	// BlockSystemLog is system log in one block
	BlockSystemLog struct {
		numActions uint64
		actionLogs []*SystemLog
	}
)

// Deserialize parse the byte stream into BlockSystemLog
func (log *BlockSystemLog) Deserialize(buf []byte) error {
	_, err := DeserializeBlockSystemLogPb(buf)
	if err != nil {
		return err
	}

	log.numActions = 0
	log.actionLogs = log.actionLogs[:0]
	return nil
}

// DeserializeBlockSystemLogPb parse the byte stream into BlockSystemLog Pb message
func DeserializeBlockSystemLogPb(buf []byte) (*iotextypes.BlockSystemLog, error) {
	pb := &iotextypes.BlockSystemLog{}
	if err := proto.Unmarshal(buf, pb); err != nil {
		return nil, err
	}
	return pb, nil
}

// Serialize returns a serialized byte stream for BlockSystemLog
func (log *BlockSystemLog) Serialize() []byte {
	return byteutil.Must(proto.Marshal(log.toProto()))
}

func (log *BlockSystemLog) toProto() *iotextypes.BlockSystemLog {
	if len(log.actionLogs) == 0 {
		return nil
	}

	sysLog := iotextypes.BlockSystemLog{
		ActionSystemLog: []*iotextypes.ActionSystemLog{},
	}
	for _, l := range log.actionLogs {
		if log := l.toProto(); log != nil {
			sysLog.ActionSystemLog = append(sysLog.ActionSystemLog, log)
			sysLog.NumTransactions++
		}
	}

	if len(sysLog.ActionSystemLog) == 0 {
		return nil
	}
	return &sysLog
}

func (log *SystemLog) toProto() *iotextypes.ActionSystemLog {
	if len(log.txRecords) == 0 {
		return nil
	}

	actionLog := iotextypes.ActionSystemLog{
		Transactions: []*iotextypes.ActionSystemLog_Transaction{},
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

func (log *TokenTxRecord) toProto() *iotextypes.ActionSystemLog_Transaction {
	return &iotextypes.ActionSystemLog_Transaction{
		Amount:    log.amount,
		Sender:    log.sender,
		Recipient: log.recipient,
	}
}

// SystemLogFromReceipt returns system logs in the receipt
func SystemLogFromReceipt(receipts []*Receipt) *BlockSystemLog {
	if len(receipts) == 0 {
		return nil
	}

	blkLog := BlockSystemLog{
		actionLogs: []*SystemLog{},
	}
	for _, r := range receipts {
		if log := ReceiptSystemLog(r); log != nil {
			blkLog.actionLogs = append(blkLog.actionLogs, log)
			blkLog.numActions++
		}
	}

	if blkLog.numActions == 0 {
		return nil
	}
	return &blkLog
}

// ReceiptSystemLog generates system log from receipt
func ReceiptSystemLog(r *Receipt) *SystemLog {
	if r == nil || len(r.Logs) == 0 || r.Status != uint64(iotextypes.ReceiptStatus_Success) {
		return nil
	}

	actionLog := SystemLog{
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
func LogTokenTxRecord(log *Log) *TokenTxRecord {
	txRecord := TokenTxRecord{}

	switch {
	case IsEvmTransfer(log):
		txRecord.amount = new(big.Int).SetBytes(log.Data).String()
		from, _ := address.FromBytes(log.Topics[1][12:])
		txRecord.sender = from.String()
		to, _ := address.FromBytes(log.Topics[2][12:])
		txRecord.recipient = to.String()
		return &txRecord
	case IsWithdrawBucket(log):
		txRecord.amount = new(big.Int).SetBytes(log.Data).String()
		txRecord.sender = log.Address
		to, _ := address.FromBytes(log.Topics[2][12:])
		txRecord.recipient = to.String()
		return &txRecord
	default:
		return nil
	}
}
