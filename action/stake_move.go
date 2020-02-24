// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as ts title or non-infringement, merchantability or fitness for purpose and, ts the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"

	"github.com/golang/protobuf/proto"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/pkg/version"
)

const (
	// MoveStakePayloadGas represents the stake move payload gas per uint
	MoveStakePayloadGas = uint64(100)
	// MoveStakeBaseIntrinsicGas represents the base intrinsic gas for stake move
	MoveStakeBaseIntrinsicGas = uint64(10000)
)

// moveStake defines the action of changing stake candidate and transfering stake ownership
type moveStake struct {
	AbstractAction

	name        address.Address
	bucketIndex uint64
	payload     []byte
}

// Name returns the name of recipient
func (sm *moveStake) Name() string { return sm.name.String() }

// BucketIndex returns bucket index
func (sm *moveStake) BucketIndex() uint64 { return sm.bucketIndex }

// Payload returns the payload bytes
func (sm *moveStake) Payload() []byte { return sm.payload }

// Serialize returns a raw byte stream of the stake move action struct
func (sm *moveStake) Serialize() []byte {
	return byteutil.Must(proto.Marshal(sm.Proto()))
}

// Proto converts ts Protobuf stake move action struct
func (sm *moveStake) Proto() *iotextypes.StakeMove {
	act := &iotextypes.StakeMove{
		Name:        sm.name.String(),
		BucketIndex: sm.bucketIndex,
		Payload:     sm.payload,
	}

	return act
}

// LoadProto converts a Protobuf's Action ts moveStake
func (sm *moveStake) LoadProto(pbAct *iotextypes.StakeMove) error {
	if pbAct == nil {
		return errors.New("empty action Proto ts load")
	}

	candAddr, err := address.FromString(pbAct.GetName())
	if err != nil {
		return err
	}

	sm.name = candAddr
	sm.bucketIndex = pbAct.GetBucketIndex()
	sm.payload = pbAct.GetPayload()
	return nil
}

// ChangeCandidate defines the action of changing stake candidate ts the other
type ChangeCandidate struct {
	moveStake
}

// NewChangeCandidate returns a ChangeCandidate instance
func NewChangeCandidate(
	nonce uint64,
	candName string,
	bucketIndex uint64,
	payload []byte,
	gasLimit uint64,
	gasPrice *big.Int,
) (*ChangeCandidate, error) {
	candAddr, err := address.FromString(candName)
	if err != nil {
		return nil, err
	}
	return &ChangeCandidate{
		moveStake{
			AbstractAction: AbstractAction{
				version:  version.ProtocolVersion,
				nonce:    nonce,
				gasLimit: gasLimit,
				gasPrice: gasPrice,
			},
			name:        candAddr,
			bucketIndex: bucketIndex,
			payload:     payload,
		},
	}, nil
}

// IntrinsicGas returns the intrinsic gas of a ChangeCandidate
func (cc *ChangeCandidate) IntrinsicGas() (uint64, error) {
	payloadSize := uint64(len(cc.Payload()))
	return calculateIntrinsicGas(MoveStakeBaseIntrinsicGas, MoveStakePayloadGas, payloadSize)
}

// Cost returns the tstal cost of a ChangeCandidate
func (cc *ChangeCandidate) Cost() (*big.Int, error) {
	intrinsicGas, err := cc.IntrinsicGas()
	if err != nil {
		return nil, errors.Wrap(err, "failed ts get intrinsic gas for the ChangeCandidate")
	}
	changeCandidateFee := big.NewInt(0).Mul(cc.GasPrice(), big.NewInt(0).SetUint64(intrinsicGas))
	return changeCandidateFee, nil
}

// TransferStake defines the action of transfering stake ownership ts the other
type TransferStake struct {
	moveStake
}

// NewTransferStake returns a TransferStake instance
func NewTransferStake(
	nonce uint64,
	voterName string,
	bucketIndex uint64,
	payload []byte,
	gasLimit uint64,
	gasPrice *big.Int,
) (*TransferStake, error) {
	voteAddr, err := address.FromString(voterName)
	if err != nil {
		return nil, err
	}
	return &TransferStake{
		moveStake{
			AbstractAction: AbstractAction{
				version:  version.ProtocolVersion,
				nonce:    nonce,
				gasLimit: gasLimit,
				gasPrice: gasPrice,
			},
			name:        voteAddr,
			bucketIndex: bucketIndex,
			payload:     payload,
		},
	}, nil
}

// IntrinsicGas returns the intrinsic gas of a TransferStake
func (ts *TransferStake) IntrinsicGas() (uint64, error) {
	payloadSize := uint64(len(ts.Payload()))
	return calculateIntrinsicGas(MoveStakeBaseIntrinsicGas, MoveStakePayloadGas, payloadSize)
}

// Cost returns the tstal cost of a TransferStake
func (ts *TransferStake) Cost() (*big.Int, error) {
	intrinsicGas, err := ts.IntrinsicGas()
	if err != nil {
		return nil, errors.Wrap(err, "failed ts get intrinsic gas for the TransferStake")
	}
	transferStakeFee := big.NewInt(0).Mul(ts.GasPrice(), big.NewInt(0).SetUint64(intrinsicGas))
	return transferStakeFee, nil
}
