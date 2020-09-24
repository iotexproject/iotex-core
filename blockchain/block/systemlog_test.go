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
	amount = big.NewInt(100)

	evmLog = &action.TransactionLog{
		Sender:    identityset.Address(0).String(),
		Recipient: identityset.Address(1).String(),
		Amount:    amount,
		Type:      iotextypes.TransactionLogType_IN_CONTRACT_TRANSFER,
	}
	createLog = &action.TransactionLog{
		Sender:    identityset.Address(0).String(),
		Recipient: address.StakingBucketPoolAddr,
		Amount:    amount,
		Type:      iotextypes.TransactionLogType_CREATE_BUCKET,
	}
	depositLog = &action.TransactionLog{
		Sender:    identityset.Address(0).String(),
		Recipient: address.StakingBucketPoolAddr,
		Amount:    amount,
		Type:      iotextypes.TransactionLogType_DEPOSIT_TO_BUCKET,
	}
	withdrawLog = &action.TransactionLog{
		Sender:    address.StakingBucketPoolAddr,
		Recipient: identityset.Address(0).String(),
		Amount:    amount,
		Type:      iotextypes.TransactionLogType_WITHDRAW_BUCKET,
	}
	selfstakeLog = &action.TransactionLog{
		Sender:    identityset.Address(0).String(),
		Recipient: address.StakingBucketPoolAddr,
		Amount:    amount,
		Type:      iotextypes.TransactionLogType_CANDIDATE_SELF_STAKE,
	}
	registerLog = &action.TransactionLog{
		Sender:    identityset.Address(0).String(),
		Recipient: address.RewardingPoolAddr,
		Amount:    amount,
		Type:      iotextypes.TransactionLogType_CANDIDATE_REGISTRATION_FEE,
	}
	normalLog = &action.Log{
		Address: "io1qnpz47hx5q6r3w876axtrn6yz95d70cjl35r53",
		Topics: []hash.Hash256{
			hash.BytesToHash256(identityset.PrivateKey(0).PublicKey().Hash()),
			hash.BytesToHash256(identityset.PrivateKey(1).PublicKey().Hash()),
		},
		Data: amount.Bytes(),
	}
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
			(&action.Receipt{Status: uint64(iotextypes.ReceiptStatus_Success)}).AddLogs(normalLog),
			0,
		},
		{
			// contain evm transfer
			(&action.Receipt{Status: uint64(iotextypes.ReceiptStatus_Success)}).AddTransactionLogs(evmLog),
			1,
		},
		{
			// contain create bucket
			(&action.Receipt{Status: uint64(iotextypes.ReceiptStatus_Success)}).AddTransactionLogs(createLog),
			1,
		},
		{
			// contain deposit bucket
			(&action.Receipt{Status: uint64(iotextypes.ReceiptStatus_Success)}).AddTransactionLogs(depositLog),
			1,
		},
		{
			// contain withdraw bucket
			(&action.Receipt{Status: uint64(iotextypes.ReceiptStatus_Success)}).AddTransactionLogs(withdrawLog),
			1,
		},
		{
			// contain candidate self-stake
			(&action.Receipt{Status: uint64(iotextypes.ReceiptStatus_Success)}).AddTransactionLogs(selfstakeLog),
			1,
		},
		{
			// contain candidate register
			(&action.Receipt{Status: uint64(iotextypes.ReceiptStatus_Success)}).AddTransactionLogs(registerLog),
			1,
		},
		{
			// contain all
			(&action.Receipt{Status: uint64(iotextypes.ReceiptStatus_Success)}).AddTransactionLogs(evmLog, createLog, depositLog, withdrawLog, selfstakeLog, registerLog),
			6,
		},
	}
)

func TestReceiptSystemLog(t *testing.T) {
	r := require.New(t)

	for _, v := range receiptTest {
		sysLog := ReceiptTransactionLog(v.r)
		if v.num > 0 {
			r.Equal(v.r.ActionHash, sysLog.actHash)
			r.EqualValues(v.num, len(sysLog.txRecords))
			logs := v.r.TransactionLogs()
			for i, rec := range sysLog.txRecords {
				r.Equal(LogTokenTxRecord(logs[i]), rec)
			}
		} else {
			r.Nil(sysLog)
		}
	}

	// can serialize an empty log
	log := &BlkTransactionLog{}
	r.Equal([]byte{}, log.Serialize())
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
	blk.Receipts = append(blk.Receipts, (&action.Receipt{Status: uint64(iotextypes.ReceiptStatus_Success)}).AddLogs(normalLog))
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
	r.Equal(implicitTransferNum, len(blkLog.actionLogs))

	// test serialize/deserialize
	b := blkLog.Serialize()
	r.NotNil(b)
	pb, err := DeserializeSystemLogPb(b)
	r.NoError(err)

	// verify block systemlog pb message
	r.EqualValues(implicitTransferNum, len(pb.Logs))
	for i, sysLog := range pb.Logs {
		receipt := blk.Receipts[i]
		logs := receipt.TransactionLogs()
		r.Equal(receipt.ActionHash, hash.BytesToHash256(sysLog.ActionHash))
		r.EqualValues(len(logs), sysLog.NumTransactions)
		r.Equal(len(logs), len(sysLog.Transactions))
		for i, tx := range sysLog.Transactions {
			// verify token tx record
			rec := &TokenTxRecord{
				amount:    tx.Amount,
				sender:    tx.Sender,
				recipient: tx.Recipient,
				typ:       tx.Type,
			}
			r.Equal(LogTokenTxRecord(logs[i]), rec)
		}
	}
}
