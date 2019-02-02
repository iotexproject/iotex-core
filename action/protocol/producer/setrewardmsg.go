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

// SetBlockProducerReward is the action to update the per block reward amount
type SetBlockProducerReward struct {
	action.AbstractAction
	amount *big.Int
	data   []byte
	t      int
}

// Amount returns the amount to set block producer reward
func (s *SetBlockProducerReward) Amount() *big.Int { return s.amount }

// Data returns the additional data
func (s *SetBlockProducerReward) Data() []byte { return s.data }

// RewardType returns the grant reward type
func (s *SetBlockProducerReward) RewardType() int { return s.t }

// ByteStream returns a raw byte stream of a set block producer reward action
func (s *SetBlockProducerReward) ByteStream() []byte {
	return byteutil.Must(proto.Marshal(s.Proto()))
}

// Proto converts a set block producer reward action struct to a set block producer reward action protobuf
func (s *SetBlockProducerReward) Proto() *iproto.SetBlockProducerReward {
	sProto := iproto.SetBlockProducerReward{
		Amount: s.amount.Bytes(),
		Data:   s.data,
	}
	switch s.t {
	case BlockReward:
		sProto.Type = iproto.BlockProducerRewardType_Block
	case EpochReward:
		sProto.Type = iproto.BlockProducerRewardType_Epoch
	}
	return &sProto
}

// LoadProto converts a set block producer reward action protobuf to a set block producer reward action struct
func (s *SetBlockProducerReward) LoadProto(sProto *iproto.SetBlockProducerReward) error {
	*s = SetBlockProducerReward{}
	s.amount = big.NewInt(0).SetBytes(sProto.Amount)
	s.data = sProto.Data
	switch sProto.Type {
	case iproto.BlockProducerRewardType_Block:
		s.t = BlockReward
	case iproto.BlockProducerRewardType_Epoch:
		s.t = EpochReward
	}
	return nil
}

// IntrinsicGas returns the intrinsic gas of a set block producer reward action
func (c *SetBlockProducerReward) IntrinsicGas() (uint64, error) {
	dataLen := uint64(len(c.Data()))
	if (math.MaxUint64-setBlockRewardBaseGas)/setBlockRewardGasPerByte < dataLen {
		return 0, action.ErrOutOfGas
	}
	return setBlockRewardBaseGas + setBlockRewardGasPerByte*dataLen, nil
}

// Cost returns the total cost of a set block producer reward action
func (c *SetBlockProducerReward) Cost() (*big.Int, error) {
	intrinsicGas, err := c.IntrinsicGas()
	if err != nil {
		return nil, errors.Wrap(err, "error when getting intrinsic gas for the set block reward action")
	}
	return big.NewInt(0).Mul(c.GasPrice(), big.NewInt(0).SetUint64(intrinsicGas)), nil
}

// SetBlockProducerRewardBuilder is the struct to build SetBlockProducerReward
type SetBlockProducerRewardBuilder struct {
	action.Builder
	setReward SetBlockProducerReward
}

// SetAmount sets the amount to set block producer reward
func (b *SetBlockProducerRewardBuilder) SetAmount(amount *big.Int) *SetBlockProducerRewardBuilder {
	b.setReward.amount = amount
	return b
}

// SetData sets the additional data
func (b *SetBlockProducerRewardBuilder) SetData(data []byte) *SetBlockProducerRewardBuilder {
	b.setReward.data = data
	return b
}

// SetRewardType sets the grant reward type
func (b *SetBlockProducerRewardBuilder) SetRewardType(t int) *SetBlockProducerRewardBuilder {
	b.setReward.t = t
	return b
}

// Build builds a new set block producer reward action
func (b *SetBlockProducerRewardBuilder) Build() SetBlockProducerReward {
	b.setReward.AbstractAction = b.Builder.Build()
	return b.setReward
}
