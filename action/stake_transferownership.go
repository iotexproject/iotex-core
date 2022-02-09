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

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/pkg/version"
)

// TransferStake defines the action of transfering stake ownership ts the other
type TransferStake struct {
	AbstractAction

	voterAddress address.Address
	bucketIndex  uint64
	payload      []byte
}

// NewTransferStake returns a TransferStake instance
func NewTransferStake(
	nonce uint64,
	voterAddress string,
	bucketIndex uint64,
	payload []byte,
	gasLimit uint64,
	gasPrice *big.Int,
) (*TransferStake, error) {
	voterAddr, err := address.FromString(voterAddress)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load address from string")
	}
	return &TransferStake{
		AbstractAction: AbstractAction{
			version:  version.ProtocolVersion,
			nonce:    nonce,
			gasLimit: gasLimit,
			gasPrice: gasPrice,
		},
		voterAddress: voterAddr,
		bucketIndex:  bucketIndex,
		payload:      payload,
	}, nil
}

// VoterAddress returns the address of recipient
func (ts *TransferStake) VoterAddress() address.Address { return ts.voterAddress }

// BucketIndex returns bucket index
func (ts *TransferStake) BucketIndex() uint64 { return ts.bucketIndex }

// Payload returns the payload bytes
func (ts *TransferStake) Payload() []byte { return ts.payload }

// Serialize returns a raw byte stream of the transfer stake action struct
func (ts *TransferStake) Serialize() []byte {
	return byteutil.Must(proto.Marshal(ts.Proto()))
}

// Proto converts transfer stake to protobuf
func (ts *TransferStake) Proto() *iotextypes.StakeTransferOwnership {
	act := &iotextypes.StakeTransferOwnership{
		VoterAddress: ts.voterAddress.String(),
		BucketIndex:  ts.bucketIndex,
		Payload:      ts.payload,
	}

	return act
}

// LoadProto loads transfer stake protobuf
func (ts *TransferStake) LoadProto(pbAct *iotextypes.StakeTransferOwnership) error {
	if pbAct == nil {
		return errors.New("empty action proto to load")
	}
	voterAddress, err := address.FromString(pbAct.GetVoterAddress())
	if err != nil {
		return errors.Wrap(err, "failed to load address from string")
	}
	ts.voterAddress = voterAddress
	ts.bucketIndex = pbAct.GetBucketIndex()
	ts.payload = pbAct.GetPayload()
	return nil
}

// IntrinsicGas returns the intrinsic gas of a TransferStake
func (ts *TransferStake) IntrinsicGas() (uint64, error) {
	payloadSize := uint64(len(ts.Payload()))
	return CalculateIntrinsicGas(MoveStakeBaseIntrinsicGas, MoveStakePayloadGas, payloadSize)
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
