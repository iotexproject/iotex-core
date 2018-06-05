// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package txpool

import (
	"container/heap"
	"encoding/hex"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	. "github.com/iotexproject/iotex-core/blockchain"
	trx "github.com/iotexproject/iotex-core/blockchain/trx"
	"github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/proto"
	ta "github.com/iotexproject/iotex-core/test/testaddress"
)

const (
	testingConfigPath = "../config.yaml"
	testCoinbaseData  = "The Times 03/Jan/2009 Chancellor on brink of second bailout for banks"
)

func decodeHash(in string) []byte {
	hash, _ := hex.DecodeString(in)
	return hash
}

func TestTxPool(t *testing.T) {
	assert := assert.New(t)

	cfg, err := config.LoadConfigWithPathWithoutValidation(testingConfigPath)
	assert.Nil(err)
	// disable account-based testing
	cfg.Chain.TrieDBPath = ""
	cfg.Chain.InMemTest = true
	// Disable block reward to make bookkeeping easier
	Gen.BlockReward = uint64(0)
	bc := CreateBlockchain(cfg, Gen, nil)
	assert.NotNil(bc)
	defer bc.Stop()

	tp := NewTxPool(bc)
	assert.NotNil(tp)
	cbTx := trx.NewCoinbaseTx(ta.Addrinfo["miner"].RawAddress, 50, testCoinbaseData)
	assert.NotNil(cbTx)
	if _, err := tp.ProcessTx(cbTx, true, false, 13245); assert.NotNil(err) {
		t.Logf("Coinbase Tx cannot be processed")
	}
	/*
		if true {
			pool := bc.UtxoPool()
			fmt.Println("utxoPool before tx1:", len(pool))
			for hash, _ := range pool {
				fmt.Printf("hash:%x\n", hash)
			}
			payees := []*Payee{}
			payees = append(payees, &Payee{Addrinfo["alfa"].Address, 10})
			payees = append(payees, &Payee{Addrinfo["bravo"].Address, 1})
			payees = append(payees, &Payee{Addrinfo["charlie"].Address, 1})
			payees = append(payees, &Payee{Addrinfo["delta"].Address, 1})
			payees = append(payees, &Payee{Addrinfo["echo"].Address, 1})
			payees = append(payees, &Payee{Addrinfo["foxtrot"].Address, 5})
			tx1 := bc.CreateTransaction(Addrinfo["miner"], 19, payees)
			fmt.Printf("tx1: %x\n", tx1.Hash())
			fmt.Println("version:", tx1.Version)
			fmt.Println("NumTxIn:", tx1.NumTxIn)
			fmt.Println("TxIn:")
			for idx, txIn := range tx1.TxIn {
				hash := ZeroHash32B
				copy(hash[:], txIn.TxHash)
				fmt.Printf("tx1.TxIn.%d %x %d %x\n", idx, hash, txIn.UnlockScriptSize, txIn.UnlockScript)
			}
			fmt.Println("NumTxOut:", tx1.NumTxOut)
			fmt.Println("TxOut:")
			for idx, txOut := range tx1.TxOut {
				fmt.Printf("tx1.TxOut.%d %d %x %d\n", idx, txOut.LockScriptSize, txOut.LockScript, txOut.Value)
			}
			blk := bc.MintNewBlock([]*Tx{tx1}, Addrinfo["miner"], "")
			fmt.Println(blk)
			err := bc.AddBlockCommit(blk)
			assert.Nil(err)
			fmt.Println("Add 1st block")
			bc.ResetUTXO()
			payees = nil
			payees = append(payees, &Payee{Addrinfo["bravo"].Address, 3})
			payees = append(payees, &Payee{Addrinfo["delta"].Address, 2})
			payees = append(payees, &Payee{Addrinfo["echo"].Address, 1})
			tx2 := bc.CreateTransaction(Addrinfo["alfa"], 6, payees)
			fmt.Printf("tx2: %x\n", tx2.Hash())
			fmt.Println("tx2.TxIn:", tx2.NumTxIn)
			for idx, txIn := range tx2.TxIn {
				hash := ZeroHash32B
				copy(hash[:], txIn.TxHash)
				fmt.Printf("tx2.TxIn.%d %x %d %x\n", idx, hash, txIn.UnlockScriptSize, txIn.UnlockScript)
			}
			fmt.Println("TxOut:")
			for idx, txOut := range tx2.TxOut {
				fmt.Printf("tx2.TxOut.%d %d %x %d\n", idx, txOut.LockScriptSize, txOut.LockScript, txOut.Value)
			}
			fmt.Println(tx2.TxIn)
			return
		} //*/
	txHash := common.ZeroHash32B
	copy(txHash[:], decodeHash("9de6306b08158c423330f7a27243a1a5cbe39bfd764f07818437882d21241567"))
	txIn1_0 := trx.NewTxInput(
		txHash, 0,
		decodeHash("40f9ea2b1357dde55519246a6ad82c466b9f2b988ff81a7c2fb114c932d44f322ba2edd178c2326739638b536e5f803977c24332b8f5b8ebc5f6683ff2bcaad90720b9b8d7316705dc4ff62bb323e610f3f5072abedc9834e999d6537f6681284ea2"),
		0)
	txOut1_0 := trx.NewTxOutput(10, 0)
	txOut1_0.LockScriptSize = 25
	txOut1_0.LockScript = decodeHash("65b014a97ce8e76ade9b3181c63432a62330a5ca83ab9ba1b1")
	txOut1_1 := trx.NewTxOutput(1, 1)
	txOut1_1.LockScriptSize = 25
	txOut1_1.LockScript = decodeHash("65b014af33097c8fd571c6c1efc52b0a802514ea0fbb03a1b1")
	txOut1_2 := trx.NewTxOutput(1, 2)
	txOut1_2.LockScriptSize = 25
	txOut1_2.LockScript = decodeHash("65b0140fb02223c1a78c3f1fb81a1572e8b07adb700bffa1b1")
	txOut1_3 := trx.NewTxOutput(1, 3)
	txOut1_3.LockScriptSize = 25
	txOut1_3.LockScript = decodeHash("65b01443251ba4fd765a2cfa65256aabd64f98c5c00e40a1b1")
	txOut1_4 := trx.NewTxOutput(1, 4)
	txOut1_4.LockScriptSize = 25
	txOut1_4.LockScript = decodeHash("65b01430f1db72a44136e8634121b6730c2b8ef094f1c9a1b1")
	txOut1_5 := trx.NewTxOutput(5, 5)
	txOut1_5.LockScriptSize = 25
	txOut1_5.LockScript = decodeHash("65b014d94ee6c7205e85c3d97c557f08faf8ac41102806a1b1")
	txOut1_6 := trx.NewTxOutput(9999999981, 6)
	txOut1_6.LockScriptSize = 25
	txOut1_6.LockScript = decodeHash("65b014d4f743a24d5386f8d1c2a648da7015f08800cd11a1b1")
	tx1 := &trx.Tx{
		Version:  1,
		TxIn:     []*trx.TxInput{txIn1_0},
		TxOut:    []*trx.TxOutput{txOut1_0, txOut1_1, txOut1_2, txOut1_3, txOut1_4, txOut1_5, txOut1_6},
		LockTime: 0,
	}

	copy(txHash[:], decodeHash("aeedd06eb44f08abbcc72a2293aff580f13662fa59cc1b0aa4a15ee7c118e4eb"))
	txIn2_0 := trx.NewTxInput(
		txHash, 0,
		decodeHash("40535e20b5c5075fa80d6bff220aea755737e3787bfbc7122c0c45015f6c249fbca28d069dc028fad01fda2766ea90411aad38ce9a9de7c59a30e4bebc80b940002009d8c6fc6f5cb0a03df112da90486fad7cdece1501aaab658551f8afbe7f59ee"),
		0)
	txOut2_0 := trx.NewTxOutput(3, 0)
	txOut2_0.LockScriptSize = 25
	txOut2_0.LockScript = decodeHash("65b014af33097c8fd571c6c1efc52b0a802514ea0fbb03a1b1")
	txOut2_1 := trx.NewTxOutput(2, 1)
	txOut2_1.LockScriptSize = 25
	txOut2_1.LockScript = decodeHash("65b01443251ba4fd765a2cfa65256aabd64f98c5c00e40a1b1")
	txOut2_2 := trx.NewTxOutput(1, 2)
	txOut2_2.LockScriptSize = 25
	txOut2_2.LockScript = decodeHash("65b01430f1db72a44136e8634121b6730c2b8ef094f1c9a1b1")
	txOut2_3 := trx.NewTxOutput(4, 2)
	txOut2_3.LockScriptSize = 25
	txOut2_3.LockScript = decodeHash("65b014a97ce8e76ade9b3181c63432a62330a5ca83ab9ba1b1")
	tx2 := &trx.Tx{
		Version:  1,
		TxIn:     []*trx.TxInput{txIn2_0},
		TxOut:    []*trx.TxOutput{txOut2_0, txOut2_1, txOut2_2, txOut2_3},
		LockTime: 0,
	}

	t.Logf("tx1 hash: %x", tx1.Hash())
	t.Logf("tx2 hash: %x", tx2.Hash())
	descs, err := tp.ProcessTx(tx2, true, false, 12341234)
	assert.Nil(err)
	assert.Equal(0, len(descs))
	descs, err = tp.ProcessTx(tx1, true, false, 12341234)
	t.Log(tp.TxDescs())
	for hash, desc := range tp.TxDescs() {
		t.Logf("hash: %x desc: %v", hash, desc)
	}
	assert.Nil(err)
	// TODO: refactor this test
	//assert.Equal(2, len(tp.TxDescs()))
}

func TestUpdateTxDescPriority(t *testing.T) {
	// Create four dummy TxOutputs
	// TxOutputSize = ValueSizeInBytes + LockScriptSizeInBytes + uint32(out.LockScriptSize)
	// txOutput1 size = 13
	txOutput1 := trx.TxOutput{TxOutputPb: &iproto.TxOutputPb{1, 1, nil}}
	// txOutput2 size = 14
	txOutput2 := trx.TxOutput{TxOutputPb: &iproto.TxOutputPb{2, 2, nil}}
	// txOutput3 size = 15
	txOutput3 := trx.TxOutput{TxOutputPb: &iproto.TxOutputPb{3, 3, nil}}
	// txOutput4 size = 16
	txOutput4 := trx.TxOutput{TxOutputPb: &iproto.TxOutputPb{4, 4, nil}}

	// Create four dummy Txs
	// TxSize = uint32(VersionSizeInBytes + NumTxInSizeInBytes + NumTxOutSizeInBytes + LockTimeSizeInBytes) + TxInputs Size + TxOutputs size
	// tx1 size = 29
	tx1 := trx.Tx{TxOut: []*trx.TxOutput{&txOutput1}}
	// tx2 size = 30
	tx2 := trx.Tx{TxOut: []*trx.TxOutput{&txOutput2}}
	// tx3 size = 31
	tx3 := trx.Tx{TxOut: []*trx.TxOutput{&txOutput3}}
	// tx4 size = 32
	tx4 := trx.Tx{TxOut: []*trx.TxOutput{&txOutput4}}

	// Create four dummy TxDescs
	// TxDesc1
	desc1 := TxDesc{
		Tx:          &tx1,
		AddedTime:   time.Now(),
		BlockHeight: uint64(1),
		Fee:         int64(0),
		FeePerKB:    int64(0),
		Priority:    float64(0),
	}

	// TxDesc2
	desc2 := TxDesc{
		Tx:          &tx2,
		AddedTime:   time.Now(),
		BlockHeight: uint64(1),
		Fee:         int64(0),
		FeePerKB:    int64(0),
		Priority:    float64(0),
	}

	// TxDesc3
	desc3 := TxDesc{
		Tx:          &tx3,
		AddedTime:   time.Now(),
		BlockHeight: uint64(1),
		Fee:         int64(0),
		FeePerKB:    int64(0),
		Priority:    float64(0),
	}

	// TxDesc4
	desc4 := TxDesc{
		Tx:          &tx4,
		AddedTime:   time.Now(),
		BlockHeight: uint64(1),
		Fee:         int64(0),
		FeePerKB:    int64(0),
		Priority:    float64(5),
	}

	tp := txPool{}

	heap.Push(&tp.txDescPriorityQueue, &desc1)
	heap.Push(&tp.txDescPriorityQueue, &desc2)
	heap.Push(&tp.txDescPriorityQueue, &desc3)
	heap.Push(&tp.txDescPriorityQueue, &desc4)

	time.Sleep(time.Second * 6)
	tp.updateTxDescPriority()

	// Expected TxDesc Ordering after sleeping 6 seconds
	expectedOrdering := []*TxDesc{&desc4, &desc3, &desc2, &desc1}
	t.Log("After updating priority for each TxDesc, the priority of each TxDesc in the order of popped is as follows:")
	for i := 0; i < 4; i++ {
		txDesc := heap.Pop(&tp.txDescPriorityQueue).(*TxDesc)
		t.Log(txDesc.Priority)
		assert.Equal(t, expectedOrdering[i], txDesc)
		t.Log()
	}
}
