// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/test/identityset"
)

var (
	addr        = "io1qnpz47hx5q6r3w876axtrn6yz95d70cjl35r53"
	amount      = big.NewInt(100)
	sender      = identityset.Address(0)
	senderTopic = hash.BytesToHash256(identityset.PrivateKey(0).PublicKey().Hash())
	recver      = identityset.Address(1)
	recverTopic = hash.BytesToHash256(identityset.PrivateKey(1).PublicKey().Hash())

	evmTopics = []hash.Hash256{hash.BytesToHash256(InContractTransfer[:]), senderTopic, recverTopic}
	evmLog    = &Log{addr, evmTopics, amount.Bytes(), 1, hash.ZeroHash256, 1, false}

	withdrawTopics = []hash.Hash256{BucketWithdraAmount, hash.ZeroHash256, senderTopic}
	withdrawLog    = &Log{addr, withdrawTopics, amount.Bytes(), 1, hash.ZeroHash256, 1, false}

	normalLog = &Log{addr, []hash.Hash256{senderTopic, recverTopic}, amount.Bytes(), 1, hash.ZeroHash256, 0, false}
	panicLog  = &Log{addr, withdrawTopics, amount.Bytes(), 1, hash.ZeroHash256, 0, false}

	receiptTest = []struct {
		r   *Receipt
		num uint64
	}{
		{
			// failed receipt
			&Receipt{Status: uint64(iotextypes.ReceiptStatus_Failure)},
			0,
		},
		{
			// success but not system log
			&Receipt{
				Status: uint64(iotextypes.ReceiptStatus_Success), Logs: []*Log{normalLog}},
			0,
		},
		{
			// contain evm transfer
			&Receipt{Status: uint64(iotextypes.ReceiptStatus_Success), Logs: []*Log{evmLog}},
			1,
		},
		{
			// contain withdraw bucket
			&Receipt{Status: uint64(iotextypes.ReceiptStatus_Success), Logs: []*Log{withdrawLog}},
			1,
		},
		{
			// contain both
			&Receipt{Status: uint64(iotextypes.ReceiptStatus_Success), Logs: []*Log{evmLog, withdrawLog}},
			2,
		},
	}
)

func TestIsSystemLog(t *testing.T) {
	r := require.New(t)

	r.True(IsEvmTransfer(evmLog))
	r.False(IsEvmTransfer(withdrawLog))
	r.False(IsEvmTransfer(normalLog))
	r.False(IsEvmTransfer(panicLog))

	r.True(IsWithdrawBucket(withdrawLog))
	r.False(IsWithdrawBucket(evmLog))
	r.False(IsWithdrawBucket(normalLog))
	r.Panics(func() { IsWithdrawBucket(panicLog) })
}

func validateSystemLog(r *require.Assertions, log *Log, rec *TokenTxRecord) bool {
	if !IsSystemLog(log) {
		return false
	}
	txAmount := new(big.Int).SetBytes(log.Data)
	r.Equal(txAmount.String(), rec.amount)
	if IsEvmTransfer(log) {
		from, _ := address.FromBytes(log.Topics[1][12:])
		r.Equal(from.String(), rec.sender)
	}
	if IsWithdrawBucket(log) {
		r.Equal(log.Address, rec.sender)
	}
	to, _ := address.FromBytes(log.Topics[2][12:])
	r.Equal(to.String(), rec.recipient)
	return true
}

func TestReceiptSystemLog(t *testing.T) {
	r := require.New(t)

	for _, v := range receiptTest {
		sysLog := ReceiptSystemLog(v.r)
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

	blkLog := SystemLogFromReceipt(nil)
	r.Nil(blkLog)
	blkLog = SystemLogFromReceipt([]*Receipt{})
	r.Nil(blkLog)
	blkLog = SystemLogFromReceipt([]*Receipt{&Receipt{
		Status: uint64(iotextypes.ReceiptStatus_Success), Logs: []*Log{normalLog}},
	})
	r.Nil(blkLog)

	sysReceipts := []*Receipt{}
	for i := range receiptTest {
		if receiptTest[i].num > 0 {
			sysReceipts = append(sysReceipts, receiptTest[i].r)
		}
	}
	blkLog = SystemLogFromReceipt(sysReceipts)
	r.EqualValues(len(sysReceipts), blkLog.numActions)
	r.Equal(len(sysReceipts), len(blkLog.actionLogs))

	// test serialize/deserialize
	b := blkLog.Serialize()
	r.NotNil(b)
	pb, err := DeserializeBlockSystemLogPb(b)
	r.NoError(err)

	// verify block systemlog pb message
	r.EqualValues(len(sysReceipts), pb.NumTransactions)
	r.Equal(len(sysReceipts), len(pb.ActionSystemLog))
	for i, sysLog := range pb.ActionSystemLog {
		receipt := sysReceipts[i]
		r.Equal(receipt.ActionHash, hash.BytesToHash256(sysLog.ActionHash))
		r.EqualValues(len(receipt.Logs), sysLog.NumTransactions)
		r.Equal(len(receipt.Logs), len(sysLog.Transactions))
		for i, rec := range sysLog.Transactions {
			// verify token tx record
			rec := &TokenTxRecord{
				amount:    rec.Amount,
				sender:    rec.Sender,
				recipient: rec.Recipient,
			}
			r.True(validateSystemLog(r, receipt.Logs[i], rec))
		}
	}
}
