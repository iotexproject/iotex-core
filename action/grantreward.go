// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
)

var (
	_grantRewardMethod abi.Method
	_                  EthCompatibleAction = (*GrantReward)(nil)
)

const (
	// BlockReward indicates that the action is to grant block reward
	BlockReward = iota
	// EpochReward indicates that the action is to grant epoch reward
	EpochReward

	_grantrewardInterfaceABI = `[
		{
			"inputs": [
				{
					"internalType": "int8",
					"name": "rewardType",
					"type": "int8"
				},
				{
					"internalType": "uint64",
					"name": "height",
					"type": "uint64"
				}
			],
			"name": "grantReward",
			"outputs": [],
			"stateMutability": "nonpayable",
			"type": "function"
		}
	]`
)

func init() {
	grantRewardInterface, err := abi.JSON(strings.NewReader(_grantrewardInterfaceABI))
	if err != nil {
		panic(err)
	}
	var ok bool
	_grantRewardMethod, ok = grantRewardInterface.Methods["grantReward"]
	if !ok {
		panic("fail to load the method")
	}
}

// GrantReward is the action to grant either block or epoch reward
type GrantReward struct {
	reward_common
	rewardType int
	height     uint64
}

func NewGrantReward(rewardType int, height uint64) *GrantReward {
	return &GrantReward{
		rewardType: rewardType,
		height:     height,
	}
}

// RewardType returns the grant reward type
func (g *GrantReward) RewardType() int { return g.rewardType }

// Height returns the block height to grant reward
func (g *GrantReward) Height() uint64 { return g.height }

// Serialize returns a raw byte stream of a grant reward action
func (g *GrantReward) Serialize() []byte {
	return byteutil.Must(proto.Marshal(g.Proto()))
}

func (act *GrantReward) FillAction(core *iotextypes.ActionCore) {
	core.Action = &iotextypes.ActionCore_GrantReward{GrantReward: act.Proto()}
}

// Proto converts a grant reward action struct to a grant reward action protobuf
func (g *GrantReward) Proto() *iotextypes.GrantReward {
	gProto := iotextypes.GrantReward{
		Height: g.height,
	}
	switch g.rewardType {
	case BlockReward:
		gProto.Type = iotextypes.RewardType_BlockReward
	case EpochReward:
		gProto.Type = iotextypes.RewardType_EpochReward
	}
	return &gProto
}

// LoadProto converts a grant reward action protobuf to a grant reward action struct
func (g *GrantReward) LoadProto(gProto *iotextypes.GrantReward) error {
	*g = GrantReward{
		height: gProto.Height,
	}
	switch gProto.Type {
	case iotextypes.RewardType_BlockReward:
		g.rewardType = BlockReward
	case iotextypes.RewardType_EpochReward:
		g.rewardType = EpochReward
	}
	return nil
}

// IntrinsicGas returns the intrinsic gas of a grant reward action, which is 0
func (*GrantReward) IntrinsicGas() (uint64, error) {
	return 0, nil
}

func (*GrantReward) SanityCheck() error { return nil }

// EthData returns the ABI-encoded data for converting to eth tx
func (g *GrantReward) EthData() ([]byte, error) {
	data, err := _grantRewardMethod.Inputs.Pack(
		int8(g.rewardType),
		g.height,
	)
	if err != nil {
		return nil, err
	}
	return append(_grantRewardMethod.ID, data...), nil
}
