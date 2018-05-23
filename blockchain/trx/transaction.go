// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package transaction

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"math/big"

	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"
	"golang.org/x/crypto/blake2b"

	"github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/proto"
	"github.com/iotexproject/iotex-core/txvm"
)

const (
	// TxOutputPb fields

	// ValueSizeInBytes defines the size of value in byte units
	ValueSizeInBytes = 8
	// LockScriptSizeInBytes defines the size of lock script in byte units
	LockScriptSizeInBytes = 4

	// TxPb fields

	// VersionSizeInBytes defines the size of version in byte units
	VersionSizeInBytes = 4
	// LockTimeSizeInBytes defines the size of lock time in byte units
	LockTimeSizeInBytes = 4
	// NonceSizeInBytes defines the size of nonce in byte units
	NonceSizeInBytes = 8
)

// TxInput defines the transaction input protocol buffer
type TxInput = iproto.TxInputPb

// TxOutput defines the transaction output protocol buffer
type TxOutput struct {
	*iproto.TxOutputPb // embedded

	// below fields only used internally, not part of serialize/deserialize
	OutIndex int32 // outIndex is needed when spending UTXO
}

// Tx defines the struct of transaction
// make sure the variable type and order of this struct is same as "type Tx" in blockchain.pb.go
type Tx struct {
	Version  uint32
	LockTime uint32 // transaction to be locked until this time

	// used by utxo-based model
	TxIn  []*TxInput
	TxOut []*TxOutput

	// used by state-based model
	Nonce           uint64
	Recipient       *iotxaddress.Address
	Amount          *big.Int
	Payload         []byte
	SenderPublicKey []byte
	Signature       []byte
}

// NewTxInput returns a TxInput instance
func NewTxInput(hash common.Hash32B, index int32, unlock []byte, seq uint32) *TxInput {
	return &TxInput{
		hash[:],
		index,
		uint32(len(unlock)),
		unlock,
		seq}
}

// NewTxOutput returns a TxOutput instance
func NewTxOutput(amount uint64, index int32) *TxOutput {
	return &TxOutput{
		&iproto.TxOutputPb{amount, 0, nil},
		index}
}

// NewTx returns a Tx instance
func NewTx(in []*TxInput, out []*TxOutput, lockTime uint32) *Tx {
	return &Tx{
		Version:  common.ProtocolVersion,
		LockTime: lockTime,

		// used by utxo-based model
		TxIn:  in,
		TxOut: out,

		// used by state-based model
		Nonce:     0,
		Recipient: &iotxaddress.Address{},
		Payload:   []byte{},
	}
}

// NewCoinbaseTx creates the coinbase transaction - a special type of transaction that does not require previously outputs.
func NewCoinbaseTx(toaddr string, amount uint64, data string) *Tx {
	if data == "" {
		randData := make([]byte, 20)
		_, err := rand.Read(randData)
		if err != nil {
			glog.Error(err)
			return nil
		}

		data = fmt.Sprintf("%x", randData)
	}

	txin := NewTxInput(common.ZeroHash32B, -1, []byte(data), 0xffffffff)
	txout := CreateTxOutput(toaddr, amount)
	return NewTx([]*TxInput{txin}, []*TxOutput{txout}, 0)
}

// IsCoinbase checks if it is a coinbase transaction by checking if Vin is empty
func (tx *Tx) IsCoinbase() bool {
	return len(tx.TxIn) == 1 && len(tx.TxOut) == 1 && tx.TxIn[0].OutIndex == -1 && tx.TxIn[0].Sequence == 0xffffffff &&
		bytes.Compare(tx.TxIn[0].TxHash[:], common.ZeroHash32B[:]) == 0
}

// TotalSize returns the total size of this transaction
func (tx *Tx) TotalSize() uint32 {
	size := uint32(VersionSizeInBytes + LockTimeSizeInBytes + NonceSizeInBytes)

	// add transaction input size
	for _, in := range tx.TxIn {
		size += in.TotalSize()
	}

	// add transaction output size
	for _, out := range tx.TxOut {
		size += out.TotalSize()
	}

	// add receipt, amount and payload sizes
	if tx.Recipient != nil && len(tx.Recipient.RawAddress) > 0 {
		size += uint32(len(tx.Recipient.RawAddress))
	}
	if tx.Amount != nil && len(tx.Amount.Bytes()) > 0 {
		size += uint32(len(tx.Amount.Bytes()))
	}
	size += uint32(len(tx.Payload))
	size += uint32(len(tx.SenderPublicKey))
	size += uint32(len(tx.Signature))
	return size
}

// ByteStream returns a raw byte stream of trnx data
func (tx *Tx) ByteStream() []byte {
	stream := make([]byte, 4)
	common.MachineEndian.PutUint32(stream, tx.Version)

	temp := make([]byte, 4)
	common.MachineEndian.PutUint32(temp, tx.LockTime)
	stream = append(stream, temp...)

	// 1. used by utxo-based model
	for _, txIn := range tx.TxIn {
		stream = append(stream, txIn.ByteStream()...)
	}
	for _, txOut := range tx.TxOut {
		stream = append(stream, txOut.ByteStream()...)
	}

	// 2. used by state-based model
	temp = make([]byte, 8)
	common.MachineEndian.PutUint64(temp, tx.Nonce)
	stream = append(stream, temp...)
	if tx.Recipient != nil && len(tx.Recipient.RawAddress) > 0 {
		stream = append(stream, tx.Recipient.RawAddress...)
	}
	if tx.Amount != nil && len(tx.Amount.Bytes()) > 0 {
		stream = append(stream, tx.Amount.Bytes()...)
	}
	stream = append(stream, tx.Payload...)
	if tx.SenderPublicKey != nil && len(tx.SenderPublicKey) > 0 {
		stream = append(stream, tx.SenderPublicKey...)
	}
	if tx.Signature != nil && len(tx.Signature) > 0 {
		stream = append(stream, tx.Signature...)
	}
	return stream
}

// ConvertToTxPb creates a protobuf's Tx using type Tx
func (tx *Tx) ConvertToTxPb() *iproto.TxPb {
	pbOut := make([]*iproto.TxOutputPb, len(tx.TxOut))
	for i, out := range tx.TxOut {
		pbOut[i] = out.TxOutputPb
	}

	t := &iproto.TxPb{
		Version:  tx.Version,
		LockTime: tx.LockTime,

		// used by utxo-based model
		TxIn:  tx.TxIn,
		TxOut: pbOut,

		// used by state-based model
		Nonce:   tx.Nonce,
		Payload: tx.Payload,
	}

	if tx.Amount != nil && len(tx.Amount.Bytes()) > 0 {
		t.Amount = tx.Amount.Bytes()
	}
	if tx.Recipient != nil && len(tx.Recipient.RawAddress) > 0 {
		t.Recipient = []byte(tx.Recipient.RawAddress)
	}
	if tx.SenderPublicKey != nil && len(tx.SenderPublicKey) > 0 {
		t.SenderPubKey = tx.SenderPublicKey
	}
	if tx.Signature != nil && len(tx.Signature) > 0 {
		t.Signature = tx.Signature
	}
	return t
}

// Serialize returns a serialized byte stream for the Tx
func (tx *Tx) Serialize() ([]byte, error) {
	return proto.Marshal(tx.ConvertToTxPb())
}

// ConvertFromTxPb converts a protobuf's Tx back to type Tx
func (tx *Tx) ConvertFromTxPb(pbTx *iproto.TxPb) {
	// set trnx fields
	tx.Version = pbTx.GetVersion()
	tx.LockTime = pbTx.GetLockTime()

	// used by utxo-based model
	tx.TxIn = nil
	tx.TxIn = pbTx.TxIn
	tx.TxOut = nil
	tx.TxOut = make([]*TxOutput, len(pbTx.TxOut))
	for i, out := range pbTx.TxOut {
		tx.TxOut[i] = &TxOutput{out, int32(i)}
	}

	// used by state-based model
	tx.Nonce = pbTx.Nonce
	tx.Recipient = &iotxaddress.Address{RawAddress: string(pbTx.Recipient[:])}
	if len(pbTx.Amount) > 0 {
		tx.Amount.SetBytes(pbTx.Amount)
	}
	tx.Payload = pbTx.Payload
	tx.SenderPublicKey = pbTx.SenderPubKey
	tx.Signature = pbTx.Signature
}

// Deserialize parse the byte stream into the Tx
func (tx *Tx) Deserialize(buf []byte) error {
	pbTx := iproto.TxPb{}
	if err := proto.Unmarshal(buf, &pbTx); err != nil {
		panic(err)
	}

	tx.ConvertFromTxPb(&pbTx)
	return nil
}

// Hash returns the hash of the Tx
func (tx *Tx) Hash() common.Hash32B {
	hash := blake2b.Sum256(tx.ByteStream())
	return blake2b.Sum256(hash[:])
}

// ConvertToUtxoPb creates a protobuf's UTXO
func (tx *Tx) ConvertToUtxoPb() *iproto.UtxoMapPb {
	return nil
}

//======================================
// Below are transaction output functions
//======================================

// CreateTxOutput creates a new transaction output
func CreateTxOutput(toaddr string, value uint64) *TxOutput {
	out := NewTxOutput(value, 0)

	locks, err := txvm.PayToAddrScript(toaddr)
	if err != nil {
		glog.Error(err)
		return nil
	}
	out.LockScript = locks
	out.LockScriptSize = uint32(len(out.LockScript))

	return out
}

// IsLockedWithKey checks if the UTXO in output is locked with script
func (out *TxOutput) IsLockedWithKey(lockScript []byte) bool {
	if len(out.LockScript) < 23 {
		glog.Error("LockScript too short")
		return false
	}
	// TODO: avoid hard-coded extraction of public key hash
	return bytes.Compare(out.LockScript[3:23], lockScript) == 0
}

// TotalSize returns the total size of transaction output
func (out *TxOutput) TotalSize() uint32 {
	return ValueSizeInBytes + LockScriptSizeInBytes + uint32(out.LockScriptSize)
}

// ByteStream returns a raw byte stream of transaction output
func (out *TxOutput) ByteStream() []byte {
	stream := make([]byte, 8)
	common.MachineEndian.PutUint64(stream, out.Value)

	temp := make([]byte, 4)
	common.MachineEndian.PutUint32(temp, out.LockScriptSize)
	stream = append(stream, temp...)
	stream = append(stream, out.LockScript...)

	return stream
}
