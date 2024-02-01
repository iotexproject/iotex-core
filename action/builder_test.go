// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/pkg/version"
)

func TestBuilderEthAddr(t *testing.T) {
	r := require.New(t)

	r.Equal(address.StakingProtocolAddrHash[:], _stakingProtocolEthAddr.Bytes())
	r.Equal(address.RewardingProtocolAddrHash[:], _rewardingProtocolEthAddr.Bytes())
}

func TestActionBuilder(t *testing.T) {
	r := require.New(t)

	bd := &Builder{}
	act := bd.SetVersion(version.ProtocolVersion).
		SetNonce(2).
		SetGasLimit(10003).
		SetGasPrice(big.NewInt(10004)).
		Build()

	r.Equal(uint32(version.ProtocolVersion), act.Version())
	r.Equal(uint64(2), act.Nonce())
	r.Equal(uint64(10003), act.GasLimit())
	r.Equal(big.NewInt(10004), act.GasPrice())
}

func TestBuildExecution(t *testing.T) {
	r := require.New(t)

	for _, v := range []struct {
		inner types.TxData
		acl   bool
	}{
		{&types.LegacyTx{
			Nonce:    1,
			GasPrice: nil,
			Gas:      10000,
			To:       nil,
			Value:    nil,
			Data:     nil,
		}, false},
		{&types.AccessListTx{
			ChainID:  big.NewInt(333),
			Nonce:    2,
			GasPrice: big.NewInt(0),
			Gas:      20000,
			To:       &common.Address{},
			Value:    big.NewInt(0),
			Data:     nil,
		}, true},
	} {
		tx := types.NewTx(v.inner)
		elp, err := (&EnvelopeBuilder{}).BuildExecution(tx)
		r.NoError(err)
		r.Equal(v.acl, elp.Action().(*Execution).IsAccessList())
		tx1, err := elp.Action().(EthCompatibleAction).ToEthTx(uint32(tx.ChainId().Uint64()))
		r.NoError(err)
		if v.acl {
			r.EqualValues(types.AccessListTxType, tx1.Type())
		} else {
			r.EqualValues(types.LegacyTxType, tx1.Type())
		}
		b, err := tx.MarshalJSON()
		r.NoError(err)
		b1, err := tx1.MarshalJSON()
		r.NoError(err)
		r.Equal(b, b1)
	}
}

func TestBuildRewardingAction(t *testing.T) {
	r := require.New(t)

	eb := &EnvelopeBuilder{}
	eb.SetChainID(2)

	claimData, _ := hex.DecodeString("2df163ef000000000000000000000000000000000000000000000000000000000000006500000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000000")
	to := common.HexToAddress("0x0000000000000000000000000000000000000001")
	env, err := eb.BuildRewardingAction(types.NewTx(&types.LegacyTx{
		Nonce:    1,
		GasPrice: big.NewInt(10004),
		Gas:      10000,
		To:       &to,
		Value:    big.NewInt(100),
		Data:     claimData,
	}))
	r.Nil(env)
	r.EqualValues("invalid action type", err.Error())

	env, err = eb.BuildRewardingAction(types.NewTx(&types.LegacyTx{
		Nonce:    1,
		GasPrice: big.NewInt(10004),
		Gas:      10000,
		To:       &_rewardingProtocolEthAddr,
		Value:    big.NewInt(100),
		Data:     claimData,
	}))
	r.Nil(err)
	r.IsType(&ClaimFromRewardingFund{}, env.Action())
	r.EqualValues(big.NewInt(10004), env.GasPrice())
	r.EqualValues(10000, env.GasLimit())
	r.EqualValues(big.NewInt(101), env.Action().(*ClaimFromRewardingFund).Amount())

	depositData, _ := hex.DecodeString("27852a6b000000000000000000000000000000000000000000000000000000000000006500000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000003000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000003")
	to = common.HexToAddress("0x0000000000000000000000000000000000000001")
	env, err = eb.BuildRewardingAction(types.NewTx(&types.LegacyTx{
		Nonce:    1,
		GasPrice: big.NewInt(10004),
		Gas:      10000,
		To:       &to,
		Value:    big.NewInt(100),
		Data:     depositData,
	}))
	r.Nil(env)
	r.EqualValues("invalid action type", err.Error())

	env, err = eb.BuildRewardingAction(types.NewTx(&types.LegacyTx{
		Nonce:    1,
		GasPrice: big.NewInt(10004),
		Gas:      10000,
		To:       &_rewardingProtocolEthAddr,
		Value:    big.NewInt(100),
		Data:     depositData,
	}))
	r.Nil(err)
	r.IsType(&DepositToRewardingFund{}, env.Action())
	r.EqualValues(big.NewInt(10004), env.GasPrice())
	r.EqualValues(10000, env.GasLimit())
	r.EqualValues(big.NewInt(101), env.Action().(*DepositToRewardingFund).Amount())
}
