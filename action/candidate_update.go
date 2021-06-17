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

// CandidateUpdateBaseIntrinsicGas represents the base intrinsic gas for CandidateUpdate
const CandidateUpdateBaseIntrinsicGas = uint64(10000)

// CandidateUpdate is the action to update a candidate
type CandidateUpdate struct {
	AbstractAction

	name            string
	operatorAddress address.Address
	rewardAddress   address.Address
}

// NewCandidateUpdate creates a CandidateUpdate instance
func NewCandidateUpdate(
	nonce uint64,
	name, operatorAddrStr, rewardAddrStr string,
	gasLimit uint64,
	gasPrice *big.Int,
) (*CandidateUpdate, error) {
	cu := &CandidateUpdate{
		AbstractAction: AbstractAction{
			version:  version.ProtocolVersion,
			nonce:    nonce,
			gasLimit: gasLimit,
			gasPrice: gasPrice,
		},
		name: name,
	}

	var err error
	if len(operatorAddrStr) > 0 {
		cu.operatorAddress, err = address.FromString(operatorAddrStr)
		if err != nil {
			return nil, err
		}
	}

	if len(rewardAddrStr) > 0 {
		cu.rewardAddress, err = address.FromString(rewardAddrStr)
		if err != nil {
			return nil, err
		}
	}
	return cu, nil
}

// Name returns candidate name to update
func (cu *CandidateUpdate) Name() string { return cu.name }

// OperatorAddress returns candidate operatorAddress to update
func (cu *CandidateUpdate) OperatorAddress() address.Address { return cu.operatorAddress }

// RewardAddress returns candidate rewardAddress to update
func (cu *CandidateUpdate) RewardAddress() address.Address { return cu.rewardAddress }

// Serialize returns a raw byte stream of the CandidateUpdate struct
func (cu *CandidateUpdate) Serialize() []byte {
	return byteutil.Must(proto.Marshal(cu.Proto()))
}

// Proto converts to protobuf CandidateUpdate Action
func (cu *CandidateUpdate) Proto() *iotextypes.CandidateBasicInfo {
	act := &iotextypes.CandidateBasicInfo{
		Name: cu.name,
	}

	if cu.operatorAddress != nil {
		act.OperatorAddress = cu.operatorAddress.String()
	}

	if cu.rewardAddress != nil {
		act.RewardAddress = cu.rewardAddress.String()
	}

	return act
}

// LoadProto converts a protobuf's Action to CandidateUpdate
func (cu *CandidateUpdate) LoadProto(pbAct *iotextypes.CandidateBasicInfo) error {
	if pbAct == nil {
		return errors.New("empty action proto to load")
	}

	cu.name = pbAct.GetName()

	if len(pbAct.GetOperatorAddress()) > 0 {
		operatorAddr, err := address.FromString(pbAct.GetOperatorAddress())
		if err != nil {
			return err
		}
		cu.operatorAddress = operatorAddr
	}

	if len(pbAct.GetRewardAddress()) > 0 {
		rewardAddr, err := address.FromString(pbAct.GetRewardAddress())
		if err != nil {
			return err
		}
		cu.rewardAddress = rewardAddr
	}
	return nil
}

// IntrinsicGas returns the intrinsic gas of a CandidateUpdate
func (cu *CandidateUpdate) IntrinsicGas() (uint64, error) {
	return CandidateUpdateBaseIntrinsicGas, nil
}

// Cost returns the total cost of a CandidateUpdate
func (cu *CandidateUpdate) Cost() (*big.Int, error) {
	intrinsicGas, err := cu.IntrinsicGas()
	if err != nil {
		return nil, errors.Wrap(err, "failed ts get intrinsic gas for the CandidateUpdate")
	}
	fee := big.NewInt(0).Mul(cu.GasPrice(), big.NewInt(0).SetUint64(intrinsicGas))
	return fee, nil
}
