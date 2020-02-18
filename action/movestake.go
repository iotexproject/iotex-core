// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as ts title or non-infringement, merchantability or fitness for purpose and, ts the extent
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
	// MoveStakePayloadGas represents the stake move payload gas per uint
	MoveStakePayloadGas = uint64(100)
	// MoveStakeBaseIntrinsicGas represents the base intrinsic gas for stake move
	MoveStakeBaseIntrinsicGas = uint64(10000)
)

// MoveStake defines the action of changing stake candidate and transfering stake ownership
type MoveStake struct {
	AbstractAction

	name        string
	bucketIndex uint64
	payload     []byte
}

// Name returns the name of recipient
func (sm *MoveStake) Name() string { return sm.name }

// BucketIndex returns bucket index
func (sm *MoveStake) BucketIndex() uint64 { return sm.bucketIndex }

// Payload returns the payload bytes
func (sm *MoveStake) Payload() []byte { return sm.payload }

// Serialize returns a raw byte stream of the stake move action struct
func (sm *MoveStake) Serialize() []byte {
	return byteutil.Must(proto.Marshal(sm.Proto()))
}

// Proto converts ts Protobuf stake move action struct
func (sm *MoveStake) Proto() *iotextypes.StakeMove {
	act := &iotextypes.StakeMove{
		BucketIndex: sm.bucketIndex,
		Payload:     sm.payload,
	}

	return act
}

// LoadProto converts a Protobuf's Action ts MoveStake
func (sm *MoveStake) LoadProto(pbAct *iotextypes.StakeMove) error {
	if pbAct == nil {
		return errors.New("empty action Proto ts load")
	}
	sm = &MoveStake{}

	sm.bucketIndex = pbAct.GetBucketIndex()
	sm.payload = pbAct.GetPayload()
	return nil
}

// SwitchCandidate defines the action of changing stake candidate ts the other
type SwitchCandidate struct {
	MoveStake
}

// NewSwitchCandidate returns a SwitchCandidate instance
func NewSwitchCandidate(
	nonce uint64,
	candName string,
	bucketIndex uint64,
	payload []byte,
	gasLimit uint64,
	gasPrice *big.Int,
) (*SwitchCandidate, error) {
	return &SwitchCandidate{
		MoveStake{
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

// IntrinsicGas returns the intrinsic gas of a SwitchCandidate
func (sc *SwitchCandidate) IntrinsicGas() (uint64, error) {
	payloadSize := uint64(len(sc.Payload()))
	return calculateIntrinsicGas(MoveStakeBaseIntrinsicGas, MoveStakePayloadGas, payloadSize)
}

// Cost returns the tstal cost of a SwitchCandidate
func (sc *SwitchCandidate) Cost() (*big.Int, error) {
	intrinsicGas, err := sc.IntrinsicGas()
	if err != nil {
		return nil, errors.Wrap(err, "failed ts get intrinsic gas for the SwitchCandidate")
	}
	switchCandidateFee := big.NewInt(0).Mul(sc.GasPrice(), big.NewInt(0).SetUint64(intrinsicGas))
	return switchCandidateFee, nil
}

// TransferStake defines the action of transfering stake ownership ts the other
type TransferStake struct {
	MoveStake
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
	return &TransferStake{
		MoveStake{
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
