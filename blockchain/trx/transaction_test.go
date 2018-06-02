// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package trx

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/blake2b"

	"github.com/iotexproject/iotex-core/iotxaddress"
	ta "github.com/iotexproject/iotex-core/test/testaddress"
)

const (
	testCoinbaseData = "The Times 03/Jan/2009 Chancellor on brink of second bailout for banks"
)

func TestTransaction(t *testing.T) {
	assert := assert.New(t)

	// create some transactions
	cbtx := NewCoinbaseTx(ta.Addrinfo["miner"].RawAddress, 50<<22, testCoinbaseData)
	assert.NotNil(cbtx)
	hash := cbtx.Hash()

	if assert.True(cbtx.IsCoinbase()) {
		/*
			t.Logf("version = %x, numTxIn = %x, numTxOut = %x", cbtx.version, cbtx.numTxIn, cbtx.numTxOut )
			t.Logf("Vin.hash = %x", cbtx.vin[0].txHash )
			t.Logf("Vin.outIndex = %x, Vin.length = %d", cbtx.vin[0].outIndex, cbtx.vin[0].unlockScriptSize )
			t.Logf("Vin.script = %x", cbtx.vin[0].unlockScript )
			t.Logf("Vin.sequence = %x", cbtx.vin[0].sequence )
			t.Logf("numTxOut = %x", cbtx.numTxOut )
			t.Logf("Vout.value = %x, Vout.length = %d", cbtx.vout[0].Value, cbtx.vout[0].lockScriptSize)
			t.Logf("Vout.script = %x", cbtx.vout[0].PubKeyHash)
		*/
		t.Log("Create coinbase pass")
	}

	// verify byte stream size
	assert.Equal(uint32(113), cbtx.TxIn[0].TotalSize())
	t.Log("TxIn size match pass")

	assert.Equal(uint32(37), cbtx.TxOut[0].TotalSize())
	t.Log("TxOut size match pass")

	assert.Equal(uint32(158), cbtx.TotalSize())
	t.Log("Tx size match pass")

	// verify coinbase transaction hash value
	expected, _ := hex.DecodeString("1a0130efd104f6236dac0f1c8c6817465f082e5555c29ef8878abb5ddc8c6002")
	assert.Equal(expected, hash[:])
	t.Logf("Coinbase hash = %x match", hash)

	// serialize
	data, err := cbtx.Serialize()
	assert.Nil(err)
	/*
		t.Logf("Serialized = %x", data)
		t.Logf("Version = %d, NumIn = %d, NumOut = %d, LockTime = %d",
			cbtx.Version, cbtx.NumTxIn, cbtx.NumTxOut, cbtx.LockTime)
		t.Logf("Vin.hash = %x", cbtx.TxIn[0].TxHash)
		t.Logf("Vin.outIndex = %d, Vin.scriptSize = %d, Vin.sequence = %x",
			cbtx.TxIn[0].OutIndex, cbtx.TxIn[0].UnlockScriptSize, cbtx.TxIn[0].Sequence)
		t.Logf("Vin.script = %s", cbtx.TxIn[0].UnlockScript)
		t.Logf("Vout.value = %d, Vin.scriptSize = %d", cbtx.TxOut[0].Value, cbtx.TxOut[0].LockScriptSize)
	*/
	t.Log("Marshaling Tx pass")

	// deserialize
	newTx := Tx{}
	err = newTx.Deserialize(data)
	assert.Nil(err)
	/*
		t.Logf("Version = %d, NumIn = %d, NumOut = %d, LockTime = %d",
			newTx.Version, newTx.NumTxIn, newTx.NumTxOut, newTx.LockTime)
		t.Logf("Vin.hash = %x", newTx.TxIn[0].TxHash)
		t.Logf("Vin.outIndex = %d, Vin.scriptSize = %d, Vin.sequence = %x",
			newTx.TxIn[0].OutIndex, newTx.TxIn[0].UnlockScriptSize, newTx.TxIn[0].Sequence)
		t.Logf("Vin.script = %s", newTx.TxIn[0].UnlockScript)
		t.Logf("Vout.value = %d, Vin.scriptSize = %d", newTx.TxOut[0].Value, newTx.TxOut[0].LockScriptSize)
	*/
	t.Log("Unmarshaling Tx pass")

	// compare hash
	newHash := blake2b.Sum256(newTx.ByteStream())
	newHash = blake2b.Sum256(newHash[:])

	assert.Equal(hash[:], newHash[:])
	t.Logf("Serialize/Deserialize Tx hash = %x match\n", hash)

	// a/(b+c) + b/(a+c) + c/(a+b) = 4
	//var a float64 = 154476802108746166441951315019919837485664325669565431700026634898253202035277999
	//var b float64 = 36875131794129999827197811565225474825492979968971970996283137471637224634055579
	//var c float64 = 4373612677928697257861252602371390152816537558161613618621437993378423467772036
}

func TestIsLockedWithKey(t *testing.T) {
	addr := ta.Addrinfo["miner"].RawAddress
	cbtx := NewCoinbaseTx(addr, 100, testCoinbaseData)
	assert.NotNil(t, cbtx)
	assert.False(t, cbtx.TxOut[0].IsLockedWithKey([]byte("tooshort")))
	assert.True(t, cbtx.TxOut[0].IsLockedWithKey(iotxaddress.GetPubkeyHash(addr)))
}
