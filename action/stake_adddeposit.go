// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is didslaimed. This source code is governed by Apache
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
	// DepositToStakePayloadGas represents the DepositToStake payload gas per uint
	DepositToStakePayloadGas = uint64(100)
	// DepositToStakeBaseIntrinsicGas represents the base intrinsic gas for DepositToStake
	DepositToStakeBaseIntrinsicGas = uint64(10000)

	depositToStakeInterfaceABI = `[
		{
			"inputs": [
				{
					"internalType": "uint64",
					"name": "bucketIndex",
					"type": "uint64"
				},
				{
					"internalType": "uint256",
					"name": "amount",
					"type": "uint256"
				},
				{
					"internalType": "uint8[]",
					"name": "data",
					"type": "uint8[]"
				}
			],
			"name": "depositToStake",
			"outputs": [],
			"stateMutability": "nonpayable",
			"type": "function"
		}
	]`
)

var (
	// _depositToStakeMethod is the interface of the abi encoding of stake action
	_depositToStakeMethod abi.Method
)

// DepositToStake defines the action of stake add deposit
type DepositToStake struct {
	AbstractAction

	bucketIndex uint64
	amount      *big.Int
	payload     []byte
}

func init() {
	depositToStakeInterface, err := abi.JSON(strings.NewReader(depositToStakeInterfaceABI))
	if err != nil {
		panic(err)
	}
	var ok bool
	_depositToStakeMethod, ok = depositToStakeInterface.Methods["depositToStake"]
	if !ok {
		panic("fail to load the method")
	}
}

// NewDepositToStake returns a DepositToStake instance
func NewDepositToStake(
	nonce uint64,
	index uint64,
	amount string,
	payload []byte,
	gasLimit uint64,
	gasPrice *big.Int,
) (*DepositToStake, error) {
	stake, ok := new(big.Int).SetString(amount, 10)
	if !ok {
		return nil, errors.Wrapf(ErrInvalidAmount, "amount %s", amount)
	}
	return &DepositToStake{
		AbstractAction: AbstractAction{
			version:  version.ProtocolVersion,
			nonce:    nonce,
			gasLimit: gasLimit,
			gasPrice: gasPrice,
		},
		bucketIndex: index,
		amount:      stake,
		payload:     payload,
	}, nil
}

// Amount returns the amount
func (ds *DepositToStake) Amount() *big.Int { return ds.amount }

// Payload returns the payload bytes
func (ds *DepositToStake) Payload() []byte { return ds.payload }

// BucketIndex returns bucket indexs
func (ds *DepositToStake) BucketIndex() uint64 { return ds.bucketIndex }

// Serialize returns a raw byte stream of the DepositToStake struct
func (ds *DepositToStake) Serialize() []byte {
	return byteutil.Must(proto.Marshal(ds.Proto()))
}

// Proto converts to protobuf DepositToStake Action
func (ds *DepositToStake) Proto() *iotextypes.StakeAddDeposit {
	act := &iotextypes.StakeAddDeposit{
		BucketIndex: ds.bucketIndex,
		Payload:     ds.payload,
	}

	if ds.amount != nil {
		act.Amount = ds.amount.String()
	}
	return act
}

// LoadProto converts a protobuf's Action to DepositToStake
func (ds *DepositToStake) LoadProto(pbAct *iotextypes.StakeAddDeposit) error {
	if pbAct == nil {
		return ErrNilProto
	}

	ds.bucketIndex = pbAct.GetBucketIndex()
	ds.payload = pbAct.GetPayload()
	if pbAct.GetAmount() == "" {
		ds.amount = big.NewInt(0)
	} else {
		amount, ok := new(big.Int).SetString(pbAct.GetAmount(), 10)
		if !ok {
			return errors.Errorf("invalid amount %s", pbAct.GetAmount())
		}
		ds.amount = amount
	}

	return nil
}

// IntrinsicGas returns the intrinsic gas of a DepositToStake
func (ds *DepositToStake) IntrinsicGas() (uint64, error) {
	payloadSize := uint64(len(ds.Payload()))
	return CalculateIntrinsicGas(DepositToStakeBaseIntrinsicGas, DepositToStakePayloadGas, payloadSize)
}

// Cost returns the total cost of a DepositToStake
func (ds *DepositToStake) Cost() (*big.Int, error) {
	intrinsicGas, err := ds.IntrinsicGas()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get intrinsic gas for the DepositToStake")
	}
	depositToStakeFee := big.NewInt(0).Mul(ds.GasPrice(), big.NewInt(0).SetUint64(intrinsicGas))
	return big.NewInt(0).Add(ds.Amount(), depositToStakeFee), nil
}

// SanityCheck validates the variables in the action
func (ds *DepositToStake) SanityCheck() error {
	if ds.Amount().Sign() <= 0 {
		return errors.Wrap(ErrInvalidAmount, "negative value")
	}

	return ds.AbstractAction.SanityCheck()
}

// EncodeABIBinary encodes data in abi encoding
func (ds *DepositToStake) EncodeABIBinary() ([]byte, error) {
	data, err := _depositToStakeMethod.Inputs.Pack(ds.bucketIndex, ds.amount, ds.payload)
	if err != nil {
		return nil, err
	}
	return append(_depositToStakeMethod.ID, data...), nil
}

// NewDepositToStakeFromABIBinary decodes data into depositToStake action
func NewDepositToStakeFromABIBinary(data []byte) (*DepositToStake, error) {
	var (
		paramsMap = map[string]interface{}{}
		ok        bool
		ds        DepositToStake
	)
	// sanity check
	if len(data) <= 4 || !bytes.Equal(_depositToStakeMethod.ID[:], data[:4]) {
		return nil, errDecodeFailure
	}
	if err := _depositToStakeMethod.Inputs.UnpackIntoMap(paramsMap, data[4:]); err != nil {
		return nil, err
	}
	if ds.bucketIndex, ok = paramsMap["bucketIndex"].(uint64); !ok {
		return nil, errDecodeFailure
	}
	if ds.amount, ok = paramsMap["amount"].(*big.Int); !ok {
		return nil, errDecodeFailure
	}
	if ds.payload, ok = paramsMap["data"].([]byte); !ok {
		return nil, errDecodeFailure
	}
	return &ds, nil
}
