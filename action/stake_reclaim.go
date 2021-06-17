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
	// ReclaimStakePayloadGas represents the stake reclaim payload gas per uint
	ReclaimStakePayloadGas = uint64(100)
	// ReclaimStakeBaseIntrinsicGas represents the base intrinsic gas for stake reclaim
	ReclaimStakeBaseIntrinsicGas = uint64(10000)
)

// reclaimStake defines the action of stake restake/withdraw
type reclaimStake struct {
	AbstractAction

	bucketIndex uint64
	payload     []byte
}

// BucketIndex returns bucket index
func (sr *reclaimStake) BucketIndex() uint64 { return sr.bucketIndex }

// Payload returns the payload bytes
func (sr *reclaimStake) Payload() []byte { return sr.payload }

// Serialize returns a raw byte stream of the stake reclaim action struct
func (sr *reclaimStake) Serialize() []byte {
	return byteutil.Must(proto.Marshal(sr.Proto()))
}

// Proto converts to protobuf stake reclaim action struct
func (sr *reclaimStake) Proto() *iotextypes.StakeReclaim {
	act := &iotextypes.StakeReclaim{
		BucketIndex: sr.bucketIndex,
		Payload:     sr.payload,
	}

	return act
}

// LoadProto converts a protobuf's Action to reclaimStake
func (sr *reclaimStake) LoadProto(pbAct *iotextypes.StakeReclaim) error {
	if pbAct == nil {
		return errors.New("empty action proto to load")
	}

	sr.bucketIndex = pbAct.GetBucketIndex()
	sr.payload = pbAct.GetPayload()
	return nil
}

// Unstake defines the action of unstake
type Unstake struct {
	reclaimStake
}

// NewUnstake returns a Unstake instance
func NewUnstake(
	nonce uint64,
	bucketIndex uint64,
	payload []byte,
	gasLimit uint64,
	gasPrice *big.Int,
) (*Unstake, error) {
	return &Unstake{
		reclaimStake{
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

// IntrinsicGas returns the intrinsic gas of a Unstake
func (su *Unstake) IntrinsicGas() (uint64, error) {
	payloadSize := uint64(len(su.Payload()))
	return calculateIntrinsicGas(ReclaimStakeBaseIntrinsicGas, ReclaimStakePayloadGas, payloadSize)
}

// Cost returns the total cost of a Unstake
func (su *Unstake) Cost() (*big.Int, error) {
	intrinsicGas, err := su.IntrinsicGas()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get intrinsic gas for the unstake")
	}
	unstakeFee := big.NewInt(0).Mul(su.GasPrice(), big.NewInt(0).SetUint64(intrinsicGas))
	return unstakeFee, nil
}

// WithdrawStake defines the action of stake withdraw
type WithdrawStake struct {
	reclaimStake
}

// NewWithdrawStake returns a WithdrawStake instance
func NewWithdrawStake(
	nonce uint64,
	bucketIndex uint64,
	payload []byte,
	gasLimit uint64,
	gasPrice *big.Int,
) (*WithdrawStake, error) {
	return &WithdrawStake{
		reclaimStake{
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

// IntrinsicGas returns the intrinsic gas of a WithdrawStake
func (sw *WithdrawStake) IntrinsicGas() (uint64, error) {
	payloadSize := uint64(len(sw.Payload()))
	return calculateIntrinsicGas(ReclaimStakeBaseIntrinsicGas, ReclaimStakePayloadGas, payloadSize)
}

// Cost returns the total cost of a WithdrawStake
func (sw *WithdrawStake) Cost() (*big.Int, error) {
	intrinsicGas, err := sw.IntrinsicGas()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get intrinsic gas for the WithdrawStake")
	}
	withdrawFee := big.NewInt(0).Mul(sw.GasPrice(), big.NewInt(0).SetUint64(intrinsicGas))
	return withdrawFee, nil
}
