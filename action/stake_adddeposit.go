// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is didslaimed. This source code is governed by Apache
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
	// DepositToStakePayloadGas represents the DepositToStake payload gas per uint
	DepositToStakePayloadGas = uint64(100)
	// DepositToStakeBaseIntrinsicGas represents the base intrinsic gas for DepositToStake
	DepositToStakeBaseIntrinsicGas = uint64(10000)
)

// DepositToStake defines the action of stake add deposit
type DepositToStake struct {
	AbstractAction

	bucketIndex uint64
	amount      *big.Int
	payload     []byte
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

// Serialize returns a raw byte stream of the Stake Create struct
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
		_, ok := ds.amount.SetString(pbAct.GetAmount(), 10)
		if !ok {
			return errors.New("failed to set proto amount")
		}
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
		return nil, errors.Wrap(err, "failed to get intrinsic gas for the stake creates")
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
