// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"bytes"
	"context"
	"math"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
)

const (
	LegacyTxType     = 0
	AccessListTxType = 1
	DynamicFeeTxType = 2
	BlobTxType       = 3
	SetCodeTxType    = 4
)

type (
	// Action is the action that can be executed in protocols. The method is added to avoid mistakenly using empty interface as action.
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

// FakeSeal creates a SealedActionEnvelope without signature.
// This method should be only used in tests.
func FakeSeal(act Envelope, pubk crypto.PublicKey) *SealedEnvelope {
	sealed := &SealedEnvelope{
		Envelope:  act,
		srcPubkey: pubk,
	}
	return sealed
}

// AssembleSealedEnvelope assembles a SealedEnvelope using Envelope, Sender Address and Signature.
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

// FloorDataGas computes the minimum gas required for a transaction based on its data tokens (EIP-7623).
func FloorDataGas(data []byte) (uint64, error) {
	var (
		z      = uint64(bytes.Count(data, []byte{0}))
		nz     = uint64(len(data)) - z
		tokens = nz*TxTokenPerNonZeroByte + z
	)
	// Check for overflow
	if (math.MaxUint64-TxGas)/TxCostFloorPerToken < tokens {
		return 0, ErrGasUintOverflow
	}
	// Minimum gas required for a transaction based on its data tokens (EIP-7623).
	return TxGas + tokens*TxCostFloorPerToken, nil
}

// IsSystemAction determine whether input action belongs to system action
func IsSystemAction(act *SealedEnvelope) bool {
	switch act.Action().(type) {
	case *GrantReward, *PutPollResult, *ScheduleCandidateDeactivation:
		return true
	default:
		return false
	}
}

// CheckActionAddress checks the address fields of an action before it is
// accepted into the action pool.
//
// address.FromString decodes the special addresses (address.StakingBucketPoolAddr
// and address.RewardingPoolAddr) without an error, but they name protocol-internal
// pools, have no 20-byte form, and cannot be used as an account by an action, so
// every address field an action carries is rejected if it holds one of them. A
// Transfer recipient is also decoded here, as Transfer.SanityCheck does not.
//
// This is an admission check only. It is deliberately not part of SanityCheck,
// which also runs when validating blocks, so it does not change which blocks
// are valid.
func CheckActionAddress(act Action) error {
	switch act := act.(type) {
	case *Transfer:
		addr, err := address.FromString(act.recipient)
		if err != nil {
			return errors.Wrapf(err, "invalid address %s", act.recipient)
		}
		return checkNotSpecialAddress(addr, "recipient")
	case *Execution:
		if act.contract == EmptyAddress {
			return nil
		}
		if !address.IsAddrV1Special(act.contract) {
			// a malformed contract address is rejected by SanityCheck
			return nil
		}
		return errors.Wrapf(ErrAddress, "contract cannot be special address %s", act.contract)
	case *TransferStake:
		return checkNotSpecialAddress(act.voterAddress, "voter address")
	case *CandidateTransferOwnership:
		return checkNotSpecialAddress(act.newOwner, "new owner")
	case *CandidateRegister:
		if err := checkNotSpecialAddress(act.operatorAddress, "operator address"); err != nil {
			return err
		}
		if err := checkNotSpecialAddress(act.rewardAddress, "reward address"); err != nil {
			return err
		}
		return checkNotSpecialAddress(act.ownerAddress, "owner address")
	case *CandidateUpdate:
		if err := checkNotSpecialAddress(act.operatorAddress, "operator address"); err != nil {
			return err
		}
		return checkNotSpecialAddress(act.rewardAddress, "reward address")
	case *ClaimFromRewardingFund:
		return checkNotSpecialAddress(act.address, "claim address")
	}
	return nil
}

// checkNotSpecialAddress returns ErrAddress if addr is one of the special
// addresses. A nil addr is an absent optional field and passes.
func checkNotSpecialAddress(addr address.Address, field string) error {
	if _, ok := addr.(*address.AddrV1Special); ok {
		return errors.Wrapf(ErrAddress, "%s cannot be special address %s", field, addr.String())
	}
	return nil
}
