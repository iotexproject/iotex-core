// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/pkg/version"
)

func TestActionBuilder(t *testing.T) {
	bd := &Builder{}
	act := bd.SetVersion(version.ProtocolVersion).
		SetNonce(2).
		SetGasLimit(10003).
		SetGasPrice(big.NewInt(10004)).
		Build()

	assert.Equal(t, uint32(version.ProtocolVersion), act.Version())
	assert.Equal(t, uint64(2), act.Nonce())
	assert.Equal(t, uint64(10003), act.GasLimit())
	assert.Equal(t, big.NewInt(10004), act.GasPrice())
}

func TestBuildRewardingAction(t *testing.T) {
	r := require.New(t)

	eb := &EnvelopeBuilder{}
	eb.SetChainID(2)

	claimData, _ := hex.DecodeString("2df163ef000000000000000000000000000000000000000000000000000000000000006500000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000000")
	tx := types.NewTransaction(1, common.HexToAddress("0x0000000000000000000000000000000000000001"), big.NewInt(100), 10000, big.NewInt(10004), claimData)

	env, err := eb.BuildRewardingAction(tx)
	r.Nil(env)
	r.EqualValues("invalid action type", err.Error())

	tx = types.NewTransaction(1, common.HexToAddress(_rewardingProtocolAddr.Hex()), big.NewInt(100), 10000, big.NewInt(10004), claimData)
	env, err = eb.BuildRewardingAction(tx)
	r.Nil(err)
	r.IsType(&RewardingClaim{}, env.Action())
	r.EqualValues(big.NewInt(10004), env.GasPrice())
	r.EqualValues(10000, env.GasLimit())
	r.EqualValues(big.NewInt(101), env.Action().(*RewardingClaim).Amount())

	depositData, _ := hex.DecodeString("27852a6b000000000000000000000000000000000000000000000000000000000000006500000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000003000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000003")
	tx = types.NewTransaction(1, common.HexToAddress("0x0000000000000000000000000000000000000001"), big.NewInt(100), 10000, big.NewInt(10004), depositData)

	env, err = eb.BuildRewardingAction(tx)
	r.Nil(env)
	r.EqualValues("invalid action type", err.Error())

	tx = types.NewTransaction(1, common.HexToAddress(_rewardingProtocolAddr.Hex()), big.NewInt(100), 10000, big.NewInt(10004), depositData)
	env, err = eb.BuildRewardingAction(tx)
	r.Nil(err)
	r.IsType(&RewardingDeposit{}, env.Action())
	r.EqualValues(big.NewInt(10004), env.GasPrice())
	r.EqualValues(10000, env.GasLimit())
	r.EqualValues(big.NewInt(101), env.Action().(*RewardingDeposit).Amount())
}
