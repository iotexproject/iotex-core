// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"bytes"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/pkg/version"
)

var (
	_stakingProtocolEthAddr   = common.BytesToAddress(address.StakingProtocolAddrHash[:])
	_rewardingProtocolEthAddr = common.BytesToAddress(address.RewardingProtocolAddrHash[:])
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
// TODO: change envelope to *envelope
type EnvelopeBuilder struct {
	elp envelope
	ab  AbstractAction
}

func (b *EnvelopeBuilder) SetTxType(v uint32) *EnvelopeBuilder {
	b.ab.txType = v
	return b
}

// SetVersion sets action's version.
func (b *EnvelopeBuilder) SetVersion(v uint32) *EnvelopeBuilder {
	b.ab.version = v
	return b
}

// SetNonce sets action's nonce.
func (b *EnvelopeBuilder) SetNonce(n uint64) *EnvelopeBuilder {
	b.ab.nonce = n
	return b
}

// SetGasLimit sets action's gas limit.
func (b *EnvelopeBuilder) SetGasLimit(l uint64) *EnvelopeBuilder {
	b.ab.gasLimit = l
	return b
}

// SetGasPrice sets action's gas price.
func (b *EnvelopeBuilder) SetGasPrice(p *big.Int) *EnvelopeBuilder {
	if p == nil {
		return b
	}
	b.ab.gasPrice = &big.Int{}
	b.ab.gasPrice.Set(p)
	return b
}

// SetGasPriceByBytes sets action's gas price from a byte slice source.
func (b *EnvelopeBuilder) SetGasPriceByBytes(buf []byte) *EnvelopeBuilder {
	if len(buf) == 0 {
		return b
	}
	b.ab.gasPrice = &big.Int{}
	b.ab.gasPrice.SetBytes(buf)
	return b
}

// SetAction sets the action payload for the Envelope Builder is building.
func (b *EnvelopeBuilder) SetAction(action actionPayload) *EnvelopeBuilder {
	b.elp.payload = action
	return b
}

// SetChainID sets action's chainID.
func (b *EnvelopeBuilder) SetChainID(chainID uint32) *EnvelopeBuilder {
	b.ab.chainID = chainID
	return b
}

func (b *EnvelopeBuilder) SetAccessList(acl types.AccessList) *EnvelopeBuilder {
	b.ab.accessList = acl
	return b
}

func (b *EnvelopeBuilder) SetDynamicGas(feeCap, tipCap *big.Int) *EnvelopeBuilder {
	b.ab.gasFeeCap = feeCap
	b.ab.gasTipCap = tipCap
	return b
}

func (b *EnvelopeBuilder) SetBlobTxData(
	feeCap *uint256.Int, hashes []common.Hash, sc *types.BlobTxSidecar) *EnvelopeBuilder {
	b.ab.blobData = &BlobTxData{
		blobFeeCap: feeCap,
		blobHashes: hashes,
		sidecar:    sc,
	}
	return b
}

// Build builds a new action.
func (b *EnvelopeBuilder) Build() Envelope {
	return b.build()
}

func (b *EnvelopeBuilder) build() Envelope {
	if b.ab.gasPrice == nil {
		b.ab.gasPrice = big.NewInt(0)
	}
	if b.ab.version == 0 {
		// default to version = 1
		b.ab.version = version.ProtocolVersion
	}
	if err := b.ab.validateTx(); err != nil {
		panic(err.Error())
	}
	if b.elp.payload == nil {
		panic("cannot build Envelope w/o a valid payload")
	}
	b.elp.common = b.ab.convertToTx()
	return &b.elp
}

// BuildTransfer loads transfer action into envelope
func (b *EnvelopeBuilder) BuildTransfer(tx *types.Transaction) (Envelope, error) {
	if tx.To() == nil {
		return nil, ErrInvalidAct
	}
	if err := b.setEnvelopeCommonFields(tx); err != nil {
		return nil, err
	}
	b.elp.payload = NewTransfer(tx.Value(), getRecipientAddr(tx.To()), tx.Data())
	return b.build(), nil
}

func (b *EnvelopeBuilder) setEnvelopeCommonFields(tx *types.Transaction) error {
	v, err := convertEthTxType(tx.Type())
	if err != nil {
		return err
	}
	b.ab.txType = uint32(v)
	b.ab.nonce = tx.Nonce()
	b.ab.gasPrice = tx.GasPrice()
	b.ab.gasLimit = tx.Gas()
	if acl := tx.AccessList(); len(acl) > 0 {
		b.ab.accessList = tx.AccessList()
	}
	b.ab.gasFeeCap = tx.GasFeeCap()
	b.ab.gasTipCap = tx.GasTipCap()
	if hashes := tx.BlobHashes(); len(hashes) > 0 {
		b.ab.blobData = &BlobTxData{
			blobFeeCap: uint256.MustFromBig(tx.BlobGasFeeCap()),
			blobHashes: hashes,
			sidecar:    tx.BlobTxSidecar(),
		}
	}
	return nil
}

func convertEthTxType(typ uint8) (int, error) {
	switch typ {
	case types.LegacyTxType:
		return LegacyTxType, nil
	case types.AccessListTxType:
		return AccessListTxType, nil
	case types.DynamicFeeTxType:
		return DynamicFeeTxType, nil
	case types.BlobTxType:
		return BlobTxType, nil
	default:
		return 0, errors.Wrapf(ErrInvalidAct, "unsupported eth tx type %d", typ)
	}
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
	if err := b.setEnvelopeCommonFields(tx); err != nil {
		return nil, err
	}
	b.elp.payload = NewExecution(getRecipientAddr(tx.To()), tx.Value(), tx.Data())
	return b.build(), nil
}

// BuildStakingAction loads staking action into envelope from abi-encoded data
func (b *EnvelopeBuilder) BuildStakingAction(tx *types.Transaction) (Envelope, error) {
	if !bytes.Equal(tx.To().Bytes(), _stakingProtocolEthAddr.Bytes()) {
		return nil, ErrInvalidAct
	}
	if err := b.setEnvelopeCommonFields(tx); err != nil {
		return nil, err
	}
	act, err := newStakingActionFromABIBinary(tx.Data())
	if err != nil {
		return nil, err
	}
	b.elp.payload = act
	return b.build(), nil
}

// BuildRewardingAction loads rewarding action into envelope from abi-encoded data
func (b *EnvelopeBuilder) BuildRewardingAction(tx *types.Transaction) (Envelope, error) {
	if !bytes.Equal(tx.To().Bytes(), _rewardingProtocolEthAddr.Bytes()) {
		return nil, ErrInvalidAct
	}
	if err := b.setEnvelopeCommonFields(tx); err != nil {
		return nil, err
	}
	act, err := newRewardingActionFromABIBinary(tx.Data())
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
	if act, err := NewCandidateActivateFromABIBinary(data); err == nil {
		return act, nil
	}
	if act, err := NewCandidateEndorsementFromABIBinary(data); err == nil {
		return act, nil
	}
	if act, err := NewCandidateTransferOwnershipFromABIBinary(data); err == nil {
		return act, nil
	}
	if act, err := NewMigrateStakeFromABIBinary(data); err == nil {
		return act, nil
	}
	return nil, ErrInvalidABI
}

func newRewardingActionFromABIBinary(data []byte) (actionPayload, error) {
	if len(data) <= 4 {
		return nil, ErrInvalidABI
	}
	if act, err := NewClaimFromRewardingFundFromABIBinary(data); err == nil {
		return act, nil
	}
	if act, err := NewDepositToRewardingFundFromABIBinary(data); err == nil {
		return act, nil
	}
	return nil, ErrInvalidABI
}
