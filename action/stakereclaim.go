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
	// StakeReclaimPayloadGas represents the stake reclaim payload gas per uint
	StakeReclaimPayloadGas = uint64(100)
	// StakeReclaimBaseIntrinsicGas represents the base intrinsic gas for stake reclaim
	StakeReclaimBaseIntrinsicGas = uint64(10000)
)

// StakeReclaim defines the action of stake restake/withdraw
type StakeReclaim struct {
	AbstractAction

	bucketIndex uint64
	payload     []byte
}

// BucketIndex returns bucket index
func (sr *StakeReclaim) BucketIndex() uint64 { return sr.bucketIndex }

// Payload returns the payload bytes
func (sr *StakeReclaim) Payload() []byte { return sr.payload }

// Serialize returns a raw byte stream of the stake reclaim action struct
func (sr *StakeReclaim) Serialize() []byte {
	return byteutil.Must(proto.Marshal(sr.Proto()))
}

// Proto converts to protobuf stake reclaim action struct
func (sr *StakeReclaim) Proto() *iotextypes.StakeReclaim {
	act := &iotextypes.StakeReclaim{
		BucketIndex: sr.bucketIndex,
		Payload:     sr.payload,
	}

	return act
}

// LoadProto converts a protobuf's Action to StakeReclaim
func (sr *StakeReclaim) LoadProto(pbAct *iotextypes.StakeReclaim) error {
	if pbAct == nil {
		return errors.New("empty action proto to load")
	}
	sr = &StakeReclaim{}

	sr.bucketIndex = pbAct.GetBucketIndex()
	sr.payload = pbAct.GetPayload()
	return nil
}

// StakeUnstake defines the action of unstake
type StakeUnstake struct {
	StakeReclaim
}

// NewStakeUnstake returns a StakeUnstake instance
func NewStakeUnstake(
	nonce uint64,
	bucketIndex uint64,
	payload []byte,
	gasLimit uint64,
	gasPrice *big.Int,
) (*StakeUnstake, error) {
	return &StakeUnstake{
		StakeReclaim{
			AbstractAction: AbstractAction{
				version:  version.ProtocolVersion,
				nonce:    nonce,
				gasLimit: gasLimit,
				gasPrice: gasPrice,
			},
			bucketIndex: bucketIndex,
			payload:     payload,
		},
	}, nil
}

// IntrinsicGas returns the intrinsic gas of a stakeunstake
func (su *StakeUnstake) IntrinsicGas() (uint64, error) {
	payloadSize := uint64(len(su.Payload()))
	if (math.MaxUint64-StakeReclaimBaseIntrinsicGas)/StakeReclaimPayloadGas < payloadSize {
		return 0, ErrOutOfGas
	}

	return payloadSize*StakeReclaimPayloadGas + StakeReclaimBaseIntrinsicGas, nil
}

// Cost returns the total cost of a stakeunstake
func (su *StakeUnstake) Cost() (*big.Int, error) {
	intrinsicGas, err := su.IntrinsicGas()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get intrinsic gas for the unstake")
	}
	stakeUnstakeFee := big.NewInt(0).Mul(su.GasPrice(), big.NewInt(0).SetUint64(intrinsicGas))
	return stakeUnstakeFee, nil
}

// StakeWithdraw defines the action of stake withdraw
type StakeWithdraw struct {
	StakeReclaim
}

// NewStakeWithdraw returns a StakeWithdraw instance
func NewStakeWithdraw(
	nonce uint64,
	bucketIndex uint64,
	payload []byte,
	gasLimit uint64,
	gasPrice *big.Int,
) (*StakeWithdraw, error) {
	return &StakeWithdraw{
		StakeReclaim{
			AbstractAction: AbstractAction{
				version:  version.ProtocolVersion,
				nonce:    nonce,
				gasLimit: gasLimit,
				gasPrice: gasPrice,
			},
			bucketIndex: bucketIndex,
			payload:     payload,
		},
	}, nil
}

// IntrinsicGas returns the intrinsic gas of a stakewithdraw
func (sw *StakeWithdraw) IntrinsicGas() (uint64, error) {
	payloadSize := uint64(len(sw.Payload()))
	if (math.MaxUint64-StakeReclaimBaseIntrinsicGas)/StakeReclaimPayloadGas < payloadSize {
		return 0, ErrOutOfGas
	}

	return payloadSize*StakeReclaimPayloadGas + StakeReclaimBaseIntrinsicGas, nil
}

// Cost returns the total cost of a stakewithdraw
func (sw *StakeWithdraw) Cost() (*big.Int, error) {
	intrinsicGas, err := sw.IntrinsicGas()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get intrinsic gas for the StakeWithdraw")
	}
	stakeUnstakeFee := big.NewInt(0).Mul(sw.GasPrice(), big.NewInt(0).SetUint64(intrinsicGas))
	return stakeUnstakeFee, nil
}
