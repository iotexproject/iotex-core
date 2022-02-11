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

	DepositToStakeInterfaceABI = `[
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
	// DepositToStakeMethodID is the method ID of depositToStake Method
	DepositToStakeMethodID [4]byte
	// _depositToStakeInterface is the interface of the abi encoding of stake action
	_depositToStakeInterface abi.ABI
)

// DepositToStake defines the action of stake add deposit
type DepositToStake struct {
	AbstractAction

	bucketIndex uint64
	amount      *big.Int
	payload     []byte
}

func init() {
	var err error
	_depositToStakeInterface, err = abi.JSON(strings.NewReader(DepositToStakeInterfaceABI))
	if err != nil {
		panic(err)
	}
	method, ok := _depositToStakeInterface.Methods["depositToStake"]
	if !ok {
		panic("fail to load the method")
	}
	copy(DepositToStakeMethodID[:], method.ID)
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
		return errors.New("empty action proto to load")
	}

	ds.bucketIndex = pbAct.GetBucketIndex()
	ds.payload = pbAct.GetPayload()
	ds.amount = big.NewInt(0)
	if len(pbAct.GetAmount()) > 0 {
		ds.amount.SetString(pbAct.GetAmount(), 10)
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

// EncodingABIBinary encodes data in abi encoding
func (cs *DepositToStake) EncodingABIBinary() ([]byte, error) {
	return _depositToStakeInterface.Pack("depositToStake", cs.bucketIndex, cs.amount, cs.payload)
}

// DecodingABIBinary decodes data into depositToStake action
func (cs *DepositToStake) DecodingABIBinary(data []byte) error {
	var (
		paramsMap = map[string]interface{}{}
		ok        bool
	)
	// sanity check
	if len(data) <= 4 || !bytes.Equal(DepositToStakeMethodID[:], data[:4]) {
		return errDecodeFailure
	}
	if err := _depositToStakeInterface.Methods["depositToStake"].Inputs.UnpackIntoMap(paramsMap, data[4:]); err != nil {
		return err
	}
	if cs.bucketIndex, ok = paramsMap["bucketIndex"].(uint64); !ok {
		return errDecodeFailure
	}
	if cs.amount, ok = paramsMap["amount"].(*big.Int); !ok {
		return errDecodeFailure
	}
	if cs.payload, ok = paramsMap["data"].([]byte); !ok {
		return errDecodeFailure
	}
	return nil
}
