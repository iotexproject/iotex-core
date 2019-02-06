// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rewarding

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

// SetReward is the action to update the reward amount
type SetReward struct {
	action.AbstractAction
	amount *big.Int
	data   []byte
	t      int
}

// Amount returns the amount to reward
func (s *SetReward) Amount() *big.Int { return s.amount }

// Data returns the additional data
func (s *SetReward) Data() []byte { return s.data }

// RewardType returns the grant reward type
func (s *SetReward) RewardType() int { return s.t }

// ByteStream returns a raw byte stream of a set reward action
func (s *SetReward) ByteStream() []byte {
	return byteutil.Must(proto.Marshal(s.Proto()))
}

// Proto converts a set reward action struct to a set reward action protobuf
func (s *SetReward) Proto() *iproto.SetReward {
	sProto := iproto.SetReward{
		Amount: s.amount.Bytes(),
		Data:   s.data,
	}
	switch s.t {
	case BlockReward:
		sProto.Type = iproto.RewardType_Block
	case EpochReward:
		sProto.Type = iproto.RewardType_Epoch
	}
	return &sProto
}

// LoadProto converts a set block rewarding reward action protobuf to a set reward action struct
func (s *SetReward) LoadProto(sProto *iproto.SetReward) error {
	*s = SetReward{}
	s.amount = big.NewInt(0).SetBytes(sProto.Amount)
	s.data = sProto.Data
	switch sProto.Type {
	case iproto.RewardType_Block:
		s.t = BlockReward
	case iproto.RewardType_Epoch:
		s.t = EpochReward
	}
	return nil
}

// IntrinsicGas returns the intrinsic gas of a set reward action
func (s *SetReward) IntrinsicGas() (uint64, error) {
	dataLen := uint64(len(s.Data()))
	if (math.MaxUint64-setBlockRewardBaseGas)/setBlockRewardGasPerByte < dataLen {
		return 0, action.ErrOutOfGas
	}
	return setBlockRewardBaseGas + setBlockRewardGasPerByte*dataLen, nil
}

// Cost returns the total cost of a set reward action
func (s *SetReward) Cost() (*big.Int, error) {
	intrinsicGas, err := s.IntrinsicGas()
	if err != nil {
		return nil, errors.Wrap(err, "error when getting intrinsic gas for the set block reward action")
	}
	return big.NewInt(0).Mul(s.GasPrice(), big.NewInt(0).SetUint64(intrinsicGas)), nil
}

// SetRewardBuilder is the struct to build SetReward
type SetRewardBuilder struct {
	action.Builder
	setReward SetReward
}

// SetAmount sets the amount to reward
func (b *SetRewardBuilder) SetAmount(amount *big.Int) *SetRewardBuilder {
	b.setReward.amount = amount
	return b
}

// SetData sets the additional data
func (b *SetRewardBuilder) SetData(data []byte) *SetRewardBuilder {
	b.setReward.data = data
	return b
}

// SetRewardType sets the grant reward type
func (b *SetRewardBuilder) SetRewardType(t int) *SetRewardBuilder {
	b.setReward.t = t
	return b
}

// Build builds a new set reward action
func (b *SetRewardBuilder) Build() SetReward {
	b.setReward.AbstractAction = b.Builder.Build()
	return b.setReward
}
