// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"math"
	"math/big"

	"github.com/golang/protobuf/proto"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/pkg/version"
)

const (
	// StakeCreatePayloadGas represents the stakecreate payload gas per uint
	StakeCreatePayloadGas = uint64(100)
	// StakeCreateBaseIntrinsicGas represents the base intrinsic gas for stake create
	StakeCreateBaseIntrinsicGas = uint64(10000)
)

// StakeCreate defines the action of stake creation
type StakeCreate struct {
	AbstractAction

	candName  string
	amount    *big.Int
	duration  uint32
	autoStake bool
	payload   []byte
}

// NewStakeCreate returns a StakeCreate instance
func NewStakeCreate(
	nonce uint64,
	candName string,
	amount *big.Int,
	duration uint32,
	autoStake bool,
	payload []byte,
	gasLimit uint64,
	gasPrice *big.Int,
) (*StakeCreate, error) {
	return &StakeCreate{
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
func (sc *StakeCreate) Amount() *big.Int { return sc.amount }

// Payload returns the payload bytes
func (sc *StakeCreate) Payload() []byte { return sc.payload }

// CandName returns the candidate name
func (sc *StakeCreate) CandName() string { return sc.candName }

// Duration returns the staked duration
func (sc *StakeCreate) Duration() uint32 { return sc.duration }

// AutoStake returns the flag of autostake s
func (sc *StakeCreate) AutoStake() bool { return sc.autoStake }

// Serialize returns a raw byte stream of the Stake Create struct
func (sc *StakeCreate) Serialize() []byte {
	return byteutil.Must(proto.Marshal(sc.Proto()))
}

// Proto converts to protobuf stakeCreate Action
func (sc *StakeCreate) Proto() *iotextypes.StakeCreate {
	act := &iotextypes.StakeCreate{
		CandidateName:  sc.candName,
		StakedDuration: sc.duration,
		AutoStake:      sc.autoStake,
		Payload:        sc.payload,
	}

	if sc.amount != nil {
		act.StakedAmount = sc.amount.String()
	}
	return act
}

// LoadProto converts a protobuf's Action to StakeCreate
func (sc *StakeCreate) LoadProto(pbAct *iotextypes.StakeCreate) error {
	if pbAct == nil {
		return errors.New("empty action proto to load")
	}
	sc = &StakeCreate{}

	sc.candName = pbAct.GetCandidateName()
	sc.duration = pbAct.GetStakedDuration()
	sc.autoStake = pbAct.GetAutoStake()
	sc.payload = pbAct.GetPayload()
	sc.amount = big.NewInt(0)
	sc.amount.SetString(pbAct.GetStakedAmount(), 10)

	return nil
}

// IntrinsicGas returns the intrinsic gas of a stakecreate
func (sc *StakeCreate) IntrinsicGas() (uint64, error) {
	payloadSize := uint64(len(sc.Payload()))
	if (math.MaxUint64-StakeCreateBaseIntrinsicGas)/StakeCreatePayloadGas < payloadSize {
		return 0, ErrOutOfGas
	}

	return payloadSize*StakeCreatePayloadGas + StakeCreateBaseIntrinsicGas, nil
}

// Cost returns the total cost of a stakecreate
func (sc *StakeCreate) Cost() (*big.Int, error) {
	intrinsicGas, err := sc.IntrinsicGas()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get intrinsic gas for the stake creates")
	}
	stakecreateFee := big.NewInt(0).Mul(sc.GasPrice(), big.NewInt(0).SetUint64(intrinsicGas))
	return big.NewInt(0).Add(sc.Amount(), stakecreateFee), nil
}
