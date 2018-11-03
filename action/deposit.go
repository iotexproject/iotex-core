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

// Deposit represents the action to deposit the token from main-chain to sub-chain. The recipient address must be a
// sub-chain address, but it doesn't need to be owned by the sender.
type Deposit struct {
	AbstractAction
	amount *big.Int
}

// NewDeposit instantiates a deposit to sub-chain action struct
func NewDeposit(
	nonce uint64,
	amount *big.Int,
	sender string,
	recipient string,
	gasLimit uint64,
	gasPrice *big.Int,
) *Deposit {
	return &Deposit{
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
func (d *Deposit) Amount() *big.Int { return d.amount }

// Sender returns the sender address. It's the wrapper of Action.SrcAddr
func (d *Deposit) Sender() string { return d.SrcAddr() }

// SenderPublicKey returns the sender public key. It's the wrapper of Action.SrcPubkey
func (d *Deposit) SenderPublicKey() keypair.PublicKey { return d.SrcPubkey() }

// SetSenderPublicKey sets the sender public key. It's the wrapper of Action.SetSrcPubkey
func (d *Deposit) SetSenderPublicKey(pubkey keypair.PublicKey) { d.SetSrcPubkey(pubkey) }

// Recipient returns the recipient address. It's the wrapper of Action.DstAddr. The recipient should be an address on
// the sub-chain
func (d *Deposit) Recipient() string { return d.DstAddr() }

// ByteStream returns a raw byte stream of the deposit action
func (d *Deposit) ByteStream() []byte {
	stream := []byte(reflect.TypeOf(d).String())
	stream = append(stream, d.BasicActionByteStream()...)
	if d.amount != nil && len(d.amount.Bytes()) > 0 {
		stream = append(stream, d.amount.Bytes()...)
	}
	return stream
}

// Proto converts Deposit to protobuf's ActionPb
func (d *Deposit) Proto() *iproto.ActionPb {
	act := &iproto.ActionPb{
		Action: &iproto.ActionPb_Deposit{
			Deposit: &iproto.DepositPb{
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
		act.GetDeposit().Amount = d.amount.Bytes()
	}
	if d.gasPrice != nil && len(d.gasPrice.Bytes()) > 0 {
		act.GasPrice = d.gasPrice.Bytes()
	}
	return act
}

// LoadProto converts a protobuf's ActionPb to Deposit
func (d *Deposit) LoadProto(pbAct *iproto.ActionPb) {
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
	pbDpst := pbAct.GetDeposit()
	if len(pbDpst.Amount) > 0 {
		d.amount.SetBytes(pbDpst.Amount)
	}
	d.dstAddr = pbDpst.Recipient
}

// Hash returns the hash of a deposit
func (d *Deposit) Hash() hash.Hash32B { return blake2b.Sum256(d.ByteStream()) }

// IntrinsicGas returns the intrinsic gas of a deposit
func (d *Deposit) IntrinsicGas() (uint64, error) { return DepositIntrinsicGas, nil }

// Cost returns the total cost of a deposit
func (d *Deposit) Cost() (*big.Int, error) {
	intrinsicGas, err := d.IntrinsicGas()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get intrinsic gas for the deposit")
	}
	depositFee := big.NewInt(0).Mul(d.GasPrice(), big.NewInt(0).SetUint64(intrinsicGas))
	return big.NewInt(0).Add(d.Amount(), depositFee), nil
}
