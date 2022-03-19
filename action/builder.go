// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/pkg/version"
)

// Builder is used to build an action.
type Builder struct {
	act AbstractAction
}

// SetVersion sets action's version.
func (b *Builder) SetVersion(v uint32) *Builder {
	b.act.version = v
	return b
}

// SetNonce sets action's nonce.
func (b *Builder) SetNonce(n uint64) *Builder {
	b.act.nonce = n
	return b
}

// SetSourcePublicKey sets action's source's public key.
func (b *Builder) SetSourcePublicKey(key crypto.PublicKey) *Builder {
	b.act.srcPubkey = key
	return b
}

// SetGasLimit sets action's gas limit.
func (b *Builder) SetGasLimit(l uint64) *Builder {
	b.act.gasLimit = l
	return b
}

// SetGasPrice sets action's gas price.
func (b *Builder) SetGasPrice(p *big.Int) *Builder {
	if p == nil {
		return b
	}
	b.act.gasPrice = &big.Int{}
	b.act.gasPrice.Set(p)
	return b
}

// SetGasPriceByBytes sets action's gas price from a byte slice source.
func (b *Builder) SetGasPriceByBytes(buf []byte) *Builder {
	if len(buf) == 0 {
		return b
	}
	b.act.gasPrice = &big.Int{}
	b.act.gasPrice.SetBytes(buf)
	return b
}

// Build builds a new action.
func (b *Builder) Build() AbstractAction {
	if b.act.gasPrice == nil {
		b.act.gasPrice = big.NewInt(0)
	}
	if b.act.version == 0 {
		b.act.version = version.ProtocolVersion
	}
	return b.act
}

// EnvelopeBuilder is the builder to build Envelope.
type EnvelopeBuilder struct {
	elp envelope
}

// SetVersion sets action's version.
func (b *EnvelopeBuilder) SetVersion(v uint32) *EnvelopeBuilder {
	b.elp.version = v
	return b
}

// SetNonce sets action's nonce.
func (b *EnvelopeBuilder) SetNonce(n uint64) *EnvelopeBuilder {
	b.elp.nonce = n
	return b
}

// SetGasLimit sets action's gas limit.
func (b *EnvelopeBuilder) SetGasLimit(l uint64) *EnvelopeBuilder {
	b.elp.gasLimit = l
	return b
}

// SetGasPrice sets action's gas price.
func (b *EnvelopeBuilder) SetGasPrice(p *big.Int) *EnvelopeBuilder {
	if p == nil {
		return b
	}
	b.elp.gasPrice = &big.Int{}
	b.elp.gasPrice.Set(p)
	return b
}

// SetGasPriceByBytes sets action's gas price from a byte slice source.
func (b *EnvelopeBuilder) SetGasPriceByBytes(buf []byte) *EnvelopeBuilder {
	if len(buf) == 0 {
		return b
	}
	b.elp.gasPrice = &big.Int{}
	b.elp.gasPrice.SetBytes(buf)
	return b
}

// SetAction sets the action payload for the Envelope Builder is building.
func (b *EnvelopeBuilder) SetAction(action actionPayload) *EnvelopeBuilder {
	b.elp.payload = action
	return b
}

// SetChainID sets action's chainID.
func (b *EnvelopeBuilder) SetChainID(chainID uint32) *EnvelopeBuilder {
	b.elp.chainID = chainID
	return b
}

// Build builds a new action.
func (b *EnvelopeBuilder) Build() Envelope {
	return b.build()
}

func (b *EnvelopeBuilder) build() Envelope {
	if b.elp.gasPrice == nil {
		b.elp.gasPrice = big.NewInt(0)
	}
	if b.elp.version == 0 {
		b.elp.version = version.ProtocolVersion
	}
	return &b.elp
}

// BuildTransfer loads transfer action into envelope
func (b *EnvelopeBuilder) BuildTransfer(tx *types.Transaction) (Envelope, error) {
	b.elp.nonce = tx.Nonce()
	b.elp.gasPrice = new(big.Int).Set(tx.GasPrice())
	b.elp.gasLimit = tx.Gas()
	tsf, err := NewTransfer(tx.Nonce(), tx.Value(), getRecipientAddr(tx.To()), tx.Data(), tx.Gas(), tx.GasPrice())
	if err != nil {
		return nil, err
	}
	b.elp.payload = tsf
	return b.build(), nil
}

func getRecipientAddr(addr *common.Address) string {
	if addr == nil {
		return ""
	}
	ioAddr, _ := address.FromBytes(addr.Bytes())
	return ioAddr.String()
}

// BuildExecution loads executino action into envelope
func (b *EnvelopeBuilder) BuildExecution(tx *types.Transaction) (Envelope, error) {
	b.elp.nonce = tx.Nonce()
	b.elp.gasPrice = new(big.Int).Set(tx.GasPrice())
	b.elp.gasLimit = tx.Gas()
	exec, err := NewExecution(getRecipientAddr(tx.To()), tx.Nonce(), tx.Value(), tx.Gas(), tx.GasPrice(), tx.Data())
	if err != nil {
		return nil, err
	}
	b.elp.payload = exec
	return b.build(), nil
}

// BuildStakingAction loads staking action into envelope from abi-encoded data
func (b *EnvelopeBuilder) BuildStakingAction(tx *types.Transaction) (Envelope, error) {
	b.elp.nonce = tx.Nonce()
	b.elp.gasPrice = new(big.Int).Set(tx.GasPrice())
	b.elp.gasLimit = tx.Gas()
	act, err := newStakingActionFromABIBinary(tx.Data())
	if err != nil {
		return nil, err
	}
	b.elp.payload = act
	return b.build(), nil
}

func newStakingActionFromABIBinary(data []byte) (actionPayload, error) {
	if len(data) <= 4 {
		return nil, ErrInvalidABI
	}
	if act, err := NewCreateStakeFromABIBinary(data); err == nil {
		return act, nil
	}
	if act, err := NewDepositToStakeFromABIBinary(data); err == nil {
		return act, nil
	}
	if act, err := NewChangeCandidateFromABIBinary(data); err == nil {
		return act, nil
	}
	if act, err := NewUnstakeFromABIBinary(data); err == nil {
		return act, nil
	}
	if act, err := NewWithdrawStakeFromABIBinary(data); err == nil {
		return act, nil
	}
	if act, err := NewRestakeFromABIBinary(data); err == nil {
		return act, nil
	}
	if act, err := NewTransferStakeFromABIBinary(data); err == nil {
		return act, nil
	}
	if act, err := NewCandidateRegisterFromABIBinary(data); err == nil {
		return act, nil
	}
	if act, err := NewCandidateUpdateFromABIBinary(data); err == nil {
		return act, nil
	}
	return nil, ErrInvalidABI
}
