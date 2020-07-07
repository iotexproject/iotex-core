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
	addr        = "io1qnpz47hx5q6r3w876axtrn6yz95d70cjl35r53"
	amount      = big.NewInt(100)
	sender      = identityset.Address(0)
	senderTopic = hash.BytesToHash256(identityset.PrivateKey(0).PublicKey().Hash())
	recver      = identityset.Address(1)
	recverTopic = hash.BytesToHash256(identityset.PrivateKey(1).PublicKey().Hash())

	evmTopics = []hash.Hash256{hash.BytesToHash256(action.InContractTransfer[:]), senderTopic, recverTopic}
	evmLog    = &action.Log{addr, evmTopics, amount.Bytes(), 1, hash.ZeroHash256, 1, false}

	withdrawTopics = []hash.Hash256{action.BucketWithdrawAmount, hash.ZeroHash256, senderTopic}
	withdrawLog    = &action.Log{addr, withdrawTopics, amount.Bytes(), 1, hash.ZeroHash256, 1, false}

	normalLog = &action.Log{addr, []hash.Hash256{senderTopic, recverTopic}, amount.Bytes(), 1, hash.ZeroHash256, 0, false}
	panicLog  = &action.Log{addr, withdrawTopics, amount.Bytes(), 1, hash.ZeroHash256, 0, false}

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
			// success but not system log
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
			// contain withdraw bucket
			&action.Receipt{Status: uint64(iotextypes.ReceiptStatus_Success), Logs: []*action.Log{withdrawLog}},
			1,
		},
		{
			// contain both
			&action.Receipt{Status: uint64(iotextypes.ReceiptStatus_Success), Logs: []*action.Log{evmLog, withdrawLog}},
			2,
		},
	}
)

func TestIsSystemLog(t *testing.T) {
	r := require.New(t)

	r.True(evmLog.IsEvmTransfer())
	r.False(withdrawLog.IsEvmTransfer())
	r.False(normalLog.IsEvmTransfer())
	r.False(panicLog.IsEvmTransfer())

	r.True(withdrawLog.IsWithdrawBucket())
	r.False(evmLog.IsWithdrawBucket())
	r.False(normalLog.IsWithdrawBucket())
	r.Panics(func() { panicLog.IsWithdrawBucket() })
}

func validateSystemLog(r *require.Assertions, log *action.Log, rec *TokenTxRecord) bool {
	if !log.IsSystemLog() {
		return false
	}
	txAmount := new(big.Int).SetBytes(log.Data)
	r.Equal(txAmount.String(), rec.amount)
	if log.IsEvmTransfer() {
		from, _ := address.FromBytes(log.Topics[1][12:])
		r.Equal(from.String(), rec.sender)
	}
	if log.IsWithdrawBucket() {
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
	blkLog = SystemLogFromReceipt([]*action.Receipt{})
	r.Nil(blkLog)
	blkLog = SystemLogFromReceipt([]*action.Receipt{&action.Receipt{
		Status: uint64(iotextypes.ReceiptStatus_Success), Logs: []*action.Log{normalLog}},
	})
	r.Nil(blkLog)

	sysReceipts := []*action.Receipt{}
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
	pb, err := DeserializeSystemLogPb(b)
	r.NoError(err)

	// verify block systemlog pb message
	r.EqualValues(len(sysReceipts), pb.NumTransactions)
	r.Equal(len(sysReceipts), len(pb.ImplicitTransferLog))
	for i, sysLog := range pb.ImplicitTransferLog {
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
