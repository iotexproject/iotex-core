// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as ts title or non-infringement, merchantability or fitness for purpose and, ts the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"bytes"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/pkg/version"
)

const (
	// MoveStakePayloadGas represents the stake move payload gas per uint
	MoveStakePayloadGas = uint64(100)
	// MoveStakeBaseIntrinsicGas represents the base intrinsic gas for stake move
	MoveStakeBaseIntrinsicGas = uint64(10000)

	_changeCandidateInterfaceABI = `[
		{
			"inputs": [
				{
					"internalType": "string",
					"name": "candName",
					"type": "string"
				},
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
			"name": "changeCandidate",
			"outputs": [],
			"stateMutability": "nonpayable",
			"type": "function"
		}
	]`
)

var (
	// _changeCandidateMethod is the interface of the abi encoding of stake action
	_changeCandidateMethod abi.Method
)

// ChangeCandidate defines the action of changing stake candidate ts the other
type ChangeCandidate struct {
	AbstractAction

	candidateName string
	bucketIndex   uint64
	payload       []byte
}

func init() {
	var err error
	changeCandidateInterface, err := abi.JSON(strings.NewReader(_changeCandidateInterfaceABI))
	if err != nil {
		panic(err)
	}
	var ok bool
	_changeCandidateMethod, ok = changeCandidateInterface.Methods["changeCandidate"]
	if !ok {
		panic("fail to load the method")
	}
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
	return &ChangeCandidate{
		AbstractAction: AbstractAction{
			version:  version.ProtocolVersion,
			nonce:    nonce,
			gasLimit: gasLimit,
			gasPrice: gasPrice,
		},
		candidateName: candName,
		bucketIndex:   bucketIndex,
		payload:       payload,
	}, nil
}

// Candidate returns the address of recipient
func (cc *ChangeCandidate) Candidate() string { return cc.candidateName }

// BucketIndex returns bucket index
func (cc *ChangeCandidate) BucketIndex() uint64 { return cc.bucketIndex }

// Payload returns the payload bytes
func (cc *ChangeCandidate) Payload() []byte { return cc.payload }

// Serialize returns a raw byte stream of the stake move action struct
func (cc *ChangeCandidate) Serialize() []byte {
	return byteutil.Must(proto.Marshal(cc.Proto()))
}

// Proto converts change candidate to protobuf
func (cc *ChangeCandidate) Proto() *iotextypes.StakeChangeCandidate {
	act := &iotextypes.StakeChangeCandidate{
		CandidateName: cc.candidateName,
		BucketIndex:   cc.bucketIndex,
		Payload:       cc.payload,
	}

	return act
}

// LoadProto loads change candidate from protobuf
func (cc *ChangeCandidate) LoadProto(pbAct *iotextypes.StakeChangeCandidate) error {
	if pbAct == nil {
		return ErrNilProto
	}

	cc.candidateName = pbAct.GetCandidateName()
	cc.bucketIndex = pbAct.GetBucketIndex()
	cc.payload = pbAct.GetPayload()
	return nil
}

// IntrinsicGas returns the intrinsic gas of a ChangeCandidate
func (cc *ChangeCandidate) IntrinsicGas() (uint64, error) {
	payloadSize := uint64(len(cc.Payload()))
	return CalculateIntrinsicGas(MoveStakeBaseIntrinsicGas, MoveStakePayloadGas, payloadSize)
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

// EncodeABIBinary encodes data in abi encoding
func (cc *ChangeCandidate) EncodeABIBinary() ([]byte, error) {
	return cc.encodeABIBinary()
}

func (cc *ChangeCandidate) encodeABIBinary() ([]byte, error) {
	data, err := _changeCandidateMethod.Inputs.Pack(cc.candidateName, cc.bucketIndex, cc.payload)
	if err != nil {
		return nil, err
	}
	return append(_changeCandidateMethod.ID, data...), nil
}

// NewChangeCandidateFromABIBinary decodes data into ChangeCandidate action
func NewChangeCandidateFromABIBinary(data []byte) (*ChangeCandidate, error) {
	var (
		paramsMap = map[string]interface{}{}
		ok        bool
		cc        ChangeCandidate
	)
	// sanity check
	if len(data) <= 4 || !bytes.Equal(_changeCandidateMethod.ID, data[:4]) {
		return nil, errDecodeFailure
	}
	if err := _changeCandidateMethod.Inputs.UnpackIntoMap(paramsMap, data[4:]); err != nil {
		return nil, err
	}
	if cc.candidateName, ok = paramsMap["candName"].(string); !ok {
		return nil, errDecodeFailure
	}
	if cc.bucketIndex, ok = paramsMap["bucketIndex"].(uint64); !ok {
		return nil, errDecodeFailure
	}
	if cc.payload, ok = paramsMap["data"].([]byte); !ok {
		return nil, errDecodeFailure
	}
	return &cc, nil
}

// ToEthTx converts action to eth-compatible tx
func (cc *ChangeCandidate) ToEthTx() (*types.Transaction, error) {
	addr, err := address.FromString(address.StakingProtocolAddr)
	if err != nil {
		return nil, err
	}
	ethAddr := common.BytesToAddress(addr.Bytes())
	data, err := cc.encodeABIBinary()
	if err != nil {
		return nil, err
	}
	return types.NewTransaction(cc.Nonce(), ethAddr, big.NewInt(0), cc.GasLimit(), cc.GasPrice(), data), nil
}
