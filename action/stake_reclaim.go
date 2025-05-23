// Copyright (c) 2020 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"bytes"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
)

const (
	// ReclaimStakePayloadGas represents the stake reclaim payload gas per uint
	ReclaimStakePayloadGas = uint64(100)
	// ReclaimStakeBaseIntrinsicGas represents the base intrinsic gas for stake reclaim
	ReclaimStakeBaseIntrinsicGas = uint64(10000)

	_reclaimStakeInterfaceABI = `[
		{
			"inputs": [
				{
					"internalType": "uint64",
					"name": "bucketIndex",
					"type": "uint64"
				},
				{
					"internalType": "uint8[]",
					"name": "data",
					"type": "uint8[]"
				}
			],
			"name": "unstake",
			"outputs": [],
			"stateMutability": "nonpayable",
			"type": "function"
		},
		{
			"inputs": [
				{
					"internalType": "uint64",
					"name": "bucketIndex",
					"type": "uint64"
				},
				{
					"internalType": "uint8[]",
					"name": "data",
					"type": "uint8[]"
				}
			],
			"name": "withdrawStake",
			"outputs": [],
			"stateMutability": "nonpayable",
			"type": "function"
		}
	]`
)

var (
	// _unstakeMethod is the interface of the abi encoding of unstake action
	_unstakeMethod abi.Method
	// _withdrawStakeMethod is the interface of the abi encoding of withdrawStake action
	_withdrawStakeMethod abi.Method
	_                    EthCompatibleAction = (*Unstake)(nil)
	_                    EthCompatibleAction = (*WithdrawStake)(nil)
)

func init() {
	reclaimStakeInterface, err := abi.JSON(strings.NewReader(_reclaimStakeInterfaceABI))
	if err != nil {
		panic(err)
	}
	var ok bool
	_unstakeMethod, ok = reclaimStakeInterface.Methods["unstake"]
	if !ok {
		panic("fail to load the method")
	}
	_withdrawStakeMethod, ok = reclaimStakeInterface.Methods["withdrawStake"]
	if !ok {
		panic("fail to load the method")
	}
}

// reclaimStake defines the action of stake restake/withdraw
type reclaimStake struct {
	stake_common
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
		return ErrNilProto
	}

	sr.bucketIndex = pbAct.GetBucketIndex()
	sr.payload = pbAct.GetPayload()
	return nil
}

func (sr *reclaimStake) SanityCheck() error {
	return nil
}

// Unstake defines the action of unstake
type Unstake struct {
	reclaimStake
}

// NewUnstake returns a Unstake instance
func NewUnstake(bucketIndex uint64, payload []byte) *Unstake {
	return &Unstake{
		reclaimStake{
			bucketIndex: bucketIndex,
			payload:     payload,
		},
	}
}

// IntrinsicGas returns the intrinsic gas of a Unstake
func (su *Unstake) IntrinsicGas() (uint64, error) {
	payloadSize := uint64(len(su.Payload()))
	return CalculateIntrinsicGas(ReclaimStakeBaseIntrinsicGas, ReclaimStakePayloadGas, payloadSize)
}

// EthData returns the ABI-encoded data for converting to eth tx
func (su *Unstake) EthData() ([]byte, error) {
	data, err := _unstakeMethod.Inputs.Pack(su.bucketIndex, su.payload)
	if err != nil {
		return nil, err
	}
	return append(_unstakeMethod.ID, data...), nil
}

func (act *Unstake) FillAction(core *iotextypes.ActionCore) {
	core.Action = &iotextypes.ActionCore_StakeUnstake{StakeUnstake: act.Proto()}
}

// NewUnstakeFromABIBinary decodes data into WithdrawStake action
func NewUnstakeFromABIBinary(data []byte) (*Unstake, error) {
	var (
		paramsMap = map[string]interface{}{}
		ok        bool
		su        Unstake
	)
	// sanity check
	if len(data) <= 4 || !bytes.Equal(_unstakeMethod.ID, data[:4]) {
		return nil, errDecodeFailure
	}
	if err := _unstakeMethod.Inputs.UnpackIntoMap(paramsMap, data[4:]); err != nil {
		return nil, err
	}
	if su.bucketIndex, ok = paramsMap["bucketIndex"].(uint64); !ok {
		return nil, errDecodeFailure
	}
	if su.payload, ok = paramsMap["data"].([]byte); !ok {
		return nil, errDecodeFailure
	}
	return &su, nil
}

// WithdrawStake defines the action of stake withdraw
type WithdrawStake struct {
	reclaimStake
}

// NewWithdrawStake returns a WithdrawStake instance
func NewWithdrawStake(bucketIndex uint64, payload []byte) *WithdrawStake {
	return &WithdrawStake{
		reclaimStake{
			bucketIndex: bucketIndex,
			payload:     payload,
		},
	}
}

// IntrinsicGas returns the intrinsic gas of a WithdrawStake
func (sw *WithdrawStake) IntrinsicGas() (uint64, error) {
	payloadSize := uint64(len(sw.Payload()))
	return CalculateIntrinsicGas(ReclaimStakeBaseIntrinsicGas, ReclaimStakePayloadGas, payloadSize)
}

// EthData returns the ABI-encoded data for converting to eth tx
func (sw *WithdrawStake) EthData() ([]byte, error) {
	data, err := _withdrawStakeMethod.Inputs.Pack(sw.bucketIndex, sw.payload)
	if err != nil {
		return nil, err
	}
	return append(_withdrawStakeMethod.ID, data...), nil
}

func (act *WithdrawStake) FillAction(core *iotextypes.ActionCore) {
	core.Action = &iotextypes.ActionCore_StakeWithdraw{StakeWithdraw: act.Proto()}
}

// NewWithdrawStakeFromABIBinary decodes data into WithdrawStake action
func NewWithdrawStakeFromABIBinary(data []byte) (*WithdrawStake, error) {
	var (
		paramsMap = map[string]interface{}{}
		ok        bool
		sw        WithdrawStake
	)
	// sanity check
	if len(data) <= 4 || !bytes.Equal(_withdrawStakeMethod.ID, data[:4]) {
		return nil, errDecodeFailure
	}
	if err := _withdrawStakeMethod.Inputs.UnpackIntoMap(paramsMap, data[4:]); err != nil {
		return nil, err
	}
	if sw.bucketIndex, ok = paramsMap["bucketIndex"].(uint64); !ok {
		return nil, errDecodeFailure
	}
	if sw.payload, ok = paramsMap["data"].([]byte); !ok {
		return nil, errDecodeFailure
	}
	return &sw, nil
}
