// Copyright (c) 2020 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

var (
	_gaslimit   = uint64(1000000)
	_gasprice   = big.NewInt(10)
	_canAddress = "io1xpq62aw85uqzrccg9y5hnryv8ld2nkpycc3gza"
	_canName    = "candidate1"
	_payload    = []byte("payload")
	_nonce      = uint64(1)
	_index      = uint64(10)
	_senderKey  = identityset.PrivateKey(27)
)

func TestUnstake(t *testing.T) {
	require := require.New(t)
	stake := NewUnstake(_index, _payload)
	elp := (&EnvelopeBuilder{}).SetNonce(_nonce).SetGasLimit(_gasLimit).
		SetGasPrice(_gasPrice).SetAction(stake).Build()
	t.Run("proto", func(t *testing.T) {
		ser := stake.Serialize()
		require.Equal("080a12077061796c6f6164", hex.EncodeToString(ser))
		require.Equal(_gaslimit, elp.Gas())
		require.Equal(_gasprice, elp.GasPrice())
		require.Equal(_nonce, elp.Nonce())
		require.Equal(_payload, stake.Payload())
		require.Equal(_index, stake.BucketIndex())

		gas, err := stake.IntrinsicGas()
		require.NoError(err)
		require.Equal(uint64(10700), gas)
		cost, err := elp.Cost()
		require.NoError(err)
		require.Equal("107000", cost.Text(10))

		proto := stake.Proto()
		stake2 := &Unstake{}
		require.NoError(stake2.LoadProto(proto))
		require.Equal(_payload, stake2.Payload())
		require.Equal(_index, stake2.BucketIndex())
	})
	t.Run("sign and verify", func(t *testing.T) {
		selp, err := Sign(elp, _senderKey)
		require.NoError(err)
		ser, err := proto.Marshal(selp.Proto())
		require.NoError(err)
		require.Equal("0a1a0801100118c0843d22023130ca020b080a12077061796c6f6164124104755ce6d8903f6b3793bddb4ea5d3589d637de2d209ae0ea930815c82db564ee8cc448886f639e8a0c7e94e99a5c1335b583c0bc76ef30dd6a1038ed9da8daf331a41927fe21027d4ad83216e2c8d03effc9533333b91f82cd3dd4f024fc9f96f91a47cbe0670356b38bd818eeb5bb67d466a3167559df422b8fcfe6b337c720fa1cd00", hex.EncodeToString(ser))
		hash, err := selp.Hash()
		require.NoError(err)
		require.Equal("8ee334679fed69911b72f44b8852356641e85519542b7222e28ea48ecb730683", hex.EncodeToString(hash[:]))
		// verify signature
		require.NoError(selp.VerifySignature())
	})
	t.Run("ABI encode", func(t *testing.T) {
		data, err := stake.EthData()
		require.NoError(err)
		stake, err = NewUnstakeFromABIBinary(data)
		require.NoError(err)
		require.Equal(_index, stake.bucketIndex)
		require.Equal(_payload, stake.payload)
	})
}

func TestWithdraw(t *testing.T) {
	require := require.New(t)
	stake := NewWithdrawStake(_index, _payload)
	elp := (&EnvelopeBuilder{}).SetNonce(_nonce).SetGasLimit(_gasLimit).
		SetGasPrice(_gasPrice).SetAction(stake).Build()
	t.Run("proto", func(t *testing.T) {
		ser := stake.Serialize()
		require.Equal("080a12077061796c6f6164", hex.EncodeToString(ser))
		require.Equal(_gaslimit, elp.Gas())
		require.Equal(_gasprice, elp.GasPrice())
		require.Equal(_nonce, elp.Nonce())
		require.Equal(_payload, stake.Payload())
		require.Equal(_index, stake.BucketIndex())

		gas, err := stake.IntrinsicGas()
		require.NoError(err)
		require.Equal(uint64(10700), gas)
		cost, err := elp.Cost()
		require.NoError(err)
		require.Equal("107000", cost.Text(10))

		proto := stake.Proto()
		stake2 := &WithdrawStake{}
		require.NoError(stake2.LoadProto(proto))
		require.Equal(_payload, stake2.Payload())
		require.Equal(_index, stake2.BucketIndex())
	})
	t.Run("sign and verify", func(t *testing.T) {
		selp, err := Sign(elp, _senderKey)
		require.NoError(err)
		require.NotNil(selp)
		ser, err := proto.Marshal(selp.Proto())
		require.NoError(err)
		require.Equal("0a1a0801100118c0843d22023130d2020b080a12077061796c6f6164124104755ce6d8903f6b3793bddb4ea5d3589d637de2d209ae0ea930815c82db564ee8cc448886f639e8a0c7e94e99a5c1335b583c0bc76ef30dd6a1038ed9da8daf331a41b31d56c1f08aa04075b42290d91efc65c6b005897541fac4b91a5a8280d37c89552fd2fd248cd3676da7a4f329711069f937d1ad5aac94507d6c1c9c470592f100", hex.EncodeToString(ser))
		hash, err := selp.Hash()
		require.NoError(err)
		require.Equal("155fb88e7befdb48f83ea2833e69f430a738a39c5f5c29af673d72d3844aeaa3", hex.EncodeToString(hash[:]))
		// verify signature
		require.NoError(selp.VerifySignature())
	})
	t.Run("ABI encode", func(t *testing.T) {
		data, err := stake.EthData()
		require.NoError(err)
		stake, err = NewWithdrawStakeFromABIBinary(data)
		require.NoError(err)
		require.Equal(_index, stake.bucketIndex)
		require.Equal(_payload, stake.payload)
	})

}
