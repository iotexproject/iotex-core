// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"
	"reflect"

	"github.com/pkg/errors"
	"golang.org/x/crypto/blake2b"

	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-core/proto"
)

const (
	// DepositIntrinsicGas represents the intrinsic gas for the deposit action
	DepositIntrinsicGas = uint64(10000)
)

// CreateDeposit represents the action to deposit the token from main-chain to sub-chain. The recipient address must be a
// sub-chain address, but it doesn't need to be owned by the sender.
type CreateDeposit struct {
	AbstractAction
	amount *big.Int
}

// NewCreateDeposit instantiates a deposit to sub-chain action struct
func NewCreateDeposit(
	nonce uint64,
	amount *big.Int,
	sender string,
	recipient string,
	gasLimit uint64,
	gasPrice *big.Int,
) *CreateDeposit {
	return &CreateDeposit{
		AbstractAction: AbstractAction{
			version:  version.ProtocolVersion,
			nonce:    nonce,
			srcAddr:  sender,
			dstAddr:  recipient,
			gasLimit: gasLimit,
			gasPrice: gasPrice,
		},
		amount: amount,
	}
}

// Amount returns the amount
func (d *CreateDeposit) Amount() *big.Int { return d.amount }

// Sender returns the sender address. It's the wrapper of Action.SrcAddr
func (d *CreateDeposit) Sender() string { return d.SrcAddr() }

// SenderPublicKey returns the sender public key. It's the wrapper of Action.SrcPubkey
func (d *CreateDeposit) SenderPublicKey() keypair.PublicKey { return d.SrcPubkey() }

// SetSenderPublicKey sets the sender public key. It's the wrapper of Action.SetSrcPubkey
func (d *CreateDeposit) SetSenderPublicKey(pubkey keypair.PublicKey) { d.SetSrcPubkey(pubkey) }

// Recipient returns the recipient address. It's the wrapper of Action.DstAddr. The recipient should be an address on
// the sub-chain
func (d *CreateDeposit) Recipient() string { return d.DstAddr() }

// ByteStream returns a raw byte stream of the deposit action
func (d *CreateDeposit) ByteStream() []byte {
	stream := []byte(reflect.TypeOf(d).String())
	stream = append(stream, d.BasicActionByteStream()...)
	if d.amount != nil && len(d.amount.Bytes()) > 0 {
		stream = append(stream, d.amount.Bytes()...)
	}
	return stream
}

// Proto converts CreateDeposit to protobuf's ActionPb
func (d *CreateDeposit) Proto() *iproto.ActionPb {
	act := &iproto.ActionPb{
		Action: &iproto.ActionPb_CreateDeposit{
			CreateDeposit: &iproto.CreateDepositPb{
				Recipient: d.dstAddr,
			},
		},
		Version:      d.version,
		Sender:       d.srcAddr,
		SenderPubKey: d.srcPubkey[:],
		Nonce:        d.nonce,
		GasLimit:     d.gasLimit,
		Signature:    d.signature,
	}
	if d.amount != nil && len(d.amount.Bytes()) > 0 {
		act.GetCreateDeposit().Amount = d.amount.Bytes()
	}
	if d.gasPrice != nil && len(d.gasPrice.Bytes()) > 0 {
		act.GasPrice = d.gasPrice.Bytes()
	}
	return act
}

// LoadProto converts a protobuf's ActionPb to CreateDeposit
func (d *CreateDeposit) LoadProto(pbAct *iproto.ActionPb) {
	d.version = pbAct.Version
	d.nonce = pbAct.Nonce
	d.srcAddr = pbAct.Sender
	copy(d.srcPubkey[:], pbAct.SenderPubKey)
	d.gasLimit = pbAct.GasLimit
	if d.gasPrice == nil {
		d.gasPrice = big.NewInt(0)
	}
	if len(pbAct.GasPrice) > 0 {
		d.gasPrice.SetBytes(pbAct.GasPrice)
	}
	if d.amount == nil {
		d.amount = big.NewInt(0)
	}
	d.signature = pbAct.Signature
	pbDpst := pbAct.GetCreateDeposit()
	if len(pbDpst.Amount) > 0 {
		d.amount.SetBytes(pbDpst.Amount)
	}
	d.dstAddr = pbDpst.Recipient
}

// Hash returns the hash of a deposit
func (d *CreateDeposit) Hash() hash.Hash32B { return blake2b.Sum256(d.ByteStream()) }

// IntrinsicGas returns the intrinsic gas of a deposit
func (d *CreateDeposit) IntrinsicGas() (uint64, error) { return DepositIntrinsicGas, nil }

// Cost returns the total cost of a deposit
func (d *CreateDeposit) Cost() (*big.Int, error) {
	intrinsicGas, err := d.IntrinsicGas()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get intrinsic gas for the deposit")
	}
	depositFee := big.NewInt(0).Mul(d.GasPrice(), big.NewInt(0).SetUint64(intrinsicGas))
	return big.NewInt(0).Add(d.Amount(), depositFee), nil
}
