// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"

	"github.com/golang/protobuf/proto"

	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/protogen/iotextypes"
)

const (
	// BlockReward indicates that the action is to grant block reward
	BlockReward = iota
	// EpochReward indicates that the action is to grant epoch reward
	EpochReward
)

// GrantReward is the action to grant either block or epoch reward
type GrantReward struct {
	AbstractAction

	t int
}

// RewardType returns the grant reward type
func (g *GrantReward) RewardType() int { return g.t }

// ByteStream returns a raw byte stream of a grant reward action
func (g *GrantReward) ByteStream() []byte {
	return byteutil.Must(proto.Marshal(g.Proto()))
}

// Proto converts a grant reward action struct to a grant reward action protobuf
func (g *GrantReward) Proto() *iotextypes.GrantReward {
	gProto := iotextypes.GrantReward{}
	switch g.t {
	case BlockReward:
		gProto.Type = iotextypes.RewardType_BlockReward
	case EpochReward:
		gProto.Type = iotextypes.RewardType_EpochReward
	}
	return &gProto
}

// LoadProto converts a grant reward action protobuf to a grant reward action struct
func (g *GrantReward) LoadProto(gProto *iotextypes.GrantReward) error {
	*g = GrantReward{}
	switch gProto.Type {
	case iotextypes.RewardType_BlockReward:
		g.t = BlockReward
	case iotextypes.RewardType_EpochReward:
		g.t = EpochReward
	}
	return nil
}

// IntrinsicGas returns the intrinsic gas of a grant reward action, which is 0
func (*GrantReward) IntrinsicGas() (uint64, error) {
	return 0, nil
}

// Cost returns the total cost of a grant reward action
func (*GrantReward) Cost() (*big.Int, error) {
	return big.NewInt(0), nil
}

// GrantRewardBuilder is the struct to build GrantReward
type GrantRewardBuilder struct {
	Builder
	grantReward GrantReward
}

// SetRewardType sets the grant reward type
func (b *GrantRewardBuilder) SetRewardType(t int) *GrantRewardBuilder {
	b.grantReward.t = t
	return b
}

// Build builds a new grant reward action
func (b *GrantRewardBuilder) Build() GrantReward {
	b.grantReward.AbstractAction = b.Builder.Build()
	return b.grantReward
}
