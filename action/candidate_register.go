// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"

	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

const (
	// CandidateRegisterPayloadGas represents the CandidateRegister payload gas per uint
	CandidateRegisterPayloadGas = uint64(100)
	// CandidateRegisterBaseIntrinsicGas represents the base intrinsic gas for CandidateRegister
	CandidateRegisterBaseIntrinsicGas = uint64(10000)
)

var (
	// ErrInvalidAmount represents that amount is 0 or negative
	ErrInvalidAmount = errors.New("invalid amount")
)

// CandidateRegister is the action to register a candidate
type CandidateRegister struct {
	AbstractAction

	name            string
	operatorAddress address.Address
	rewardAddress   address.Address
	ownerAddress    address.Address
	amount          *big.Int
	duration        uint32
	autoStake       bool
	payload         []byte
}

// NewCandidateRegister creates a CandidateRegister instance
func NewCandidateRegister(
	nonce uint64,
	name, operatorAddrStr, rewardAddrStr, ownerAddrStr, amountStr string,
	duration uint32,
	autoStake bool,
	payload []byte,
	gasLimit uint64,
	gasPrice *big.Int,
) (*CandidateRegister, error) {
	operatorAddr, err := address.FromString(operatorAddrStr)
	if err != nil {
		return nil, err
	}

	rewardAddress, err := address.FromString(rewardAddrStr)
	if err != nil {
		return nil, err
	}

	amount, ok := new(big.Int).SetString(amountStr, 10)
	if !ok {
		return nil, errors.Wrapf(ErrInvalidAmount, "amount %s", amount)
	}

	cr := &CandidateRegister{
		AbstractAction: AbstractAction{
			version:  version.ProtocolVersion,
			nonce:    nonce,
			gasLimit: gasLimit,
			gasPrice: gasPrice,
		},
		name:            name,
		operatorAddress: operatorAddr,
		rewardAddress:   rewardAddress,
		amount:          amount,
		duration:        duration,
		autoStake:       autoStake,
		payload:         payload,
	}

	if len(ownerAddrStr) > 0 {
		ownerAddress, err := address.FromString(ownerAddrStr)
		if err != nil {
			return nil, err
		}
		cr.ownerAddress = ownerAddress
	}
	return cr, nil
}

// Amount returns the amount
func (cr *CandidateRegister) Amount() *big.Int { return cr.amount }

// Payload returns the payload bytes
func (cr *CandidateRegister) Payload() []byte { return cr.payload }

// Duration returns the self-stake duration
func (cr *CandidateRegister) Duration() uint32 { return cr.duration }

// AutoStake returns the if staking is auth stake
func (cr *CandidateRegister) AutoStake() bool { return cr.autoStake }

// Name returns candidate name to register
func (cr *CandidateRegister) Name() string { return cr.name }

// OperatorAddress returns candidate operatorAddress to register
func (cr *CandidateRegister) OperatorAddress() address.Address { return cr.operatorAddress }

// RewardAddress returns candidate rewardAddress to register
func (cr *CandidateRegister) RewardAddress() address.Address { return cr.rewardAddress }

// OwnerAddress returns candidate ownerAddress to register
func (cr *CandidateRegister) OwnerAddress() address.Address { return cr.ownerAddress }

// Serialize returns a raw byte stream of the CandidateRegister struct
func (cr *CandidateRegister) Serialize() []byte {
	return byteutil.Must(proto.Marshal(cr.Proto()))
}

// Proto converts to protobuf CandidateRegister Action
func (cr *CandidateRegister) Proto() *iotextypes.CandidateRegister {
	act := iotextypes.CandidateRegister{
		Candidate: &iotextypes.CandidateBasicInfo{
			Name:            cr.name,
			OperatorAddress: cr.operatorAddress.String(),
			RewardAddress:   cr.rewardAddress.String(),
		},
		StakedDuration: cr.duration,
		AutoStake:      cr.autoStake,
	}

	if cr.amount != nil {
		act.StakedAmount = cr.amount.String()
	}

	if cr.ownerAddress != nil {
		act.OwnerAddress = cr.ownerAddress.String()
	}

	if len(cr.payload) > 0 {
		act.Payload = make([]byte, len(cr.payload))
		copy(act.Payload, cr.payload)
	}
	return &act
}

// LoadProto converts a protobuf's Action to CandidateRegister
func (cr *CandidateRegister) LoadProto(pbAct *iotextypes.CandidateRegister) error {
	if pbAct == nil {
		return errors.New("empty action proto to load")
	}

	cInfo := pbAct.GetCandidate()
	cr.name = cInfo.GetName()

	operatorAddr, err := address.FromString(cInfo.GetOperatorAddress())
	if err != nil {
		return err
	}
	rewardAddr, err := address.FromString(cInfo.GetRewardAddress())
	if err != nil {
		return err
	}

	cr.operatorAddress = operatorAddr
	cr.rewardAddress = rewardAddr
	cr.duration = pbAct.GetStakedDuration()
	cr.autoStake = pbAct.GetAutoStake()

	if len(pbAct.GetStakedAmount()) > 0 {
		var ok bool
		if cr.amount, ok = new(big.Int).SetString(pbAct.GetStakedAmount(), 10); !ok {
			return errors.Errorf("invalid amount %s", pbAct.GetStakedAmount())
		}
	}

	cr.payload = nil
	if len(pbAct.GetPayload()) > 0 {
		cr.payload = make([]byte, len(pbAct.GetPayload()))
		copy(cr.payload, pbAct.GetPayload())
	}

	if len(pbAct.GetOwnerAddress()) > 0 {
		ownerAddr, err := address.FromString(pbAct.GetOwnerAddress())
		if err != nil {
			return err
		}
		cr.ownerAddress = ownerAddr
	}
	return nil
}

// IntrinsicGas returns the intrinsic gas of a CandidateRegister
func (cr *CandidateRegister) IntrinsicGas() (uint64, error) {
	payloadSize := uint64(len(cr.Payload()))
	return CalculateIntrinsicGas(CandidateRegisterBaseIntrinsicGas, CandidateRegisterPayloadGas, payloadSize)
}

// Cost returns the total cost of a CandidateRegister
func (cr *CandidateRegister) Cost() (*big.Int, error) {
	intrinsicGas, err := cr.IntrinsicGas()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get intrinsic gas for the CandidateRegister creates")
	}
	fee := big.NewInt(0).Mul(cr.GasPrice(), big.NewInt(0).SetUint64(intrinsicGas))
	return big.NewInt(0).Add(cr.Amount(), fee), nil
}

// SanityCheck validates the variables in the action
func (cr *CandidateRegister) SanityCheck() error {
	if cr.Amount().Sign() <= 0 {
		return errors.Wrap(ErrInvalidAmount, "negative value")
	}

	return cr.AbstractAction.SanityCheck()
}
