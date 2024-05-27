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

	"github.com/iotexproject/iotex-core/pkg/util/assertions"
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

func TestBuildRewardingAction(t *testing.T) {
	r := require.New(t)

	eb := &EnvelopeBuilder{}
	eb.SetChainID(2)

	t.Run("UnexpectToAddress", func(t *testing.T) {
		to := common.HexToAddress("0x0000000000000000000000000000000000000001")
		calldata := []byte("any")

		elp, err := eb.BuildRewardingAction(types.NewTx(&types.LegacyTx{
			Nonce:    1,
			GasPrice: big.NewInt(1),
			Gas:      1,
			To:       &to,
			Value:    big.NewInt(1),
			Data:     calldata,
		}))
		r.Nil(elp)
		r.ErrorIs(err, ErrInvalidAct)
	})

	t.Run("InvalidMethodSignature", func(t *testing.T) {
		// method signature should in ('claim', 'deposit')
		to := _rewardingProtocolEthAddr
		elp, err := eb.BuildRewardingAction(types.NewTx(&types.LegacyTx{
			Nonce:    1,
			GasPrice: big.NewInt(1),
			Gas:      1,
			To:       &to,
			Value:    big.NewInt(1),
			Data:     []byte("InvalidMethodSig"),
		}))
		r.Nil(elp)
		r.ErrorIs(err, ErrInvalidABI)
	})

	t.Run("Success", func(t *testing.T) {
		method := _claimRewardingMethod
		t.Run("ClaimRewarding", func(t *testing.T) {
			inputs := assertions.MustNoErrorV(method.Inputs.Pack(big.NewInt(101), []byte("any"), ""))
			elp, err := eb.BuildRewardingAction(types.NewTx(&types.LegacyTx{
				Nonce:    1,
				GasPrice: big.NewInt(10004),
				Gas:      10000,
				To:       &_rewardingProtocolEthAddr,
				Value:    big.NewInt(100),
				Data:     append(method.ID, inputs...),
			}))
			r.NoError(err)
			r.IsType(&ClaimFromRewardingFund{}, elp.Action())
			r.Equal(big.NewInt(10004), elp.GasPrice())
			r.Equal(uint64(10000), elp.GasLimit())
			r.Equal(big.NewInt(101), elp.Action().(*ClaimFromRewardingFund).Amount())

		})
		t.Run("Debug", func(t *testing.T) {
			eb := &EnvelopeBuilder{}
			eb.SetChainID(4689)

			inputs := assertions.MustNoErrorV(method.Inputs.Pack(big.NewInt(100), []byte("any"), ""))
			to := common.HexToAddress("0xA576C141e5659137ddDa4223d209d4744b2106BE")
			tx := types.NewTx(&types.LegacyTx{
				Nonce:    0,
				GasPrice: big.NewInt(100),
				Gas:      21000,
				To:       &to,
				Value:    big.NewInt(0),
				Data:     append(method.ID, inputs...),
			})
			t.Log(tx.Hash().String())
			raw, err := tx.MarshalBinary()
			r.NoError(err)
			t.Log(hex.EncodeToString(raw))
		})
		method = _depositRewardMethod
		t.Run("DepositRewarding", func(t *testing.T) {
			inputs := assertions.MustNoErrorV(method.Inputs.Pack(big.NewInt(102), []byte("any")))
			elp, err := eb.BuildRewardingAction(types.NewTx(&types.LegacyTx{
				Nonce:    1,
				GasPrice: big.NewInt(10004),
				Gas:      10000,
				To:       &_rewardingProtocolEthAddr,
				Value:    big.NewInt(100),
				Data:     append(method.ID, inputs...),
			}))
			r.NoError(err)
			r.IsType(&DepositToRewardingFund{}, elp.Action())
			r.EqualValues(big.NewInt(10004), elp.GasPrice())
			r.EqualValues(10000, elp.GasLimit())
			r.EqualValues(big.NewInt(102), elp.Action().(*DepositToRewardingFund).Amount())
		})
	})
}
