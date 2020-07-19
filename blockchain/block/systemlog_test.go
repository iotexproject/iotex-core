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
	recver      = identityset.Address(1)
	senderTopic = hash.BytesToHash256(identityset.PrivateKey(0).PublicKey().Hash())
	recverTopic = hash.BytesToHash256(identityset.PrivateKey(1).PublicKey().Hash())
	reward      = hash.Hash160b([]byte(action.RewardingProtocolID))
	rewardTopic = hash.BytesToHash256(reward[:])

	staking        = hash.Hash160b([]byte(action.StakingProtocolID))
	stakingAddr, _ = address.FromBytes(staking[:])
	stakingTopic   = hash.BytesToHash256(staking[:])
	evmTopics      = []hash.Hash256{hash.ZeroHash256, senderTopic, recverTopic}
	evmLog         = &action.Log{addr, evmTopics, amount.Bytes(), 1, hash.ZeroHash256, 0, false}
	createTopics   = []hash.Hash256{action.BucketCreateAmount, senderTopic, stakingTopic, hash.ZeroHash256}
	createLog      = &action.Log{addr, createTopics, amount.Bytes(), 1, hash.ZeroHash256, 0, false}
	depositTopics  = []hash.Hash256{action.BucketDepositAmount, senderTopic, stakingTopic, hash.ZeroHash256}
	depositLog     = &action.Log{addr, depositTopics, amount.Bytes(), 1, hash.ZeroHash256, 0, false}
	withdrawTopics = []hash.Hash256{action.BucketWithdrawAmount, stakingTopic, senderTopic, hash.ZeroHash256}
	withdrawLog    = &action.Log{addr, withdrawTopics, amount.Bytes(), 1, hash.ZeroHash256, 0, false}
	sstakeTopics   = []hash.Hash256{action.CandidateSelfStake, senderTopic, stakingTopic, hash.ZeroHash256}
	selfstakeLog   = &action.Log{addr, sstakeTopics, amount.Bytes(), 1, hash.ZeroHash256, 0, false}
	registerTopics = []hash.Hash256{action.CandidateRegistrationFee, senderTopic, rewardTopic, hash.ZeroHash256}
	registerLog    = &action.Log{addr, registerTopics, amount.Bytes(), 1, hash.ZeroHash256, 0, false}
	normalLog      = &action.Log{addr, []hash.Hash256{senderTopic, recverTopic}, amount.Bytes(), 1, hash.ZeroHash256, 0, false}
	allLogs        = []*action.Log{evmLog, createLog, depositLog, withdrawLog, selfstakeLog, registerLog}

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
			// success but not implicit transfer log
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

func TestIsSystemLog(t *testing.T) {
	r := require.New(t)

	r.Equal(addr, stakingAddr.String())
	for i, log := range allLogs {
		r.Equal(i == 0, log.IsEvmTransfer())
		r.Equal(i == 1, log.IsCreateBucket())
		r.Equal(i == 2, log.IsDepositBucket())
		r.Equal(i == 3, log.IsWithdrawBucket())
		r.Equal(i == 4, log.IsCandidateSelfStake())
		r.Equal(i == 5, log.IsCandidateRegister())

		// test wrong recipient
		if log.IsCreateBucket() {
			log.Topics[2] = recverTopic
			r.False(log.IsCreateBucket())
			log.Topics[2] = stakingTopic
		}

		if log.IsWithdrawBucket() {
			log.Topics[1] = recverTopic
			r.False(log.IsWithdrawBucket())
			log.Topics[1] = stakingTopic
		}

		if log.IsCandidateRegister() {
			log.Topics[2] = recverTopic
			r.False(log.IsCandidateRegister())
			log.Topics[2] = rewardTopic
		}
	}

	r.False(normalLog.IsEvmTransfer())
	r.False(normalLog.IsCreateBucket())
	r.False(normalLog.IsDepositBucket())
	r.False(normalLog.IsWithdrawBucket())
	r.False(normalLog.IsCandidateSelfStake())
	r.False(normalLog.IsCandidateRegister())
}

func validateSystemLog(r *require.Assertions, log *action.Log, rec *TokenTxRecord) bool {
	if !log.IsImplicitTransfer() {
		return false
	}
	r.Equal(log.Topics[0], hash.BytesToHash256(rec.topic))
	txAmount := new(big.Int).SetBytes(log.Data)
	r.Equal(txAmount.String(), rec.amount)
	from, _ := address.FromBytes(log.Topics[1][12:])
	r.Equal(from.String(), rec.sender)
	to, _ := address.FromBytes(log.Topics[2][12:])
	r.Equal(to.String(), rec.recipient)
	return true
}

func TestReceiptSystemLog(t *testing.T) {
	r := require.New(t)

	for _, v := range receiptTest {
		sysLog := ReceiptImplicitTransferLog(v.r)
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
	blkLog := blk.ImplicitTransferLog()
	r.Nil(blkLog)
	blk.Receipts = []*action.Receipt{}
	blkLog = blk.ImplicitTransferLog()
	r.Nil(blkLog)
	// normal log is not implict transfer
	blk.Receipts = append(blk.Receipts, &action.Receipt{
		Status: uint64(iotextypes.ReceiptStatus_Success), Logs: []*action.Log{normalLog},
	})
	blkLog = blk.ImplicitTransferLog()
	r.Nil(blkLog)

	blk.Receipts = blk.Receipts[:0]
	implicitTransferNum := 0
	for i := range receiptTest {
		if receiptTest[i].num > 0 {
			blk.Receipts = append(blk.Receipts, receiptTest[i].r)
			implicitTransferNum++
		}
	}
	blkLog = blk.ImplicitTransferLog()
	r.EqualValues(implicitTransferNum, blkLog.numActions)
	r.Equal(implicitTransferNum, len(blkLog.actionLogs))

	// test serialize/deserialize
	b := blkLog.Serialize()
	r.NotNil(b)
	pb, err := DeserializeSystemLogPb(b)
	r.NoError(err)

	// verify block systemlog pb message
	r.EqualValues(implicitTransferNum, pb.NumTransactions)
	r.Equal(implicitTransferNum, len(pb.ImplicitTransferLog))
	for i, sysLog := range pb.ImplicitTransferLog {
		receipt := blk.Receipts[i]
		r.Equal(receipt.ActionHash, hash.BytesToHash256(sysLog.ActionHash))
		r.EqualValues(len(receipt.Logs), sysLog.NumTransactions)
		r.Equal(len(receipt.Logs), len(sysLog.Transactions))
		for i, rec := range sysLog.Transactions {
			// verify token tx record
			rec := &TokenTxRecord{
				topic:     rec.Topic,
				amount:    rec.Amount,
				sender:    rec.Sender,
				recipient: rec.Recipient,
			}
			r.True(validateSystemLog(r, receipt.Logs[i], rec))
		}
	}
}
