// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"
	"testing"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"google.golang.org/protobuf/proto"
)

// FuzzReceiptDeserialize asserts that Receipt.Deserialize never panics on
// arbitrary input bytes. Seed corpus covers empty, populated, and
// blob-fee-bearing receipts to exercise all decode branches.
func FuzzReceiptDeserialize(f *testing.F) {
	seeds := []*Receipt{
		{},
		{Status: 1, BlockHeight: 100, GasConsumed: 21000, ContractAddress: "io1aaa"},
		{Status: 1, GasConsumed: 50000, BlobGasUsed: 131072, BlobGasPrice: big.NewInt(1), EffectiveGasPrice: big.NewInt(1000000000)},
		(&Receipt{Status: 1, BlockHeight: 1}).AddLogs(&Log{
			Address: "io1aaa",
			Topics:  Topics{hash.ZeroHash256},
			Data:    []byte("hello"),
		}),
	}
	for _, r := range seeds {
		if b, err := r.Serialize(); err == nil {
			f.Add(b)
		}
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Receipt.Deserialize panicked on %d bytes: %v", len(data), r)
			}
		}()
		var rec Receipt
		_ = rec.Deserialize(data)
	})
}

// FuzzLogDeserialize asserts that Log.Deserialize never panics on
// arbitrary input bytes.
func FuzzLogDeserialize(f *testing.F) {
	seeds := []*Log{
		{},
		{Address: "io1aaa", Topics: Topics{hash.ZeroHash256}, Data: []byte("hello")},
		{Address: "io1bbb", BlockHeight: 100, Index: 1, TxIndex: 2},
		{Topics: Topics{hash.ZeroHash256, hash.ZeroHash256}, NotFixTopicCopyBug: true},
	}
	for _, l := range seeds {
		if b, err := l.Serialize(); err == nil {
			f.Add(b)
		}
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Log.Deserialize panicked on %d bytes: %v", len(data), r)
			}
		}()
		var l Log
		_ = l.Deserialize(data)
	})
}

// FuzzActionCoreLoadProto asserts that ActionCore decoding
// (envelope.LoadProto and every action's LoadProto) does not panic on
// arbitrary input. The seed corpus includes an empty putPollResult as a
// regression case.
func FuzzActionCoreLoadProto(f *testing.F) {
	seeds := []*iotextypes.ActionCore{
		{Action: &iotextypes.ActionCore_PutPollResult{PutPollResult: &iotextypes.PutPollResult{}}},
		{Action: &iotextypes.ActionCore_Transfer{Transfer: &iotextypes.Transfer{Amount: "1", Recipient: "io1aaa"}}},
		{Action: &iotextypes.ActionCore_Execution{Execution: &iotextypes.Execution{Amount: "1"}}},
		{Action: &iotextypes.ActionCore_StakeCreate{StakeCreate: &iotextypes.StakeCreate{StakedAmount: "1"}}},
		{Action: &iotextypes.ActionCore_CandidateRegister{CandidateRegister: &iotextypes.CandidateRegister{}}},
		{Action: &iotextypes.ActionCore_GrantReward{GrantReward: &iotextypes.GrantReward{}}},
		{Action: &iotextypes.ActionCore_DepositToRewardingFund{DepositToRewardingFund: &iotextypes.DepositToRewardingFund{Amount: "1"}}},
		{Action: &iotextypes.ActionCore_ClaimFromRewardingFund{ClaimFromRewardingFund: &iotextypes.ClaimFromRewardingFund{Amount: "1"}}},
	}
	for _, c := range seeds {
		if b, err := proto.Marshal(c); err == nil {
			f.Add(b)
		}
	}
	f.Add([]byte("\x92\x03\x00")) // empty putPollResult

	f.Fuzz(func(t *testing.T, data []byte) {
		core := &iotextypes.ActionCore{}
		if proto.Unmarshal(data, core) != nil {
			return
		}
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("envelope.LoadProto panicked on %d bytes: %v", len(data), r)
			}
		}()
		_ = (&envelope{}).LoadProto(core)
	})
}
