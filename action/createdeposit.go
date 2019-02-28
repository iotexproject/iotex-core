// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-core/protogen/iotextypes"
)

const (
	// CreateDepositIntrinsicGas represents the intrinsic gas for the deposit action
	CreateDepositIntrinsicGas = uint64(10000)
)

var _ hasDestination = (*CreateDeposit)(nil)

// CreateDeposit represents the action to deposit the token from main-chain to sub-chain. The recipient address must be a
// sub-chain address, but it doesn't need to be owned by the sender.
type CreateDeposit struct {
	AbstractAction

	chainID   uint32
	amount    *big.Int
	recipient string
}

// NewCreateDeposit instantiates a deposit creation to sub-chain action struct
func NewCreateDeposit(
	nonce uint64,
	chainID uint32,
	amount *big.Int,
	recipient string,
	gasLimit uint64,
	gasPrice *big.Int,
) *CreateDeposit {
	return &CreateDeposit{
		AbstractAction: AbstractAction{
			version:  version.ProtocolVersion,
			nonce:    nonce,
			gasLimit: gasLimit,
			gasPrice: gasPrice,
		},
		chainID:   chainID,
		amount:    amount,
		recipient: recipient,
	}
}

// ChainID returns chain ID
func (d *CreateDeposit) ChainID() uint32 { return d.chainID }

// Amount returns the amount
func (d *CreateDeposit) Amount() *big.Int { return d.amount }

// SenderPublicKey returns the sender public key. It's the wrapper of Action.SrcPubkey
func (d *CreateDeposit) SenderPublicKey() keypair.PublicKey { return d.SrcPubkey() }

// Recipient returns the recipient address. The recipient should be an address on the sub-chain
func (d *CreateDeposit) Recipient() string { return d.recipient }

// Destination returns the recipient address. The recipient should be an address on the sub-chain
func (d *CreateDeposit) Destination() string { return d.Recipient() }

// ByteStream returns a raw byte stream of the deposit action
func (d *CreateDeposit) ByteStream() []byte {
	return byteutil.Must(proto.Marshal(d.Proto()))
}

// Proto converts CreateDeposit to protobuf's ActionPb
func (d *CreateDeposit) Proto() *iotextypes.CreateDeposit {
	act := &iotextypes.CreateDeposit{
		ChainID:   d.chainID,
		Recipient: d.recipient,
	}
	if d.amount != nil && len(d.amount.String()) > 0 {
		act.Amount = d.amount.String()
	}
	return act
}

// LoadProto converts a protobuf's ActionPb to CreateDeposit
func (d *CreateDeposit) LoadProto(pbDpst *iotextypes.CreateDeposit) error {
	if d == nil {
		return errors.New("nil action to load proto")
	}
	*d = CreateDeposit{}

	if pbDpst == nil {
		return errors.New("empty action proto to load")
	}

	d.chainID = pbDpst.GetChainID()
	d.amount = big.NewInt(0)
	d.amount.SetString(pbDpst.GetAmount(), 10)
	d.recipient = pbDpst.GetRecipient()
	return nil
}

// IntrinsicGas returns the intrinsic gas of a create deposit
func (d *CreateDeposit) IntrinsicGas() (uint64, error) { return CreateDepositIntrinsicGas, nil }

// Cost returns the total cost of a create deposit
func (d *CreateDeposit) Cost() (*big.Int, error) {
	intrinsicGas, err := d.IntrinsicGas()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get intrinsic gas for the create deposit")
	}
	depositFee := big.NewInt(0).Mul(d.GasPrice(), big.NewInt(0).SetUint64(intrinsicGas))
	return big.NewInt(0).Add(d.Amount(), depositFee), nil
}
