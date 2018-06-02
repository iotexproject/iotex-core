// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package trx

import (
	"fmt"
	"math/rand"
	"sort"
	"testing"
	"time"

	"github.com/iotexproject/iotex-core/proto"
)

func TestTxInputSorter(t *testing.T) {
	// Randomly generate four Byte Streams of length 32 to simulate dummy TxHashes
	rand.Seed(time.Now().UnixNano())

	hash1 := make([]byte, 32)
	rand.Read(hash1)

	hash2 := make([]byte, 32)
	rand.Read(hash2)

	hash3 := make([]byte, 32)

	hash4 := make([]byte, 32)

	// Randomly generate four Byte Streams of length 4 to simulate dummy Unlock Scripts
	unlock1 := make([]byte, 4)
	rand.Read(unlock1)

	unlock2 := make([]byte, 4)
	rand.Read(unlock2)

	unlock3 := make([]byte, 4)
	rand.Read(unlock3)

	unlock4 := make([]byte, 4)
	rand.Read(unlock4)

	// Generate four TxInput pointers
	txInput1 := TxInput{hash1, int32(0), uint32(len(unlock1)), unlock1, uint32(1)}
	txInput2 := TxInput{hash2, int32(1), uint32(len(unlock2)), unlock2, uint32(1)}
	txInput3 := TxInput{hash3, int32(3), uint32(len(unlock3)), unlock3, uint32(1)}
	txInput4 := TxInput{hash4, int32(2), uint32(len(unlock4)), unlock4, uint32(1)}

	in := []*TxInput{&txInput1, &txInput2, &txInput3, &txInput4}

	// Display the TxInput ordering before sorting
	t.Log("The TxHash and OutIndex of each TxInput before sorting are as follows:")

	for _, txIn := range in {
		t.Logf("TxHash: %s, OutIndex: %d", convertToString(txIn.TxHash), txIn.OutIndex)
	}

	// Sort TxOutput in lexicographical order based on TxHash + OutIndex
	sort.Sort(TxInSorter(in))

	// Display the TxInput ordering after lexicographical sorting
	t.Log("The TxHash and OutIndex of each TxInput after lexicographical sorting are as follows:")

	for _, txIn := range in {
		t.Logf("TxHash: %s, OutIndex: %d", convertToString(txIn.TxHash), txIn.OutIndex)
	}
}

func TestTxOutputSorter(t *testing.T) {
	//Randomly generate four Byte Streams of length 4 to simulate dummy Lock Scripts
	lock1 := make([]byte, 4)
	rand.Read(lock1)

	lock2 := make([]byte, 4)
	rand.Read(lock2)

	lock3 := make([]byte, 4)
	rand.Read(lock3)

	lock4 := make([]byte, 4)
	rand.Read(lock4)

	// Randomly generate four TxOutput pointers
	txOut1 := TxOutput{&iproto.TxOutputPb{uint64(100), uint32(len(lock1)), lock1}, int32(0)}
	txOut2 := TxOutput{&iproto.TxOutputPb{uint64(10), uint32(len(lock2)), lock2}, int32(1)}
	txOut3 := TxOutput{&iproto.TxOutputPb{uint64(50), uint32(len(lock3)), lock3}, int32(2)}
	txOut4 := TxOutput{&iproto.TxOutputPb{uint64(100), uint32(len(lock4)), lock4}, int32(3)}

	out := []*TxOutput{&txOut1, &txOut2, &txOut3, &txOut4}

	// Display the TxOutput ordering before sorting
	t.Log("The Value and LockScript of each TxOutput before sorting are as follows:")

	for _, txOut := range out {
		t.Logf("Value: %d, LockScript: %s", txOut.Value, convertToString(txOut.LockScript))
	}

	// Sort TxOutput in lexicographical order based on Value + LockScript
	sort.Sort(TxOutSorter(out))

	// Display the TxOutput ordering after lexicographical sorting
	t.Log("The Value and LockScript of each TxOutput after lexicographical sorting are as follows:")

	for _, txOut := range out {
		t.Logf("Value: %d, LockScript: %s", txOut.Value, convertToString(txOut.LockScript))
	}
}

// Helper function to transform a byte stream to the string format
func convertToString(bs []byte) string {
	return fmt.Sprintf("%x", bs)
}
