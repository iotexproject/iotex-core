// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"

	"github.com/golang/protobuf/proto"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/pkg/version"
)

const (
	// CreateStakePayloadGas represents the CreateStake payload gas per uint
	CreateStakePayloadGas = uint64(100)
	// CreateStakeBaseIntrinsicGas represents the base intrinsic gas for CreateStake
	CreateStakeBaseIntrinsicGas = uint64(10000)
)

// CreateStake defines the action of CreateStake creation
type CreateStake struct {
	AbstractAction

	candName  string
	amount    *big.Int
	duration  uint32
	autoStake bool
	payload   []byte
}

// NewCreateStake returns a CreateStake instance
func NewCreateStake(
	nonce uint64,
	candName string,
	amount *big.Int,
	duration uint32,
	autoStake bool,
	payload []byte,
	gasLimit uint64,
	gasPrice *big.Int,
) (*CreateStake, error) {
	return &CreateStake{
		AbstractAction: AbstractAction{
			version:  version.ProtocolVersion,
			nonce:    nonce,
			gasLimit: gasLimit,
			gasPrice: gasPrice,
		},
		candName:  candName,
		amount:    amount,
		duration:  duration,
		autoStake: autoStake,
		payload:   payload,
	}, nil
}

// Amount returns the amount
func (cs *CreateStake) Amount() *big.Int { return cs.amount }

// Payload returns the payload bytes
func (cs *CreateStake) Payload() []byte { return cs.payload }

// CandName returns the candidate name
func (cs *CreateStake) CandName() string { return cs.candName }

// Duration returns the CreateStaked duration
func (cs *CreateStake) Duration() uint32 { return cs.duration }

// AutoStake returns the flag of AutoStake s
func (cs *CreateStake) AutoStake() bool { return cs.autoStake }

// Serialize returns a raw byte stream of the CreateStake struct
func (cs *CreateStake) Serialize() []byte {
	return byteutil.Must(proto.Marshal(cs.Proto()))
}

// Proto converts to protobuf CreateStake Action
func (cs *CreateStake) Proto() *iotextypes.StakeCreate {
	act := &iotextypes.StakeCreate{
		CandidateName:  cs.candName,
		StakedDuration: cs.duration,
		AutoStake:      cs.autoStake,
		Payload:        cs.payload,
	}

	if cs.amount != nil {
		act.StakedAmount = cs.amount.String()
	}
	return act
}

// LoadProto converts a protobuf's Action to CreateStake
func (cs *CreateStake) LoadProto(pbAct *iotextypes.StakeCreate) error {
	if pbAct == nil {
		return errors.New("empty action proto to load")
	}
	cs = &CreateStake{}

	cs.candName = pbAct.GetCandidateName()
	cs.duration = pbAct.GetStakedDuration()
	cs.autoStake = pbAct.GetAutoStake()
	cs.payload = pbAct.GetPayload()
	cs.amount = big.NewInt(0)
	if len(pbAct.GetStakedAmount()) > 0 {
		cs.amount.SetString(pbAct.GetStakedAmount(), 10)
	}

	return nil
}

// IntrinsicGas returns the intrinsic gas of a CreateStake
func (cs *CreateStake) IntrinsicGas() (uint64, error) {
	payloadSize := uint64(len(cs.Payload()))
	return calculateIntrinsicGas(CreateStakeBaseIntrinsicGas, CreateStakePayloadGas, payloadSize)
}

// Cost returns the total cost of a CreateStake
func (cs *CreateStake) Cost() (*big.Int, error) {
	intrinsicGas, err := cs.IntrinsicGas()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get intrinsic gas for the CreateStake creates")
	}
	CreateStakeFee := big.NewInt(0).Mul(cs.GasPrice(), big.NewInt(0).SetUint64(intrinsicGas))
	return big.NewInt(0).Add(cs.Amount(), CreateStakeFee), nil
}
