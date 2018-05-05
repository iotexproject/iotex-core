// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"fmt"
	"math/big"

	"github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/proto"
	"github.com/iotexproject/iotex-core/txvm"
)

// UtxoEntry contain TxOutput + hash and index
type UtxoEntry struct {
	*iproto.TxOutputPb // embedded

	// below fields only used internally, not part of serialize/deserialize
	txHash   common.Hash32B
	outIndex int32 // outIndex is needed when spending UTXO
}

// UtxoTracker tracks the active UTXO pool
type UtxoTracker struct {
	currOutIndex int32 // newly created output index
	utxoPool     map[common.Hash32B][]*TxOutput
}

// NewUtxoTracker returns a UTXO tracker instance
func NewUtxoTracker() *UtxoTracker {
	return &UtxoTracker{0, map[common.Hash32B][]*TxOutput{}}
}

// UtxoEntries returns list of UTXO entries containing >= requested amount, and
// return (nil, addr's total balance) if cannot reach reqamount
func (tk *UtxoTracker) UtxoEntries(address string, reqamount uint64) ([]*UtxoEntry, *big.Int) {
	list := []*UtxoEntry{}
	balance := big.NewInt(0)
	tmp := big.NewInt(0)
	key := iotxaddress.GetPubkeyHash(address)
	hasEnoughFund := false

found:
	for hash, txOut := range tk.utxoPool {
		for _, out := range txOut {
			if out.IsLockedWithKey(key) {
				utxo := UtxoEntry{out.TxOutputPb, hash, out.outIndex}
				list = append(list, &utxo)
				balance.Add(balance, tmp.SetUint64(out.Value))

				if tmp.SetUint64(reqamount).Cmp(balance) == 0 || tmp.SetUint64(reqamount).Cmp(balance) == -1 {
					balance.Sub(balance, tmp.SetUint64(reqamount))
					hasEnoughFund = true
					break found
				}
			}
		}
	}

	if hasEnoughFund {
		return list, balance
	}
	return nil, balance
}

// CreateTxInputUtxo returns a UTXO transaction input
func (tk *UtxoTracker) CreateTxInputUtxo(hash common.Hash32B, index int32, unlockScript []byte) *TxInput {
	return NewTxInput(hash, index, unlockScript, 0)
}

// CreateTxOutputUtxo creates transaction to spend UTXO
func (tk *UtxoTracker) CreateTxOutputUtxo(address string, amount uint64) *TxOutput {
	out := NewTxOutput(amount, tk.currOutIndex)
	locks, err := txvm.PayToAddrScript(address)
	if err != nil {
		return nil
	}
	out.LockScript = locks

	out.LockScriptSize = uint32(len(out.LockScript))
	// increment the index for new output
	tk.currOutIndex++

	return out
}

// ValidateTxInputUtxo validates the UTXO in transaction input
// return amount of UTXO if pass, 0 otherwise
func (tk *UtxoTracker) ValidateTxInputUtxo(txIn *TxInput) uint64 {
	hash := common.ZeroHash32B
	copy(hash[:], txIn.TxHash)
	unspent, exist := tk.utxoPool[hash]

	// if hash does not exist in UTXO pool, it is spoof/fraudulent spending
	if !exist {
		fmt.Errorf("UTXO %x does not exist", hash)
		return 0
	}

	// check transaction input, including unlock script can pass authentication

	for _, utxo := range unspent {
		if utxo.outIndex == txIn.OutIndex && txIn.UnlockSuccess(utxo.LockScript) {
			return utxo.Value
		}
	}

	return 0
}

// ValidateUtxo validates all UTXO in the block
func (tk *UtxoTracker) ValidateUtxo(blk *Block) error {
	// iterate thru all transactions of this block
	for _, tx := range blk.Tranxs {
		txHash := tx.Hash()

		// coinbase has 1 output which becomes UTXO
		if tx.IsCoinbase() {
			continue
		}

		credit := uint64(0)
		for _, txIn := range tx.TxIn {
			// verify UTXO before they can be spent
			amount := tk.ValidateTxInputUtxo(txIn)
			if amount == 0 {
				return fmt.Errorf("Cannot validate UTXO %x", txIn.TxHash)
			}

			// sum up all UTXO
			credit += uint64(amount)
		}

		debit := uint64(0)
		for _, txOut := range tx.TxOut {
			debit += uint64(txOut.Value)
		}

		// make sure we have enough fund to spend
		if credit < debit {
			return fmt.Errorf("Tx %x does not have enough UTXO to spend", txHash)
		}
	}

	return nil
}

// Reset reset the out index
func (tk *UtxoTracker) Reset() {
	// reset output index
	tk.currOutIndex = 0
}

// UpdateUtxoPool updates the UTXO pool according to transactions in the block
func (tk *UtxoTracker) UpdateUtxoPool(blk *Block) error {
	// iterate thru all transactions of this block
	for _, tx := range blk.Tranxs {
		txHash := tx.Hash()

		// coinbase has 1 output which becomes UTXO
		if tx.IsCoinbase() {
			tk.utxoPool[txHash] = []*TxOutput{tx.TxOut[0]}
			continue
		}

		// add new TxOutput into pool
		utxo := []*TxOutput{}
		for _, txOut := range tx.TxOut {
			utxo = append(utxo, txOut)
		}
		tk.utxoPool[txHash] = utxo

		// remove TxInput from pool
		for _, txIn := range tx.TxIn {
			hash := common.ZeroHash32B
			copy(hash[:], txIn.TxHash)
			unspent, _ := tk.utxoPool[hash]

			if len(unspent) == 1 {
				// this is the only UTXO so remove this entry
				delete(tk.utxoPool, hash)
			} else {
				// remove this UTXO from the entry
				newUnspent := []*TxOutput{}
				for _, entry := range unspent {
					if entry.outIndex != txIn.OutIndex {
						newUnspent = append(newUnspent, entry)
					}
				}
				tk.utxoPool[hash] = newUnspent
			}
		}
	}

	return nil
}

// ConvertToUtxoPb creates a protobuf's UTXO
func (tx *Tx) ConvertToUtxoPb() *iproto.UtxoMapPb {
	return nil
}

// Serialize returns a serialized byte stream for the Tx
func (tk *UtxoTracker) Serialize() ([]byte, error) {
	return nil, nil
}

// Deserialize parse the byte stream into the Tx
func (tk *UtxoTracker) Deserialize(buf []byte) error {
	return nil
}

// GetPool returns the UTXO pool
func (tk *UtxoTracker) GetPool() map[common.Hash32B][]*TxOutput {
	return tk.utxoPool
}

// AddTx is called by TxPool to add a transaction
func (tk *UtxoTracker) AddTx(tx *Tx, height uint32) {
	hash := tx.Hash()
	outputs, exists := tk.utxoPool[hash]
	if !exists {
		outputs = []*TxOutput{}
	}
	for _, out := range tx.TxOut {
		// check script lock
		outputs = append(outputs, out)
	}
	tk.utxoPool[hash] = outputs
}
