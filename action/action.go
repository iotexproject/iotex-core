// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"context"
	"crypto/ecdsa"
	"math"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
)

const (
	AntiqueTxType    = 0 // when we first enable web3 tx format, version = 0 is used, all such tx are legacy tx
	LegacyTxType     = 1
	AccessListTxType = 2
	DynamicFeeTxType = 3
	BlobTxType       = 4
)

type (
	// Action is the action can be Executed in protocols. The method is added to avoid mistakenly used empty interface as action.
	Action interface {
		SanityCheck() error
	}

	// EthCompatibleAction is the action which is compatible to be converted to eth tx
	EthCompatibleAction interface {
		EthTo() (*common.Address, error)
		Value() *big.Int
		EthData() ([]byte, error)
	}

	TxContainer interface {
		Unfold(*SealedEnvelope, context.Context, func(context.Context, *common.Address) (bool, bool, bool, error)) error // unfold the tx inside the container
	}

	actionPayload interface {
		IntrinsicGas() (uint64, error)
		SanityCheck() error
		FillAction(*iotextypes.ActionCore)
	}

	hasDestination interface{ Destination() string }

	hasSize interface{ Size() uint32 }

	amountForCost interface{ Amount() *big.Int }

	gasLimitForCost interface{ GasLimitForCost() }

	validateSidecar interface{ ValidateSidecar() error }

	protoForRawHash interface{ ProtoForRawHash() *iotextypes.ActionCore }
)

// Sign signs the action using sender's private key
func Sign(act Envelope, sk crypto.PrivateKey) (*SealedEnvelope, error) {
	sealed := &SealedEnvelope{
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
	return sealed, nil
}

func EthSign(act Envelope, evmID uint32, sk crypto.PrivateKey) (*SealedEnvelope, error) {
	encoding := iotextypes.Encoding_ETHEREUM_EIP155
	signer, err := NewEthSigner(encoding, evmID)
	if err != nil {
		return nil, err
	}
	tx, err := act.ToEthTx(evmID, encoding)
	if err != nil {
		return nil, err
	}
	tx, err = types.SignTx(tx, signer, sk.EcdsaPrivateKey().(*ecdsa.PrivateKey))
	if err != nil {
		return nil, err
	}
	encoding, sig, pubkey, err := ExtractTypeSigPubkey(tx)
	if err != nil {
		return nil, err
	}
	req := &iotextypes.Action{
		Core:         act.Proto(),
		SenderPubKey: pubkey.Bytes(),
		Signature:    sig,
		Encoding:     encoding,
	}
	desr := Deserializer{}
	return desr.SetEvmNetworkID(evmID).ActionToSealedEnvelope(req)
}

// FakeSeal creates a SealedActionEnvelope without signature.
// This method should be only used in tests.
func FakeSeal(act Envelope, pubk crypto.PublicKey) *SealedEnvelope {
	sealed := &SealedEnvelope{
		Envelope:  act,
		srcPubkey: pubk,
	}
	return sealed
}

// AssembleSealedEnvelope assembles a SealedEnvelope use Envelope, Sender Address and Signature.
// This method should be only used in tests.
func AssembleSealedEnvelope(act Envelope, pk crypto.PublicKey, sig []byte) *SealedEnvelope {
	sealed := &SealedEnvelope{
		Envelope:  act,
		srcPubkey: pk,
		signature: sig,
	}
	return sealed
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

// IsSystemAction determine whether input action belongs to system action
func IsSystemAction(act *SealedEnvelope) bool {
	switch act.Action().(type) {
	case *GrantReward, *PutPollResult:
		return true
	default:
		return false
	}
}
