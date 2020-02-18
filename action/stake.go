// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disklaimed. This source code is governed by Apache
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
	// StakePayloadGas represents the Stake payload gas per uint
	StakePayloadGas = uint64(100)
	// StakeBaseIntrinsicGas represents the base intrinsic gas for Stake create
	StakeBaseIntrinsicGas = uint64(10000)
)

// Stake defines the action of Stake creation
type Stake struct {
	AbstractAction

	candName  string
	amount    *big.Int
	duration  uint32
	autoStake bool
	payload   []byte
}

// NewStake returns a Stake instance
func NewStake(
	nonce uint64,
	candName string,
	amount *big.Int,
	duration uint32,
	autoStake bool,
	payload []byte,
	gasLimit uint64,
	gasPrice *big.Int,
) (*Stake, error) {
	return &Stake{
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
func (sk *Stake) Amount() *big.Int { return sk.amount }

// Payload returns the payload bytes
func (sk *Stake) Payload() []byte { return sk.payload }

// CandName returns the candidate name
func (sk *Stake) CandName() string { return sk.candName }

// Duration returns the staked duration
func (sk *Stake) Duration() uint32 { return sk.duration }

// AutoStake returns the flag of autoStake s
func (sk *Stake) AutoStake() bool { return sk.autoStake }

// Serialize returns a raw byte stream of the Stake struct
func (sk *Stake) Serialize() []byte {
	return byteutil.Must(proto.Marshal(sk.Proto()))
}

// Proto converts to protobuf Stake Action
func (sk *Stake) Proto() *iotextypes.StakeCreate {
	act := &iotextypes.StakeCreate{
		CandidateName:  sk.candName,
		StakedDuration: sk.duration,
		AutoStake:      sk.autoStake,
		Payload:        sk.payload,
	}

	if sk.amount != nil {
		act.StakedAmount = sk.amount.String()
	}
	return act
}

// LoadProto converts a protobuf's Action to Stake
func (sk *Stake) LoadProto(pbAct *iotextypes.StakeCreate) error {
	if pbAct == nil {
		return errors.New("empty action proto to load")
	}
	sk = &Stake{}

	sk.candName = pbAct.GetCandidateName()
	sk.duration = pbAct.GetStakedDuration()
	sk.autoStake = pbAct.GetAutoStake()
	sk.payload = pbAct.GetPayload()
	sk.amount = big.NewInt(0)
	sk.amount.SetString(pbAct.GetStakedAmount(), 10)

	return nil
}

// IntrinsicGas returns the intrinsic gas of a Stake
func (sk *Stake) IntrinsicGas() (uint64, error) {
	payloadSize := uint64(len(sk.Payload()))
	return calculateIntrinsicGas(StakeBaseIntrinsicGas, StakePayloadGas, payloadSize)
}

// Cost returns the total cost of a Stake
func (sk *Stake) Cost() (*big.Int, error) {
	intrinsicGas, err := sk.IntrinsicGas()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get intrinsic gas for the Stake creates")
	}
	stakeFee := big.NewInt(0).Mul(sk.GasPrice(), big.NewInt(0).SetUint64(intrinsicGas))
	return big.NewInt(0).Add(sk.Amount(), stakeFee), nil
}
