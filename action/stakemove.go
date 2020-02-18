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
	// StakeMovePayloadGas represents the stake move payload gas per uint
	StakeMovePayloadGas = uint64(100)
	// StakeMoveBaseIntrinsicGas represents the base intrinsic gas for stake move
	StakeMoveBaseIntrinsicGas = uint64(10000)
)

// StakeMove defines the action of changing stake candidate and transfering stake ownership
type StakeMove struct {
	AbstractAction

	name        string
	bucketIndex uint64
	payload     []byte
}

// Name returns the name of recipient
func (sm *StakeMove) Name() string { return sm.name }

// BucketIndex returns bucket index
func (sm *StakeMove) BucketIndex() uint64 { return sm.bucketIndex }

// Payload returns the payload bytes
func (sm *StakeMove) Payload() []byte { return sm.payload }

// Serialize returns a raw byte stream of the stake move action struct
func (sm *StakeMove) Serialize() []byte {
	return byteutil.Must(proto.Marshal(sm.Proto()))
}

// Proto converts to protobuf stake move action struct
func (sm *StakeMove) Proto() *iotextypes.StakeMove {
	act := &iotextypes.StakeMove{
		BucketIndex: sm.bucketIndex,
		Payload:     sm.payload,
	}

	return act
}

// LoadProto converts a protobuf's Action to StakeMove
func (sm *StakeMove) LoadProto(pbAct *iotextypes.StakeMove) error {
	if pbAct == nil {
		return errors.New("empty action proto to load")
	}
	sm = &StakeMove{}

	sm.bucketIndex = pbAct.GetBucketIndex()
	sm.payload = pbAct.GetPayload()
	return nil
}

// StakeChangeCandidate defines the action of changing stake candidate to the other
type StakeChangeCandidate struct {
	StakeMove
}

// NewStakeChangeCandidate returns a StakeChangeCandidate instance
func NewStakeChangeCandidate(
	nonce uint64,
	candName string,
	bucketIndex uint64,
	payload []byte,
	gasLimit uint64,
	gasPrice *big.Int,
) (*StakeChangeCandidate, error) {
	return &StakeChangeCandidate{
		StakeMove{
			AbstractAction: AbstractAction{
				version:  version.ProtocolVersion,
				nonce:    nonce,
				gasLimit: gasLimit,
				gasPrice: gasPrice,
			},
			name:        candName,
			bucketIndex: bucketIndex,
			payload:     payload,
		},
	}, nil
}

// IntrinsicGas returns the intrinsic gas of a StakeChangeCandidate
func (cc *StakeChangeCandidate) IntrinsicGas() (uint64, error) {
	payloadSize := uint64(len(cc.Payload()))
	if (math.MaxUint64-StakeMoveBaseIntrinsicGas)/StakeMovePayloadGas < payloadSize {
		return 0, ErrOutOfGas
	}

	return payloadSize*StakeMovePayloadGas + StakeMoveBaseIntrinsicGas, nil
}

// Cost returns the total cost of a StakeChangeCandidate
func (cc *StakeChangeCandidate) Cost() (*big.Int, error) {
	intrinsicGas, err := cc.IntrinsicGas()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get intrinsic gas for the StakeChangeCandidate")
	}
	stakeChangeCandidateFee := big.NewInt(0).Mul(cc.GasPrice(), big.NewInt(0).SetUint64(intrinsicGas))
	return stakeChangeCandidateFee, nil
}

// StakeTransferOwnership defines the action of transfering stake ownership to the other
type StakeTransferOwnership struct {
	StakeMove
}

// NewStakeTransferOwnership returns a StakeTransferOwnership instance
func NewStakeTransferOwnership(
	nonce uint64,
	voterName string,
	bucketIndex uint64,
	payload []byte,
	gasLimit uint64,
	gasPrice *big.Int,
) (*StakeTransferOwnership, error) {
	return &StakeTransferOwnership{
		StakeMove{
			AbstractAction: AbstractAction{
				version:  version.ProtocolVersion,
				nonce:    nonce,
				gasLimit: gasLimit,
				gasPrice: gasPrice,
			},
			name:        voterName,
			bucketIndex: bucketIndex,
			payload:     payload,
		},
	}, nil
}

// IntrinsicGas returns the intrinsic gas of a StakeTransferOwnership
func (to *StakeTransferOwnership) IntrinsicGas() (uint64, error) {
	payloadSize := uint64(len(to.Payload()))
	if (math.MaxUint64-StakeMoveBaseIntrinsicGas)/StakeMovePayloadGas < payloadSize {
		return 0, ErrOutOfGas
	}

	return payloadSize*StakeMovePayloadGas + StakeMoveBaseIntrinsicGas, nil
}

// Cost returns the total cost of a StakeTransferOwnership
func (to *StakeTransferOwnership) Cost() (*big.Int, error) {
	intrinsicGas, err := to.IntrinsicGas()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get intrinsic gas for the StakeTransferOwnership")
	}
	stakeChangeCandidateFee := big.NewInt(0).Mul(to.GasPrice(), big.NewInt(0).SetUint64(intrinsicGas))
	return stakeChangeCandidateFee, nil
}
