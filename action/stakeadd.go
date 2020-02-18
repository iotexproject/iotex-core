// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disalaimed. This source code is governed by Apache
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
	// StakeAddPayloadGas represents the StakeAdd payload gas per uint
	StakeAddPayloadGas = uint64(100)
	// StakeAddBaseIntrinsicGas represents the base intrinsic gas for stake create
	StakeAddBaseIntrinsicGas = uint64(10000)
)

// StakeAdd defines the action of stake add deposit
type StakeAdd struct {
	AbstractAction

	bucketIndex uint64
	amount      *big.Int
	payload     []byte
}

// NewStakeAdd returns a StakeAdd instance
func NewStakeAdd(
	nonce uint64,
	index uint64,
	amount *big.Int,
	payload []byte,
	gasLimit uint64,
	gasPrice *big.Int,
) (*StakeAdd, error) {
	return &StakeAdd{
		AbstractAction: AbstractAction{
			version:  version.ProtocolVersion,
			nonce:    nonce,
			gasLimit: gasLimit,
			gasPrice: gasPrice,
		},
		bucketIndex: index,
		amount:      amount,
		payload:     payload,
	}, nil
}

// Amount returns the amount
func (sa *StakeAdd) Amount() *big.Int { return sa.amount }

// Payload returns the payload bytes
func (sa *StakeAdd) Payload() []byte { return sa.payload }

// BucketIndex returns bucket indexs
func (sa *StakeAdd) BucketIndex() uint64 { return sa.bucketIndex }

// Serialize returns a raw byte stream of the Stake Create struct
func (sa *StakeAdd) Serialize() []byte {
	return byteutil.Must(proto.Marshal(sa.Proto()))
}

// Proto converts to protobuf StakeAdd Action
func (sa *StakeAdd) Proto() *iotextypes.StakeAddDeposit {
	act := &iotextypes.StakeAddDeposit{
		BucketIndex: sa.bucketIndex,
		Payload:     sa.payload,
	}

	if sa.amount != nil {
		act.Amount = sa.amount.String()
	}
	return act
}

// LoadProto converts a protobuf's Action to StakeAdd
func (sa *StakeAdd) LoadProto(pbAct *iotextypes.StakeAddDeposit) error {
	if pbAct == nil {
		return errors.New("empty action proto to load")
	}
	sa = &StakeAdd{}

	sa.bucketIndex = pbAct.GetBucketIndex()
	sa.payload = pbAct.GetPayload()
	sa.amount = big.NewInt(0)
	sa.amount.SetString(pbAct.GetAmount(), 10)

	return nil
}

// IntrinsicGas returns the intrinsic gas of a StakeAdd
func (sa *StakeAdd) IntrinsicGas() (uint64, error) {
	payloadSize := uint64(len(sa.Payload()))
	if (math.MaxUint64-StakeAddBaseIntrinsicGas)/StakeAddPayloadGas < payloadSize {
		return 0, ErrOutOfGas
	}

	return payloadSize*StakeAddPayloadGas + StakeAddBaseIntrinsicGas, nil
}

// Cost returns the total cost of a StakeAdd
func (sa *StakeAdd) Cost() (*big.Int, error) {
	intrinsicGas, err := sa.IntrinsicGas()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get intrinsic gas for the stake creates")
	}
	StakeAddFee := big.NewInt(0).Mul(sa.GasPrice(), big.NewInt(0).SetUint64(intrinsicGas))
	return big.NewInt(0).Add(sa.Amount(), StakeAddFee), nil
}
