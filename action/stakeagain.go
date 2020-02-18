// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disalaimed. This source code is governed by Apache
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
	// StakeAgainPayloadGas represents the StakeAgain payload gas per uint
	StakeAgainPayloadGas = uint64(100)
	// StakeAgainBaseIntrinsicGas represents the base intrinsic gas for stake again
	StakeAgainBaseIntrinsicGas = uint64(10000)
)

// StakeAgain defines the action of stake again
type StakeAgain struct {
	AbstractAction

	bucketIndex uint64
	duration    uint32
	autoStake   bool
	payload     []byte
}

// NewStakeAgain returns a StakeAgain instance
func NewStakeAgain(
	nonce uint64,
	index uint64,
	duration uint32,
	autoStake bool,
	payload []byte,
	gasLimit uint64,
	gasPrice *big.Int,
) (*StakeAgain, error) {
	return &StakeAgain{
		AbstractAction: AbstractAction{
			version:  version.ProtocolVersion,
			nonce:    nonce,
			gasLimit: gasLimit,
			gasPrice: gasPrice,
		},
		bucketIndex: index,
		duration:    duration,
		autoStake:   autoStake,
		payload:     payload,
	}, nil
}

// Payload returns the payload bytes
func (sa *StakeAgain) Payload() []byte { return sa.payload }

// BucketIndex returns bucket index
func (sa *StakeAgain) BucketIndex() uint64 { return sa.bucketIndex }

// Duration returns the updated duration
func (sa *StakeAgain) Duration() uint32 { return sa.duration }

// AutoStake returns the autoStake boolean
func (sa *StakeAgain) AutoStake() bool { return sa.autoStake }

// Serialize returns a raw byte stream of the Stake again struct
func (sa *StakeAgain) Serialize() []byte {
	return byteutil.Must(proto.Marshal(sa.Proto()))
}

// Proto converts to protobuf StakeAgain Action
func (sa *StakeAgain) Proto() *iotextypes.StakeRestake {
	act := &iotextypes.StakeRestake{
		BucketIndex:    sa.bucketIndex,
		Payload:        sa.payload,
		StakedDuration: sa.duration,
		AutoStake:      sa.autoStake,
	}
	return act
}

// LoadProto converts a protobuf's Action to StakeAgain
func (sa *StakeAgain) LoadProto(pbAct *iotextypes.StakeRestake) error {
	if pbAct == nil {
		return errors.New("empty action proto to load")
	}
	sa = &StakeAgain{}

	sa.bucketIndex = pbAct.GetBucketIndex()
	sa.payload = pbAct.GetPayload()
	sa.duration = pbAct.GetStakedDuration()
	sa.autoStake = pbAct.GetAutoStake()

	return nil
}

// IntrinsicGas returns the intrinsic gas of a StakeAgain
func (sa *StakeAgain) IntrinsicGas() (uint64, error) {
	payloadSize := uint64(len(sa.Payload()))
	if (math.MaxUint64-StakeAgainBaseIntrinsicGas)/StakeAgainPayloadGas < payloadSize {
		return 0, ErrOutOfGas
	}

	return payloadSize*StakeAgainPayloadGas + StakeAgainBaseIntrinsicGas, nil
}

// Cost returns the total cost of a StakeAgain
func (sa *StakeAgain) Cost() (*big.Int, error) {
	intrinsicGas, err := sa.IntrinsicGas()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get intrinsic gas for the stake creates")
	}
	StakeAgainFee := big.NewInt(0).Mul(sa.GasPrice(), big.NewInt(0).SetUint64(intrinsicGas))
	return StakeAgainFee, nil
}
