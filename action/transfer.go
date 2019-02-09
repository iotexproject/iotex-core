// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"math"
	"math/big"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-core/proto"
)

const (
	// TransferPayloadGas represents the transfer payload gas per uint
	TransferPayloadGas = uint64(100)
	// TransferBaseIntrinsicGas represents the base intrinsic gas for transfer
	TransferBaseIntrinsicGas = uint64(10000)
)

// Transfer defines the struct of account-based transfer
type Transfer struct {
	AbstractAction

	amount  *big.Int
	payload []byte
}

// NewTransfer returns a Transfer instance
func NewTransfer(
	nonce uint64,
	amount *big.Int,
	recipient string,
	payload []byte,
	gasLimit uint64,
	gasPrice *big.Int,
) (*Transfer, error) {
	return &Transfer{
		AbstractAction: AbstractAction{
			version:  version.ProtocolVersion,
			nonce:    nonce,
			dstAddr:  recipient,
			gasLimit: gasLimit,
			gasPrice: gasPrice,
		},
		amount:  amount,
		payload: payload,
		// SenderPublicKey and Signature will be populated in Sign()
	}, nil
}

// Amount returns the amount
func (tsf *Transfer) Amount() *big.Int { return tsf.amount }

// Payload returns the payload bytes
func (tsf *Transfer) Payload() []byte { return tsf.payload }

// SenderPublicKey returns the sender public key. It's the wrapper of Action.SrcPubkey
func (tsf *Transfer) SenderPublicKey() keypair.PublicKey { return tsf.SrcPubkey() }

// Recipient returns the recipient address. It's the wrapper of Action.DstAddr
func (tsf *Transfer) Recipient() string { return tsf.DstAddr() }

// IsContract returns true for contract action
func (tsf *Transfer) IsContract() bool {
	return tsf.dstAddr == EmptyAddress
}

// TotalSize returns the total size of this Transfer
func (tsf *Transfer) TotalSize() uint32 {
	size := tsf.BasicActionSize()
	if tsf.amount != nil && len(tsf.amount.Bytes()) > 0 {
		size += uint32(len(tsf.amount.Bytes()))
	}

	return size + uint32(len(tsf.payload))
}

// ByteStream returns a raw byte stream of this Transfer
func (tsf *Transfer) ByteStream() []byte {
	return byteutil.Must(proto.Marshal(tsf.Proto()))
}

// Proto converts Transfer to protobuf's ActionPb
func (tsf *Transfer) Proto() *iproto.TransferPb {
	// used by account-based model
	act := &iproto.TransferPb{
		Recipient: tsf.dstAddr,
		Payload:   tsf.payload,
	}

	if tsf.amount != nil {
		act.Amount = tsf.amount.Bytes()
	}
	return act
}

// LoadProto converts a protobuf's ActionPb to Transfer
func (tsf *Transfer) LoadProto(pbAct *iproto.TransferPb) error {
	if pbAct == nil {
		return errors.New("empty action proto to load")
	}
	if tsf == nil {
		return errors.New("nil action to load proto")
	}
	*tsf = Transfer{}

	tsf.amount = big.NewInt(0)
	tsf.amount.SetBytes(pbAct.GetAmount())
	tsf.payload = pbAct.Payload
	return nil
}

// IntrinsicGas returns the intrinsic gas of a transfer
func (tsf *Transfer) IntrinsicGas() (uint64, error) {
	payloadSize := uint64(len(tsf.Payload()))
	if (math.MaxUint64-TransferBaseIntrinsicGas)/TransferPayloadGas < payloadSize {
		return 0, ErrOutOfGas
	}

	return payloadSize*TransferPayloadGas + TransferBaseIntrinsicGas, nil
}

// Cost returns the total cost of a transfer
func (tsf *Transfer) Cost() (*big.Int, error) {
	intrinsicGas, err := tsf.IntrinsicGas()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get intrinsic gas for the transfer")
	}
	transferFee := big.NewInt(0).Mul(tsf.GasPrice(), big.NewInt(0).SetUint64(intrinsicGas))
	return big.NewInt(0).Add(tsf.Amount(), transferFee), nil
}
