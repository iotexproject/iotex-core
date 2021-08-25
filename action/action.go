// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"math"
	"math/big"

	"github.com/pkg/errors"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
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

type hasDestination interface {
	Destination() string
}

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
		return sealed, errors.Wrapf(ErrAction, "failed to sign action hash = %x", h)
	}
	sealed.signature = sig
	act.Action().SetEnvelopeContext(sealed)
	return sealed, nil
}

// Sign signs the action using sender's private key
func RLPSign(act Envelope, pvk crypto.PrivateKey) (SealedEnvelope, error) {
	sealed := SealedEnvelope{
		Envelope:     act,
		srcPubkey:    pvk.PublicKey(),
		encoding:     iotextypes.Encoding_ETHEREUM_RLP,
		evmNetworkID: config.EVMNetworkID(),
	}

	h, err := sealed.envelopeHash()
	if err != nil {
		return sealed, errors.Wrap(err, "failed to generate envelope hash")
	}

	sig := GetRLPSig(act.Action(), pvk)
	if err != nil {
		return sealed, errors.Wrapf(ErrAction, "failed to sign action hash = %x", h)
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

// Verify verifies the action using sender's public key
func Verify(sealed SealedEnvelope) error {
	if sealed.SrcPubkey() == nil {
		return errors.New("empty public key")
	}
	// Reject action with insufficient gas limit
	intrinsicGas, err := sealed.IntrinsicGas()
	if intrinsicGas > sealed.GasLimit() || err != nil {
		return errors.Wrap(ErrInsufficientBalanceForGas, "insufficient gas")
	}

	h, err := sealed.envelopeHash()
	if err != nil {
		return errors.Wrap(err, "failed to generate envelope hash")
	}
	if sealed.SrcPubkey().Verify(h[:], sealed.Signature()) {
		return nil
	}
	return errors.Wrapf(
		ErrAction,
		"failed to verify action hash = %x and signature = %x",
		h,
		sealed.Signature(),
	)
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

func calculateIntrinsicGas(baseIntrinsicGas uint64, payloadGas uint64, payloadSize uint64) (uint64, error) {
	if (math.MaxUint64-baseIntrinsicGas)/payloadGas < payloadSize {
		return 0, ErrOutOfGas
	}

	return payloadSize*payloadGas + baseIntrinsicGas, nil
}
