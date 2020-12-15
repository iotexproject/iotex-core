// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"
)

func newTestLog() *Log {
	return &Log{
		Address:     "1",
		Data:        []byte("cd07d8a74179e032f030d9244"),
		BlockHeight: 1,
		ActionHash:  hash.ZeroHash256,
		Index:       1,
	}

}

func TestConvert(t *testing.T) {
	require := require.New(t)

	topics := []hash.Hash256{
		hash.Hash256b([]byte("test")),
		hash.Hash256b([]byte("Pacific")),
		hash.Hash256b([]byte("Aleutian")),
	}
	testLog := newTestLog()
	testLog.Topics = topics
	testLog.NotFixTopicCopyBug = true
	receipt := &Receipt{1, 1, hash.ZeroHash256, 1, "test", []*Log{testLog}, nil, "balance not enough"}

	typeReceipt := receipt.ConvertToReceiptPb()
	require.NotNil(typeReceipt)
	receipt2 := &Receipt{}
	receipt2.ConvertFromReceiptPb(typeReceipt)
	require.Equal(receipt.Status, receipt2.Status)
	require.Equal(receipt.BlockHeight, receipt2.BlockHeight)
	require.Equal(receipt.ActionHash, receipt2.ActionHash)
	require.Equal(receipt.GasConsumed, receipt2.GasConsumed)
	require.Equal(receipt.ContractAddress, receipt2.ContractAddress)
	require.Equal(receipt.executionRevertMsg, receipt2.executionRevertMsg)
	// block earlier than AleutianHeight overwrites all topics with last topic data
	require.NotEqual(testLog, receipt2.logs[0])
	h := receipt.Hash()

	testLog.NotFixTopicCopyBug = false
	typeReceipt = receipt.ConvertToReceiptPb()
	require.NotNil(typeReceipt)
	receipt2 = &Receipt{}
	receipt2.ConvertFromReceiptPb(typeReceipt)
	require.Equal(receipt, receipt2)
	require.NotEqual(h, receipt.Hash())
}

func TestSerDer(t *testing.T) {
	require := require.New(t)
	receipt := &Receipt{1, 1, hash.ZeroHash256, 1, "", nil, nil, ""}
	ser, err := receipt.Serialize()
	require.NoError(err)

	receipt2 := &Receipt{}
	receipt2.Deserialize(ser)
	require.Equal(receipt.Status, receipt2.Status)
	require.Equal(receipt.BlockHeight, receipt2.BlockHeight)
	require.Equal(receipt.ActionHash, receipt2.ActionHash)
	require.Equal(receipt.GasConsumed, receipt2.GasConsumed)
	require.Equal(receipt.ContractAddress, receipt2.ContractAddress)

	hash := receipt.Hash()
	oldHash := "9b1d77d8b8902e8d4e662e7cd07d8a74179e032f030d92441ca7fba1ca68e0f4"
	require.Equal(oldHash, hex.EncodeToString(hash[:]))

	// starting HawaiiHeight execution revert message is added to receipt
	receipt = receipt.SetExecutionRevertMsg("test")
	hash2 := receipt.Hash()
	require.NotEqual(oldHash, hex.EncodeToString(hash2[:]))
}
func TestConvertLog(t *testing.T) {
	require := require.New(t)

	topics := []hash.Hash256{
		hash.ZeroHash256,
		hash.Hash256b([]byte("Pacific")),
		hash.Hash256b([]byte("Aleutian")),
	}
	testLog := newTestLog()
	testLog.Topics = topics
	testLog.NotFixTopicCopyBug = true
	typeLog := testLog.ConvertToLogPb()
	require.NotNil(typeLog)
	log2 := &Log{}
	log2.ConvertFromLogPb(typeLog)
	require.Equal(testLog.Address, log2.Address)
	require.Equal(testLog.Data, log2.Data)
	require.Equal(testLog.BlockHeight, log2.BlockHeight)
	require.Equal(testLog.ActionHash, log2.ActionHash)
	require.Equal(testLog.Index, log2.Index)
	// block earlier than AleutianHeight overwrites all topics with last topic data
	last := len(log2.Topics) - 1
	for _, v := range log2.Topics[:last] {
		require.Equal(topics[last], v)
	}

	testLog.NotFixTopicCopyBug = false
	typeLog = testLog.ConvertToLogPb()
	require.NotNil(typeLog)
	log2 = &Log{}
	log2.ConvertFromLogPb(typeLog)
	require.Equal(testLog, log2)
}
func TestSerDerLog(t *testing.T) {
	require := require.New(t)

	topics := []hash.Hash256{
		hash.ZeroHash256,
		hash.Hash256b([]byte("Pacific")),
		hash.Hash256b([]byte("Aleutian")),
	}
	testLog := newTestLog()
	testLog.Topics = topics
	testLog.NotFixTopicCopyBug = true
	typeLog, err := testLog.Serialize()
	require.NoError(err)
	log2 := &Log{}
	log2.Deserialize(typeLog)
	require.Equal(testLog.Address, log2.Address)
	require.Equal(testLog.Data, log2.Data)
	require.Equal(testLog.BlockHeight, log2.BlockHeight)
	require.Equal(testLog.ActionHash, log2.ActionHash)
	require.Equal(testLog.Index, log2.Index)
	// block earlier than AleutianHeight overwrites all topics with last topic data
	last := len(log2.Topics) - 1
	for _, v := range log2.Topics[:last] {
		require.Equal(topics[last], v)
	}

	testLog.NotFixTopicCopyBug = false
	typeLog, err = testLog.Serialize()
	require.NoError(err)
	log2 = &Log{}
	log2.Deserialize(typeLog)
	require.Equal(testLog, log2)
}
