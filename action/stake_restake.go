// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"bytes"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/pkg/version"
)

const (
	// RestakePayloadGas represents the Restake payload gas per uint
	RestakePayloadGas = uint64(100)
	// RestakeBaseIntrinsicGas represents the base intrinsic gas for stake again
	RestakeBaseIntrinsicGas = uint64(10000)

	restakeInterfaceABI = `[
		{
			"inputs": [
				{
					"internalType": "uint64",
					"name": "bucketIndex",
					"type": "uint64"
				},
				{
					"internalType": "uint32",
					"name": "duration",
					"type": "uint32"
				},
				{
					"internalType": "bool",
					"name": "autoStake",
					"type": "bool"
				},
				{
					"internalType": "uint8[]",
					"name": "data",
					"type": "uint8[]"
				}
			],
			"name": "restake",
			"outputs": [],
			"stateMutability": "nonpayable",
			"type": "function"
		}
	]`
)

var (
	// _restakeMethod is the interface of the abi encoding of stake action
	_restakeMethod abi.Method
)

// Restake defines the action of stake again
type Restake struct {
	AbstractAction

	bucketIndex uint64
	duration    uint32
	autoStake   bool
	payload     []byte
}

func init() {
	restakeInterface, err := abi.JSON(strings.NewReader(restakeInterfaceABI))
	if err != nil {
		panic(err)
	}
	var ok bool
	_restakeMethod, ok = restakeInterface.Methods["restake"]
	if !ok {
		panic("fail to load the method")
	}
}

// NewRestake returns a Restake instance
func NewRestake(
	nonce uint64,
	index uint64,
	duration uint32,
	autoStake bool,
	payload []byte,
	gasLimit uint64,
	gasPrice *big.Int,
) (*Restake, error) {
	return &Restake{
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
func (rs *Restake) Payload() []byte { return rs.payload }

// BucketIndex returns bucket index
func (rs *Restake) BucketIndex() uint64 { return rs.bucketIndex }

// Duration returns the updated duration
func (rs *Restake) Duration() uint32 { return rs.duration }

// AutoStake returns the autoStake boolean
func (rs *Restake) AutoStake() bool { return rs.autoStake }

// Serialize returns a raw byte stream of the Stake again struct
func (rs *Restake) Serialize() []byte {
	return byteutil.Must(proto.Marshal(rs.Proto()))
}

// Proto converts to protobuf Restake Action
func (rs *Restake) Proto() *iotextypes.StakeRestake {
	act := &iotextypes.StakeRestake{
		BucketIndex:    rs.bucketIndex,
		Payload:        rs.payload,
		StakedDuration: rs.duration,
		AutoStake:      rs.autoStake,
	}
	return act
}

// LoadProto converts a protobuf's Action to Restake
func (rs *Restake) LoadProto(pbAct *iotextypes.StakeRestake) error {
	if pbAct == nil {
		return ErrNilProto
	}

	rs.bucketIndex = pbAct.GetBucketIndex()
	rs.payload = pbAct.GetPayload()
	rs.duration = pbAct.GetStakedDuration()
	rs.autoStake = pbAct.GetAutoStake()

	return nil
}

// IntrinsicGas returns the intrinsic gas of a Restake
func (rs *Restake) IntrinsicGas() (uint64, error) {
	payloadSize := uint64(len(rs.Payload()))
	return CalculateIntrinsicGas(RestakeBaseIntrinsicGas, RestakePayloadGas, payloadSize)
}

// Cost returns the total cost of a Restake
func (rs *Restake) Cost() (*big.Int, error) {
	intrinsicGas, err := rs.IntrinsicGas()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get intrinsic gas for the stake creates")
	}
	restakeFee := big.NewInt(0).Mul(rs.GasPrice(), big.NewInt(0).SetUint64(intrinsicGas))
	return restakeFee, nil
}

// EncodeABIBinary encodes data in abi encoding
func (rs *Restake) EncodeABIBinary() ([]byte, error) {
	data, err := _restakeMethod.Inputs.Pack(rs.bucketIndex, rs.duration, rs.autoStake, rs.payload)
	if err != nil {
		return nil, err
	}
	return append(_restakeMethod.ID, data...), nil
}

// NewRestakeFromABIBinary decodes data into Restake action
func NewRestakeFromABIBinary(data []byte) (*Restake, error) {
	var (
		paramsMap = map[string]interface{}{}
		ok        bool
		rs        Restake
	)
	// sanity check
	if len(data) <= 4 || !bytes.Equal(_restakeMethod.ID, data[:4]) {
		return nil, errDecodeFailure
	}
	if err := _restakeMethod.Inputs.UnpackIntoMap(paramsMap, data[4:]); err != nil {
		return nil, err
	}
	if rs.bucketIndex, ok = paramsMap["bucketIndex"].(uint64); !ok {
		return nil, errDecodeFailure
	}
	if rs.duration, ok = paramsMap["duration"].(uint32); !ok {
		return nil, errDecodeFailure
	}
	if rs.autoStake, ok = paramsMap["autoStake"].(bool); !ok {
		return nil, errDecodeFailure
	}
	if rs.payload, ok = paramsMap["data"].([]byte); !ok {
		return nil, errDecodeFailure
	}
	return &rs, nil
}
