// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimesd. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"
	"reflect"

	"github.com/pkg/errors"
	"golang.org/x/crypto/blake2b"

	"github.com/iotexproject/iotex-core/pkg/enc"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-core/proto"
)

const (
	// SettleDepositIntrinsicGas represents the intrinsic gas for the deposit action
	SettleDepositIntrinsicGas = uint64(10000)
)

// SettleDeposit represents the action to settle a deposit on the sub-chain
type SettleDeposit struct {
	AbstractAction
	amount *big.Int
	index  uint64
}

// NewSettleDeposit instantiates a deposit settlement to sub-chain action struct
func NewSettleDeposit(
	nonce uint64,
	amount *big.Int,
	index uint64,
	sender string,
	recipient string,
	gasLimit uint64,
	gasPrice *big.Int,
) *SettleDeposit {
	return &SettleDeposit{
		AbstractAction: AbstractAction{
			version:  version.ProtocolVersion,
			nonce:    nonce,
			srcAddr:  sender,
			dstAddr:  recipient,
			gasLimit: gasLimit,
			gasPrice: gasPrice,
		},
		amount: amount,
		index:  index,
	}
}

// Amount returns the amount
func (sd *SettleDeposit) Amount() *big.Int { return sd.amount }

// Index returns the index of the deposit on main-chain's sub-chain account
func (sd *SettleDeposit) Index() uint64 { return sd.index }

// Sender returns the sender address. It's the wrapper of Action.SrcAddr
func (sd *SettleDeposit) Sender() string { return sd.SrcAddr() }

// SenderPublicKey returns the sender public key. It's the wrapper of Action.SrcPubkey
func (sd *SettleDeposit) SenderPublicKey() keypair.PublicKey { return sd.SrcPubkey() }

// SetSenderPublicKey sets the sender public key. It's the wrapper of Action.SetSrcPubkey
func (sd *SettleDeposit) SetSenderPublicKey(pubkey keypair.PublicKey) { sd.SetSrcPubkey(pubkey) }

// Recipient returns the recipient address. It's the wrapper of Action.DstAddr. The recipient should be an address on
// the sub-chain
func (sd *SettleDeposit) Recipient() string { return sd.DstAddr() }

// ByteStream returns a raw byte stream of the settle deposit action
func (sd *SettleDeposit) ByteStream() []byte {
	stream := []byte(reflect.TypeOf(sd).String())
	stream = append(stream, sd.BasicActionByteStream()...)
	if sd.amount != nil && len(sd.amount.Bytes()) > 0 {
		stream = append(stream, sd.amount.Bytes()...)
	}
	temp := make([]byte, 8)
	enc.MachineEndian.PutUint64(temp, sd.index)
	stream = append(stream, temp...)
	return stream
}

// Proto converts SettleDeposit to protobuf's ActionPb
func (sd *SettleDeposit) Proto() *iproto.ActionPb {
	act := &iproto.ActionPb{
		Action: &iproto.ActionPb_SettleDeposit{
			SettleDeposit: &iproto.SettleDepositPb{
				Recipient: sd.dstAddr,
				Index:     sd.index,
			},
		},
		Version:      sd.version,
		Sender:       sd.srcAddr,
		SenderPubKey: sd.srcPubkey[:],
		Nonce:        sd.nonce,
		GasLimit:     sd.gasLimit,
		Signature:    sd.signature,
	}
	if sd.amount != nil && len(sd.amount.Bytes()) > 0 {
		act.GetSettleDeposit().Amount = sd.amount.Bytes()
	}
	if sd.gasPrice != nil && len(sd.gasPrice.Bytes()) > 0 {
		act.GasPrice = sd.gasPrice.Bytes()
	}
	return act
}

// LoadProto converts a protobuf's ActionPb to SettleDeposit
func (sd *SettleDeposit) LoadProto(pbAct *iproto.ActionPb) {
	sd.version = pbAct.Version
	sd.nonce = pbAct.Nonce
	sd.srcAddr = pbAct.Sender
	copy(sd.srcPubkey[:], pbAct.SenderPubKey)
	sd.gasLimit = pbAct.GasLimit
	if sd.gasPrice == nil {
		sd.gasPrice = big.NewInt(0)
	}
	if len(pbAct.GasPrice) > 0 {
		sd.gasPrice.SetBytes(pbAct.GasPrice)
	}
	if sd.amount == nil {
		sd.amount = big.NewInt(0)
	}
	sd.signature = pbAct.Signature
	pbDpst := pbAct.GetSettleDeposit()
	if len(pbDpst.Amount) > 0 {
		sd.amount.SetBytes(pbDpst.Amount)
	}
	sd.index = pbDpst.Index
	sd.dstAddr = pbDpst.Recipient
}

// Hash returns the hash of a deposit
func (sd *SettleDeposit) Hash() hash.Hash32B { return blake2b.Sum256(sd.ByteStream()) }

// IntrinsicGas returns the intrinsic gas of a settle deposit
func (sd *SettleDeposit) IntrinsicGas() (uint64, error) { return SettleDepositIntrinsicGas, nil }

// Cost returns the total cost of a settle deposit
func (sd *SettleDeposit) Cost() (*big.Int, error) {
	intrinsicGas, err := sd.IntrinsicGas()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get intrinsic gas for the settle deposit")
	}
	depositFee := big.NewInt(0).Mul(sd.GasPrice(), big.NewInt(0).SetUint64(intrinsicGas))
	return big.NewInt(0).Add(sd.Amount(), depositFee), nil
}
