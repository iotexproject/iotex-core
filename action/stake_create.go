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

	"github.com/iotexproject/iotex-proto/golang/iotextypes"

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
	candidateName, amount string,
	duration uint32,
	autoStake bool,
	payload []byte,
	gasLimit uint64,
	gasPrice *big.Int,
) (*CreateStake, error) {
	stake, ok := new(big.Int).SetString(amount, 10)
	if !ok {
		return nil, errors.Wrapf(ErrInvalidAmount, "amount %s", amount)
	}

	return &CreateStake{
		AbstractAction: AbstractAction{
			version:  version.ProtocolVersion,
			nonce:    nonce,
			gasLimit: gasLimit,
			gasPrice: gasPrice,
		},
		candName:  candidateName,
		amount:    stake,
		duration:  duration,
		autoStake: autoStake,
		payload:   payload,
	}, nil
}

// Amount returns the amount
func (cs *CreateStake) Amount() *big.Int { return cs.amount }

// Payload returns the payload bytes
func (cs *CreateStake) Payload() []byte { return cs.payload }

// Candidate returns the candidate name
func (cs *CreateStake) Candidate() string { return cs.candName }

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
	act := iotextypes.StakeCreate{
		CandidateName:  cs.candName,
		StakedDuration: cs.duration,
		AutoStake:      cs.autoStake,
	}

	if cs.amount != nil {
		act.StakedAmount = cs.amount.String()
	}

	if len(cs.payload) > 0 {
		act.Payload = make([]byte, len(cs.payload))
		copy(act.Payload, cs.payload)
	}
	return &act
}

// LoadProto converts a protobuf's Action to CreateStake
func (cs *CreateStake) LoadProto(pbAct *iotextypes.StakeCreate) error {
	if pbAct == nil {
		return errors.New("empty action proto to load")
	}

	cs.candName = pbAct.GetCandidateName()
	cs.duration = pbAct.StakedDuration
	cs.autoStake = pbAct.AutoStake

	if len(pbAct.GetStakedAmount()) > 0 {
		var ok bool
		if cs.amount, ok = new(big.Int).SetString(pbAct.StakedAmount, 10); !ok {
			return errors.Errorf("invalid amount %s", pbAct.StakedAmount)
		}
	}

	cs.payload = nil
	if len(pbAct.Payload) > 0 {
		cs.payload = make([]byte, len(pbAct.Payload))
		copy(cs.payload, pbAct.Payload)
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

// SanityCheck validates the variables in the action
func (cs *CreateStake) SanityCheck() error {
	if cs.Amount().Sign() <= 0 {
		return errors.Wrap(ErrInvalidAmount, "negative value")
	}

	return cs.AbstractAction.SanityCheck()
}
