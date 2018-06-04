// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"golang.org/x/crypto/blake2b"

	"github.com/iotexproject/iotex-core/blockchain/trx"
	"github.com/iotexproject/iotex-core/common"
	cp "github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/proto"
)

var (
	// ErrActionError indicates error for Action
	ErrActionError = errors.New("Action error")
)

type (
	// Transfer defines the struct of account-based transfer
	Transfer struct {
		Version  uint32
		LockTime uint32 // transaction to be locked until this time

		// used by account-based model
		Nonce           uint64
		Amount          *big.Int
		Sender          string
		Recipient       string
		Payload         []byte
		SenderPublicKey []byte
		Signature       []byte
	}
)

// NewTransfer returns a Transfer instance
func NewTransfer(lockTime uint32, nonce uint64, amount *big.Int, sender string, recipient string, senderPubKey []byte) *Transfer {
	return &Transfer{
		Version:  common.ProtocolVersion,
		LockTime: lockTime,

		// used by account-based model
		Nonce:           nonce,
		Amount:          amount,
		Sender:          sender,
		Recipient:       recipient,
		SenderPublicKey: senderPubKey,
		Payload:         []byte{},
	}
}

// TotalSize returns the total size of this Transfer
func (tsf *Transfer) TotalSize() uint32 {
	size := trx.VersionSizeInBytes + trx.LockTimeSizeInBytes
	// add nonce, amount, sender, receipt, and payload sizes
	size += NonceSizeInBytes
	if tsf.Amount != nil && len(tsf.Amount.Bytes()) > 0 {
		size += len(tsf.Amount.Bytes())
	}
	size += len(tsf.Sender)
	size += len(tsf.Recipient)
	size += len(tsf.Payload)
	size += len(tsf.SenderPublicKey)
	size += len(tsf.Signature)
	return uint32(size)
}

// ByteStream returns a raw byte stream of this Transfer
func (tsf *Transfer) ByteStream() []byte {
	stream := make([]byte, 4)
	common.MachineEndian.PutUint32(stream, tsf.Version)

	temp := make([]byte, 4)
	common.MachineEndian.PutUint32(temp, tsf.LockTime)
	stream = append(stream, temp...)

	// 2. used by account-based model
	temp = nil
	temp = make([]byte, 8)
	common.MachineEndian.PutUint64(temp, tsf.Nonce)
	stream = append(stream, temp...)
	if tsf.Amount != nil && len(tsf.Amount.Bytes()) > 0 {
		stream = append(stream, tsf.Amount.Bytes()...)
	}
	stream = append(stream, tsf.Sender...)
	stream = append(stream, tsf.Recipient...)
	stream = append(stream, tsf.Payload...)
	stream = append(stream, tsf.SenderPublicKey...)
	// Signature = Sign(hash(ByteStream())), so not included
	return stream
}

// ConvertToTransferPb converts Transfer to protobuf's TransferPb
func (tsf *Transfer) ConvertToTransferPb() *iproto.TransferPb {
	// used by account-based model
	t := &iproto.TransferPb{
		Version:      tsf.Version,
		LockTime:     tsf.LockTime,
		Nonce:        tsf.Nonce,
		Sender:       tsf.Sender,
		Recipient:    tsf.Recipient,
		Payload:      tsf.Payload,
		SenderPubKey: tsf.SenderPublicKey,
		Signature:    tsf.Signature,
	}

	if tsf.Amount != nil && len(tsf.Amount.Bytes()) > 0 {
		t.Amount = tsf.Amount.Bytes()
	}
	return t
}

// Serialize returns a serialized byte stream for the Transfer
func (tsf *Transfer) Serialize() ([]byte, error) {
	return proto.Marshal(tsf.ConvertToTransferPb())
}

// ConvertFromTransferPb converts a protobuf's TransferPb to Transfer
func (tsf *Transfer) ConvertFromTransferPb(pbTx *iproto.TransferPb) {
	// set trnx fields
	tsf.Version = pbTx.GetVersion()
	tsf.LockTime = pbTx.GetLockTime()
	// used by account-based model
	tsf.Nonce = pbTx.Nonce
	if tsf.Amount == nil {
		tsf.Amount = big.NewInt(0)
	}
	if len(pbTx.Amount) > 0 {
		tsf.Amount.SetBytes(pbTx.Amount)
	}
	tsf.Sender = ""
	if len(pbTx.Sender) > 0 {
		tsf.Recipient = string(pbTx.Recipient)
	}
	tsf.Recipient = ""
	if len(pbTx.Recipient) > 0 {
		tsf.Recipient = string(pbTx.Recipient)
	}
	tsf.Payload = nil
	tsf.Payload = pbTx.Payload
	tsf.SenderPublicKey = nil
	tsf.SenderPublicKey = pbTx.SenderPubKey
	tsf.Signature = nil
	tsf.Signature = pbTx.Signature
}

// Deserialize parse the byte stream into Transfer
func (tsf *Transfer) Deserialize(buf []byte) error {
	pbTransfer := &iproto.TransferPb{}
	if err := proto.Unmarshal(buf, pbTransfer); err != nil {
		return err
	}
	tsf.ConvertFromTransferPb(pbTransfer)
	return nil
}

// Hash returns the hash of the Transfer
func (tsf *Transfer) Hash() common.Hash32B {
	hash := blake2b.Sum256(tsf.ByteStream())
	return blake2b.Sum256(hash[:])
}

// Sign signs the Transfer using sender's private key
func Sign(raw []byte, sender *iotxaddress.Address) ([]byte, error) {
	tsf := &Transfer{}
	if err := tsf.Deserialize(raw); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal Transfer")
	}
	if err := tsf.sign(sender); err != nil {
		return nil, err
	}
	return tsf.Serialize()
}

// Verify verifies the Transfer using sender's public key
func (tsf *Transfer) Verify(sender *iotxaddress.Address) error {
	hash := tsf.Hash()
	if success := cp.Verify(sender.PublicKey, hash[:], tsf.Signature); success {
		return nil
	}
	return errors.Wrapf(ErrActionError, "Failed to verify Transfer signature = %x", tsf.Signature)
}

//======================================
// private functions
//======================================

func (tsf *Transfer) sign(sender *iotxaddress.Address) error {
	hash := tsf.Hash()
	if tsf.Signature = cp.Sign(sender.PrivateKey, hash[:]); tsf.Signature != nil {
		return nil
	}
	return errors.Wrapf(ErrActionError, "Failed to sign Transfer hash = %x", hash)
}
