// Copyright (c) 2020 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

var (
	_duration  = uint32(1000)
	_autoStake = true
)

func TestRestake(t *testing.T) {
	require := require.New(t)
	stake := NewRestake(_index, _duration, _autoStake, _payload)
	elp := (&EnvelopeBuilder{}).SetNonce(_nonce).SetGasLimit(_gasLimit).
		SetGasPrice(_gasPrice).SetAction(stake).Build()
	t.Run("proto", func(t *testing.T) {
		ser := stake.Serialize()
		require.Equal("080a10e807180122077061796c6f6164", hex.EncodeToString(ser))
		require.Equal(_gaslimit, elp.Gas())
		require.Equal(_gasprice, elp.GasPrice())
		require.Equal(_nonce, elp.Nonce())

		require.True(stake.AutoStake())
		require.Equal(_payload, stake.Payload())
		require.Equal(_duration, stake.Duration())
		require.Equal(_index, stake.BucketIndex())

		gas, err := stake.IntrinsicGas()
		require.NoError(err)
		require.Equal(uint64(10700), gas)
		cost, err := elp.Cost()
		require.NoError(err)
		require.Equal("107000", cost.Text(10))

		proto := stake.Proto()
		stake2 := &Restake{}
		require.NoError(stake2.LoadProto(proto))
		require.True(stake2.AutoStake())
		require.Equal(_payload, stake2.Payload())
		require.Equal(_duration, stake2.Duration())
		require.Equal(_index, stake2.BucketIndex())
	})
	t.Run("sign and verify", func(t *testing.T) {
		selp, err := Sign(elp, _senderKey)
		require.NoError(err)
		ser, err := proto.Marshal(selp.Proto())
		require.NoError(err)
		require.Equal("0a1f0801100118c0843d22023130e20210080a10e807180122077061796c6f6164124104755ce6d8903f6b3793bddb4ea5d3589d637de2d209ae0ea930815c82db564ee8cc448886f639e8a0c7e94e99a5c1335b583c0bc76ef30dd6a1038ed9da8daf331a414dfefb022b2e097726cffbdc3bf5b26d907f915fa563c9730ca883960caa8c6e1c7aab8ed7103138bf1e614e2da570e771a546942994470632ad2b8664adc39100", hex.EncodeToString(ser))
		hash, err := selp.Hash()
		require.NoError(err)
		require.Equal("fbb91e899121edb2ba62e06e6b0d98669d7e29242875e8468f52e39a2c147e4a", hex.EncodeToString(hash[:]))
		// verify signature
		require.NoError(selp.VerifySignature())
	})
	t.Run("ABI encode", func(t *testing.T) {
		data, err := stake.EthData()
		require.NoError(err)
		stake, err = NewRestakeFromABIBinary(data)
		require.NoError(err)
		require.Equal(_index, stake.bucketIndex)
		require.Equal(_duration, stake.Duration())
		require.Equal(_autoStake, stake.AutoStake())
		require.Equal(_payload, stake.payload)
	})
}
