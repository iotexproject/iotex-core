// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package producer

import (
	"math/big"

	"github.com/golang/protobuf/proto"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/proto"
)

// GrantBlockProducerReward is the action to grant either block or epoch reward
type GrantBlockProducerReward struct {
	action.AbstractAction
	t int
}

// RewardType returns the grant reward type
func (g *GrantBlockProducerReward) RewardType() int { return g.t }

// ByteStream returns a raw byte stream of a grant reward action
func (g *GrantBlockProducerReward) ByteStream() []byte {
	return byteutil.Must(proto.Marshal(g.Proto()))
}

// Proto converts a grant reward action struct to a grant reward action protobuf
func (g *GrantBlockProducerReward) Proto() *iproto.GrantBlockProducerReward {
	gProto := iproto.GrantBlockProducerReward{}
	switch g.t {
	case BlockReward:
		gProto.Type = iproto.BlockProducerRewardType_Block
	case EpochReward:
		gProto.Type = iproto.BlockProducerRewardType_Epoch
	}
	return &gProto
}

// LoadProto converts a grant reward action protobuf to a grant reward action struct
func (g *GrantBlockProducerReward) LoadProto(gProto *iproto.GrantBlockProducerReward) error {
	*g = GrantBlockProducerReward{}
	switch gProto.Type {
	case iproto.BlockProducerRewardType_Block:
		g.t = BlockReward
	case iproto.BlockProducerRewardType_Epoch:
		g.t = EpochReward
	}
	return nil
}

// IntrinsicGas returns the intrinsic gas of a grant reward action, which is 0
func (*GrantBlockProducerReward) IntrinsicGas() (uint64, error) {
	return 0, nil
}

// Cost returns the total cost of a grant reward action
func (*GrantBlockProducerReward) Cost() (*big.Int, error) {
	return big.NewInt(0), nil
}

// GrantBlockProducerRewardBuilder is the struct to build GrantBlockProducerReward
type GrantBlockProducerRewardBuilder struct {
	action.Builder
	grantReward GrantBlockProducerReward
}

// SetRewardType sets the grant reward type
func (b *GrantBlockProducerRewardBuilder) SetRewardType(t int) *GrantBlockProducerRewardBuilder {
	b.grantReward.t = t
	return b
}

// Build builds a new grant reward action
func (b *GrantBlockProducerRewardBuilder) Build() GrantBlockProducerReward {
	b.grantReward.AbstractAction = b.Builder.Build()
	return b.grantReward
}
