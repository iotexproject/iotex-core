// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
)

const (
	// ProposerRegisterPayloadGas is the gas to register a proposer
	ProposerRegisterPayloadGas = uint64(100)
	// ProposerRegisterBaseIntrinsicGas is the base intrinsic gas to register a proposer
	ProposerRegisterBaseIntrinsicGas = uint64(10000)
	// ProposerRegisterAmount is the amount to register a proposer
	ProposerRegisterAmount = uint64(10000)
)

// ProposerRegister is the action to register an execution node to produce block proposal
type ProposerRegister struct {
	AbstractAction

	operatorAddress address.Address
	rewardAddress   address.Address
	ownerAddress    address.Address
	duration        uint32
	autoStake       bool
	payload         []byte
}

// NewProposerRegister creates a ProposerRegister instance
func NewProposerRegister(
	nonce uint64,
	operatorAddrStr, rewardAddrStr, ownerAddrStr string,
	duration uint32,
	autoStake bool,
	payload []byte,
	gasLimit uint64,
	gasPrice *big.Int,
) (*ProposerRegister, error) {
	operatorAddr, err := address.FromString(operatorAddrStr)
	if err != nil {
		return nil, err
	}

	rewardAddress, err := address.FromString(rewardAddrStr)
	if err != nil {
		return nil, err
	}

	pr := &ProposerRegister{
		AbstractAction: AbstractAction{
			version:  version.ProtocolVersion,
			nonce:    nonce,
			gasLimit: gasLimit,
			gasPrice: gasPrice,
		},
		operatorAddress: operatorAddr,
		rewardAddress:   rewardAddress,
		duration:        duration,
		autoStake:       autoStake,
		payload:         payload,
	}

	if len(ownerAddrStr) > 0 {
		ownerAddress, err := address.FromString(ownerAddrStr)
		if err != nil {
			return nil, err
		}
		pr.ownerAddress = ownerAddress
	}
	return pr, nil
}

// OwnerAddress returns the owner address
func (pr *ProposerRegister) OwnerAddress() address.Address { return pr.ownerAddress }

// OperatorAddress returns the operator address
func (pr *ProposerRegister) OperatorAddress() address.Address { return pr.operatorAddress }

// Duration returns the duration
func (pr *ProposerRegister) Duration() uint32 { return pr.duration }

// AutoStake returns the auto stake flag
func (pr *ProposerRegister) AutoStake() bool { return pr.autoStake }

// RewardAddress returns the reward address
func (pr *ProposerRegister) RewardAddress() address.Address { return pr.rewardAddress }

// Payload returns the payload
func (pr *ProposerRegister) Payload() []byte { return pr.payload }

// Amount returns the amount
func (pr *ProposerRegister) Amount() *big.Int {
	return big.NewInt(int64(ProposerRegisterAmount))
}

// IntrinsicGas returns the intrinsic gas of a ProposerRegister
func (pr *ProposerRegister) IntrinsicGas() (uint64, error) {
	payloadSize := uint64(len(pr.Payload()))
	return CalculateIntrinsicGas(ProposerRegisterBaseIntrinsicGas, ProposerRegisterPayloadGas, payloadSize)
}

// Cost returns the total cost of a ProposerRegister
func (pr *ProposerRegister) Cost() (*big.Int, error) {
	intrinsicGas, err := pr.IntrinsicGas()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get intrinsic gas for the ProposerRegister creates")
	}
	fee := big.NewInt(0).Mul(pr.GasPrice(), big.NewInt(0).SetUint64(intrinsicGas))
	return big.NewInt(0).Add(pr.Amount(), fee), nil
}

// SanityCheck validates the ProposerRegister
func (pr *ProposerRegister) SanityCheck() error {
	return pr.AbstractAction.SanityCheck()
}

// Proto converts to protobuf ProposerRegister Action
func (pr *ProposerRegister) Proto() *iotextypes.ProposerRegister {
	act := iotextypes.ProposerRegister{
		Proposer: &iotextypes.ProposerBasicInfo{
			OperatorAddress: pr.operatorAddress.String(),
			RewardAddress:   pr.rewardAddress.String(),
		},
		StakedDuration: pr.duration,
		AutoStake:      pr.autoStake,
	}

	if pr.ownerAddress != nil {
		act.OwnerAddress = pr.ownerAddress.String()
	}

	if len(pr.payload) > 0 {
		act.Payload = make([]byte, len(pr.payload))
		copy(act.Payload, pr.payload)
	}
	return &act
}

// LoadProto converts a protobuf's Action to ProposerRegister
func (pr *ProposerRegister) LoadProto(pbAct *iotextypes.ProposerRegister) error {
	if pbAct == nil {
		return ErrNilProto
	}

	pInfo := pbAct.GetProposer()

	operatorAddr, err := address.FromString(pInfo.GetOperatorAddress())
	if err != nil {
		return err
	}
	rewardAddr, err := address.FromString(pInfo.GetRewardAddress())
	if err != nil {
		return err
	}

	pr.operatorAddress = operatorAddr
	pr.rewardAddress = rewardAddr
	pr.duration = pbAct.GetStakedDuration()
	pr.autoStake = pbAct.GetAutoStake()

	pr.payload = nil
	if len(pbAct.GetPayload()) > 0 {
		pr.payload = make([]byte, len(pbAct.GetPayload()))
		copy(pr.payload, pbAct.GetPayload())
	}

	if len(pbAct.GetOwnerAddress()) > 0 {
		ownerAddr, err := address.FromString(pbAct.GetOwnerAddress())
		if err != nil {
			return err
		}
		pr.ownerAddress = ownerAddr
	}
	return nil
}
