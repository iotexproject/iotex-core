// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package producer

import (
	"math"
	"math/big"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/proto"
)

var (
	setEpochRewardBaseGas    = uint64(10000)
	setEpochRewardGasPerByte = uint64(100)
)

// SetEpochReward is the action to update the per epoch reward amount
type SetEpochReward struct {
	action.AbstractAction
	amount *big.Int
	data   []byte
}

// Amount returns the amount to set epoch reward
func (c *SetEpochReward) Amount() *big.Int { return c.amount }

// Data returns the additional data
func (c *SetEpochReward) Data() []byte { return c.data }

// ByteStream returns a raw byte stream of a set epoch reward action
func (c *SetEpochReward) ByteStream() []byte {
	return byteutil.Must(proto.Marshal(c.Proto()))
}

// Proto converts a set epoch reward action struct to a set epoch reward action protobuf
func (c *SetEpochReward) Proto() *iproto.SetEpochReward {
	return &iproto.SetEpochReward{
		Amount: c.amount.Bytes(),
		Data:   c.data,
	}
}

// LoadProto converts a set epoch reward action protobuf to a set epoch reward action struct
func (c *SetEpochReward) LoadProto(setEpochReward *iproto.SetEpochReward) error {
	*c = SetEpochReward{}
	c.amount = big.NewInt(0).SetBytes(setEpochReward.Amount)
	c.data = setEpochReward.Data
	return nil
}

// IntrinsicGas returns the intrinsic gas of a set epoch reward action
func (c *SetEpochReward) IntrinsicGas() (uint64, error) {
	dataLen := uint64(len(c.Data()))
	if (math.MaxUint64-setEpochRewardBaseGas)/setEpochRewardGasPerByte < dataLen {
		return 0, action.ErrOutOfGas
	}
	return setEpochRewardBaseGas + setEpochRewardGasPerByte*dataLen, nil
}

// Cost returns the total cost of a set epoch reward action
func (c *SetEpochReward) Cost() (*big.Int, error) {
	intrinsicGas, err := c.IntrinsicGas()
	if err != nil {
		return nil, errors.Wrap(err, "error when getting intrinsic gas for the set epoch reward action")
	}
	return big.NewInt(0).Mul(c.GasPrice(), big.NewInt(0).SetUint64(intrinsicGas)), nil
}

// SetEpochRewardBuilder is the struct to build SetEpochReward
type SetEpochRewardBuilder struct {
	action.Builder
	setEpochReward SetEpochReward
}

// SetAmount sets the amount to set epoch reward
func (b *SetEpochRewardBuilder) SetAmount(amount *big.Int) *SetEpochRewardBuilder {
	b.setEpochReward.amount = amount
	return b
}

// SetData sets the additional data
func (b *SetEpochRewardBuilder) SetData(data []byte) *SetEpochRewardBuilder {
	b.setEpochReward.data = data
	return b
}

// Build builds a new set epoch reward action
func (b *SetEpochRewardBuilder) Build() SetEpochReward {
	b.setEpochReward.AbstractAction = b.Builder.Build()
	return b.setEpochReward
}
