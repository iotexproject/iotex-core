// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/hex"
	"testing"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/stretchr/testify/require"
)

func TestConvert(t *testing.T) {
	require := require.New(t)

	topics := []hash.Hash256{
		hash.ZeroHash256,
		hash.Hash256b([]byte("Pacific")),
		hash.Hash256b([]byte("Aleutian")),
	}
	log := &Log{"1", topics, []byte("cd07d8a74179e032f030d9244"), 1, hash.ZeroHash256, 1, true}
	receipt := &Receipt{1, 1, hash.ZeroHash256, 1, "test", []*Log{log}}

	typeReipt := receipt.ConvertToReceiptPb()
	require.NotNil(typeReipt)
	receipt2 := &Receipt{}
	receipt2.ConvertFromReceiptPb(typeReipt)
	require.Equal(receipt.Status, receipt2.Status)
	require.Equal(receipt.BlockHeight, receipt2.BlockHeight)
	require.Equal(receipt.ActionHash, receipt2.ActionHash)
	require.Equal(receipt.GasConsumed, receipt2.GasConsumed)
	require.Equal(receipt.ContractAddress, receipt2.ContractAddress)
	// block earlier than AleutianHeight overwrites all topics with last topic data
	require.NotEqual(log, receipt2.Logs[0])
	h := receipt.Hash()

	log.PreAleutian = false
	typeReipt = receipt.ConvertToReceiptPb()
	require.NotNil(typeReipt)
	receipt2 = &Receipt{}
	receipt2.ConvertFromReceiptPb(typeReipt)
	require.Equal(receipt, receipt2)
	require.NotEqual(h, receipt.Hash())
}
func TestSerDer(t *testing.T) {
	require := require.New(t)
	receipt := &Receipt{1, 1, hash.ZeroHash256, 1, "", nil}
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
	require.Equal("9b1d77d8b8902e8d4e662e7cd07d8a74179e032f030d92441ca7fba1ca68e0f4", hex.EncodeToString(hash[:]))
}
func TestConvertLog(t *testing.T) {
	require := require.New(t)

	topics := []hash.Hash256{
		hash.ZeroHash256,
		hash.Hash256b([]byte("Pacific")),
		hash.Hash256b([]byte("Aleutian")),
	}
	log := &Log{"1", topics, []byte("cd07d8a74179e032f030d9244"), 1, hash.ZeroHash256, 1, true}

	typeLog := log.ConvertToLogPb()
	require.NotNil(typeLog)
	log2 := &Log{}
	log2.ConvertFromLogPb(typeLog)
	require.Equal(log.Address, log2.Address)
	require.Equal(log.Data, log2.Data)
	require.Equal(log.BlockHeight, log2.BlockHeight)
	require.Equal(log.ActionHash, log2.ActionHash)
	require.Equal(log.Index, log2.Index)
	// block earlier than AleutianHeight overwrites all topics with last topic data
	require.Equal(topics[2], log2.Topics[0])
	require.Equal(topics[2], log2.Topics[1])
	require.Equal(topics[2], log2.Topics[2])

	log.PreAleutian = false
	typeLog = log.ConvertToLogPb()
	require.NotNil(typeLog)
	log2 = &Log{}
	log2.ConvertFromLogPb(typeLog)
	require.Equal(log, log2)
}
func TestSerDerLog(t *testing.T) {
	require := require.New(t)

	topics := []hash.Hash256{
		hash.ZeroHash256,
		hash.Hash256b([]byte("Pacific")),
		hash.Hash256b([]byte("Aleutian")),
	}
	log := &Log{"1", topics, []byte("cd07d8a74179e032f030d9244"), 1, hash.ZeroHash256, 1, true}

	typeLog, err := log.Serialize()
	require.NoError(err)
	log2 := &Log{}
	log2.Deserialize(typeLog)
	require.Equal(log.Address, log2.Address)
	require.Equal(log.Data, log2.Data)
	require.Equal(log.BlockHeight, log2.BlockHeight)
	require.Equal(log.ActionHash, log2.ActionHash)
	require.Equal(log.Index, log2.Index)
	// block earlier than AleutianHeight overwrites all topics with last topic data
	require.Equal(topics[2], log2.Topics[0])
	require.Equal(topics[2], log2.Topics[1])
	require.Equal(topics[2], log2.Topics[2])

	log.PreAleutian = false
	typeLog, err = log.Serialize()
	require.NoError(err)
	log2 = &Log{}
	log2.Deserialize(typeLog)
	require.Equal(log, log2)
}
