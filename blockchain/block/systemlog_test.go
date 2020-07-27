// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package block

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/test/identityset"
)

var (
	stkAddr     = "io1qnpz47hx5q6r3w876axtrn6yz95d70cjl35r53"
	amount      = big.NewInt(100)
	sender      = identityset.Address(0)
	recver      = identityset.Address(1)
	senderTopic = hash.BytesToHash256(identityset.PrivateKey(0).PublicKey().Hash())
	recverTopic = hash.BytesToHash256(identityset.PrivateKey(1).PublicKey().Hash())

	evmLog = &action.Log{
		Address: stkAddr,
		Data:    amount.Bytes(),
		TransactionData: &action.TransactionLog{
			Sender:    identityset.Address(0).String(),
			Recipient: identityset.Address(1).String(),
			Amount:    amount,
			Type:      iotextypes.TransactionLogType_IN_CONTRACT_TRANSFER,
		},
	}
	createLog = &action.Log{
		Address: stkAddr,
		Data:    amount.Bytes(),
		TransactionData: &action.TransactionLog{
			Sender:    identityset.Address(0).String(),
			Recipient: address.StakingBucketPoolAddr,
			Amount:    amount,
			Type:      iotextypes.TransactionLogType_CREATE_BUCKET,
		},
	}
	depositLog = &action.Log{
		Address: stkAddr,
		Data:    amount.Bytes(),
		TransactionData: &action.TransactionLog{
			Sender:    identityset.Address(0).String(),
			Recipient: address.StakingBucketPoolAddr,
			Amount:    amount,
			Type:      iotextypes.TransactionLogType_DEPOSIT_TO_BUCKET,
		},
	}
	withdrawLog = &action.Log{
		Address: stkAddr,
		Data:    amount.Bytes(),
		TransactionData: &action.TransactionLog{
			Sender:    address.StakingBucketPoolAddr,
			Recipient: identityset.Address(0).String(),
			Amount:    amount,
			Type:      iotextypes.TransactionLogType_WITHDRAW_BUCKET,
		},
	}
	selfstakeLog = &action.Log{
		Address: stkAddr,
		Data:    amount.Bytes(),
		TransactionData: &action.TransactionLog{
			Sender:    identityset.Address(0).String(),
			Recipient: address.StakingBucketPoolAddr,
			Amount:    amount,
			Type:      iotextypes.TransactionLogType_CANDIDATE_SELF_STAKE,
		},
	}
	registerLog = &action.Log{
		Address: stkAddr,
		Data:    amount.Bytes(),
		TransactionData: &action.TransactionLog{
			Sender:    identityset.Address(0).String(),
			Recipient: address.RewardingPoolAddr,
			Amount:    amount,
			Type:      iotextypes.TransactionLogType_CANDIDATE_REGISTRATION_FEE,
		},
	}
	normalLog = &action.Log{
		Address: stkAddr,
		Topics:  []hash.Hash256{senderTopic, recverTopic},
		Data:    amount.Bytes(),
	}
	allLogs = []*action.Log{evmLog, createLog, depositLog, withdrawLog, selfstakeLog, registerLog}

	receiptTest = []struct {
		r   *action.Receipt
		num uint64
	}{
		{
			// failed receipt
			&action.Receipt{Status: uint64(iotextypes.ReceiptStatus_Failure)},
			0,
		},
		{
			// success but not transaction log
			&action.Receipt{
				Status: uint64(iotextypes.ReceiptStatus_Success), Logs: []*action.Log{normalLog}},
			0,
		},
		{
			// contain evm transfer
			&action.Receipt{Status: uint64(iotextypes.ReceiptStatus_Success), Logs: []*action.Log{evmLog}},
			1,
		},
		{
			// contain create bucket
			&action.Receipt{Status: uint64(iotextypes.ReceiptStatus_Success), Logs: []*action.Log{createLog}},
			1,
		},
		{
			// contain deposit bucket
			&action.Receipt{Status: uint64(iotextypes.ReceiptStatus_Success), Logs: []*action.Log{depositLog}},
			1,
		},
		{
			// contain withdraw bucket
			&action.Receipt{Status: uint64(iotextypes.ReceiptStatus_Success), Logs: []*action.Log{withdrawLog}},
			1,
		},
		{
			// contain candidate self-stake
			&action.Receipt{Status: uint64(iotextypes.ReceiptStatus_Success), Logs: []*action.Log{selfstakeLog}},
			1,
		},
		{
			// contain candidate register
			&action.Receipt{Status: uint64(iotextypes.ReceiptStatus_Success), Logs: []*action.Log{registerLog}},
			1,
		},
		{
			// contain all
			&action.Receipt{Status: uint64(iotextypes.ReceiptStatus_Success), Logs: allLogs},
			6,
		},
	}
)

func validateSystemLog(r *require.Assertions, log *action.Log, rec *TokenTxRecord) bool {
	if !log.IsTransactionLog() {
		return false
	}
	r.Equal(log.TransactionData.Type, rec.typ)
	r.Equal(log.TransactionData.Amount.String(), rec.amount)
	r.Equal(log.TransactionData.Sender, rec.sender)
	r.Equal(log.TransactionData.Recipient, rec.recipient)
	return true
}

func TestReceiptSystemLog(t *testing.T) {
	r := require.New(t)

	for _, v := range receiptTest {
		sysLog := ReceiptTransactionLog(v.r)
		if v.num > 0 {
			r.Equal(v.r.ActionHash, sysLog.actHash)
			r.EqualValues(v.num, sysLog.numTxs)
			for i, rec := range sysLog.txRecords {
				r.True(validateSystemLog(r, v.r.Logs[i], rec))
			}
		} else {
			r.Nil(sysLog)
		}
	}
}

func TestSystemLogFromReceipt(t *testing.T) {
	r := require.New(t)

	blk := Block{}
	blkLog := blk.TransactionLog()
	r.Nil(blkLog)
	blk.Receipts = []*action.Receipt{}
	blkLog = blk.TransactionLog()
	r.Nil(blkLog)
	// normal log is not transaction log
	blk.Receipts = append(blk.Receipts, &action.Receipt{
		Status: uint64(iotextypes.ReceiptStatus_Success), Logs: []*action.Log{normalLog},
	})
	blkLog = blk.TransactionLog()
	r.Nil(blkLog)

	blk.Receipts = blk.Receipts[:0]
	implicitTransferNum := 0
	for i := range receiptTest {
		if receiptTest[i].num > 0 {
			blk.Receipts = append(blk.Receipts, receiptTest[i].r)
			implicitTransferNum++
		}
	}
	blkLog = blk.TransactionLog()
	r.EqualValues(implicitTransferNum, blkLog.numActions)
	r.Equal(implicitTransferNum, len(blkLog.actionLogs))

	// test serialize/deserialize
	b := blkLog.Serialize()
	r.NotNil(b)
	pb, err := DeserializeSystemLogPb(b)
	r.NoError(err)

	// verify block systemlog pb message
	r.EqualValues(implicitTransferNum, pb.NumTransactions)
	r.Equal(implicitTransferNum, len(pb.TransactionLog))
	for i, sysLog := range pb.TransactionLog {
		receipt := blk.Receipts[i]
		r.Equal(receipt.ActionHash, hash.BytesToHash256(sysLog.ActionHash))
		r.EqualValues(len(receipt.Logs), sysLog.NumTransactions)
		r.Equal(len(receipt.Logs), len(sysLog.Transactions))
		for i, tx := range sysLog.Transactions {
			// verify token tx record
			rec := &TokenTxRecord{
				amount:    tx.Amount,
				sender:    tx.Sender,
				recipient: tx.Recipient,
				typ:       tx.Type,
			}
			r.True(validateSystemLog(r, receipt.Logs[i], rec))
		}
	}
}
