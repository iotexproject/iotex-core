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
	setBlockRewardBaseGas    = uint64(10000)
	setBlockRewardGasPerByte = uint64(100)
)

// SetBlockReward is the action to update the per block reward amount
type SetBlockReward struct {
	action.AbstractAction
	amount *big.Int
	data   []byte
}

// Amount returns the amount to set block reward
func (c *SetBlockReward) Amount() *big.Int { return c.amount }

// Data returns the additional data
func (c *SetBlockReward) Data() []byte { return c.data }

// ByteStream returns a raw byte stream of a set block reward action
func (c *SetBlockReward) ByteStream() []byte {
	return byteutil.Must(proto.Marshal(c.Proto()))
}

// Proto converts a set block reward action struct to a set block reward action protobuf
func (c *SetBlockReward) Proto() *iproto.SetBlockReward {
	return &iproto.SetBlockReward{
		Amount: c.amount.Bytes(),
		Data:   c.data,
	}
}

// LoadProto converts a set block reward action protobuf to a set block reward action struct
func (c *SetBlockReward) LoadProto(setBlockReward *iproto.SetBlockReward) error {
	*c = SetBlockReward{}
	c.amount = big.NewInt(0).SetBytes(setBlockReward.Amount)
	c.data = setBlockReward.Data
	return nil
}

// IntrinsicGas returns the intrinsic gas of a set block reward action
func (c *SetBlockReward) IntrinsicGas() (uint64, error) {
	dataLen := uint64(len(c.Data()))
	if (math.MaxUint64-setBlockRewardBaseGas)/setBlockRewardGasPerByte < dataLen {
		return 0, action.ErrOutOfGas
	}
	return setBlockRewardBaseGas + setBlockRewardGasPerByte*dataLen, nil
}

// Cost returns the total cost of a set block reward action
func (c *SetBlockReward) Cost() (*big.Int, error) {
	intrinsicGas, err := c.IntrinsicGas()
	if err != nil {
		return nil, errors.Wrap(err, "error when getting intrinsic gas for the set block reward action")
	}
	return big.NewInt(0).Mul(c.GasPrice(), big.NewInt(0).SetUint64(intrinsicGas)), nil
}

// SetBlockRewardBuilder is the struct to build SetBlockReward
type SetBlockRewardBuilder struct {
	action.Builder
	setBlockReward SetBlockReward
}

// SetAmount sets the amount to set block reward
func (b *SetBlockRewardBuilder) SetAmount(amount *big.Int) *SetBlockRewardBuilder {
	b.setBlockReward.amount = amount
	return b
}

// SetData sets the additional data
func (b *SetBlockRewardBuilder) SetData(data []byte) *SetBlockRewardBuilder {
	b.setBlockReward.data = data
	return b
}

// Build builds a new set block reward action
func (b *SetBlockRewardBuilder) Build() SetBlockReward {
	b.setBlockReward.AbstractAction = b.Builder.Build()
	return b.setBlockReward
}
