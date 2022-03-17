// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"math"
	"math/big"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/pkg/errors"
)

// Action is the action can be Executed in protocols. The method is added to avoid mistakenly used empty interface as action.
type Action interface {
	SetEnvelopeContext(SealedEnvelope)
	SanityCheck() error
}

type actionPayload interface {
	Cost() (*big.Int, error)
	IntrinsicGas() (uint64, error)
	SetEnvelopeContext(SealedEnvelope)
	SanityCheck() error
}

// EthCompatibleAction is the action which is compatible to be converted to eth tx
type EthCompatibleAction interface {
	ToEthTx() (*types.Transaction, error)
}

type hasDestination interface {
	Destination() string
}

var (
	errInvalidABI = errors.New("invalid abi binary data")
)

// Sign signs the action using sender's private key
func Sign(act Envelope, sk crypto.PrivateKey) (SealedEnvelope, error) {
	sealed := SealedEnvelope{
		Envelope:  act,
		srcPubkey: sk.PublicKey(),
	}

	h, err := sealed.envelopeHash()
	if err != nil {
		return sealed, errors.Wrap(err, "failed to generate envelope hash")
	}
	sig, err := sk.Sign(h[:])
	if err != nil {
		return sealed, ErrInvalidSender
	}
	sealed.signature = sig
	act.Action().SetEnvelopeContext(sealed)
	return sealed, nil
}

// FakeSeal creates a SealedActionEnvelope without signature.
// This method should be only used in tests.
func FakeSeal(act Envelope, pubk crypto.PublicKey) SealedEnvelope {
	sealed := SealedEnvelope{
		Envelope:  act,
		srcPubkey: pubk,
	}
	act.Action().SetEnvelopeContext(sealed)
	return sealed
}

// AssembleSealedEnvelope assembles a SealedEnvelope use Envelope, Sender Address and Signature.
// This method should be only used in tests.
func AssembleSealedEnvelope(act Envelope, pk crypto.PublicKey, sig []byte) SealedEnvelope {
	sealed := SealedEnvelope{
		Envelope:  act,
		srcPubkey: pk,
		signature: sig,
	}
	act.Action().SetEnvelopeContext(sealed)
	return sealed
}

// ClassifyActions classfies actions
func ClassifyActions(actions []SealedEnvelope) ([]*Transfer, []*Execution) {
	tsfs := make([]*Transfer, 0)
	exes := make([]*Execution, 0)
	for _, elp := range actions {
		act := elp.Action()
		switch act := act.(type) {
		case *Transfer:
			tsfs = append(tsfs, act)
		case *Execution:
			exes = append(exes, act)
		}
	}
	return tsfs, exes
}

// CalculateIntrinsicGas returns the intrinsic gas of an action
func CalculateIntrinsicGas(baseIntrinsicGas uint64, payloadGas uint64, payloadSize uint64) (uint64, error) {
	if payloadGas == 0 && payloadSize == 0 {
		return baseIntrinsicGas, nil
	}
	if (math.MaxUint64-baseIntrinsicGas)/payloadGas < payloadSize {
		return 0, ErrInsufficientFunds
	}
	return payloadSize*payloadGas + baseIntrinsicGas, nil
}

// NewStakingActionFromABIBinary creates staking action from abi binary data
func NewStakingActionFromABIBinary(data []byte) (Action, error) {
	if len(data) <= 4 {
		return nil, errInvalidABI
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
	return nil, errInvalidABI
}
