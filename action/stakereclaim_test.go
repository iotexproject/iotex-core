// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/test/identityset"
)

var (
	gaslimit   = uint64(1000000)
	gasprice   = big.NewInt(10)
	canAddress = "io1xpq62aw85uqzrccg9y5hnryv8ld2nkpycc3gza"
	payload    = []byte("payload")
	nonce      = uint64(0)
	index      = uint64(10)
	senderKey  = identityset.PrivateKey(27)
)

func TestUnstake(t *testing.T) {
	require := require.New(t)
	stake, err := NewUnstake(nonce, index, payload, gaslimit, gasprice)
	require.NoError(err)

	ser := stake.Serialize()
	require.Equal("080a12077061796c6f6164", hex.EncodeToString(ser))

	require.NoError(err)
	require.Equal(gaslimit, stake.GasLimit())
	require.Equal(gasprice, stake.GasPrice())
	require.Equal(nonce, stake.Nonce())

	require.Equal(payload, stake.Payload())
	require.Equal(index, stake.BucketIndex())

	gas, err := stake.IntrinsicGas()
	require.NoError(err)
	require.Equal(uint64(10700), gas)
	cost, err := stake.Cost()
	require.NoError(err)
	require.Equal("107000", cost.Text(10))

	proto := stake.Proto()
	stake2 := &Unstake{}
	require.NoError(stake2.LoadProto(proto))
	require.Equal(payload, stake2.Payload())
	require.Equal(index, stake2.BucketIndex())
}

func TestUnstakeSignVerify(t *testing.T) {
	require := require.New(t)
	require.Equal("cfa6ef757dee2e50351620dca002d32b9c090cfda55fb81f37f1d26b273743f1", senderKey.HexString())

	stake, err := NewUnstake(nonce, index, payload, gaslimit, gasprice)
	require.NoError(err)

	bd := &EnvelopeBuilder{}
	elp := bd.SetGasLimit(gaslimit).
		SetGasPrice(gasprice).
		SetAction(stake).Build()
	// sign
	selp, err := Sign(elp, senderKey)
	require.NoError(err)
	require.NotNil(selp)
	ser, err := proto.Marshal(selp.Proto())
	require.NoError(err)
	require.Equal("0a18080118c0843d22023130ca020b080a12077061796c6f6164124104755ce6d8903f6b3793bddb4ea5d3589d637de2d209ae0ea930815c82db564ee8cc448886f639e8a0c7e94e99a5c1335b583c0bc76ef30dd6a1038ed9da8daf331a4100adee39b48e1d3dbbd65298a57c7889709fc4df39987130da306f6997374a184b7e7c232a42f21e89b06e6e7ceab81303c6b7483152d08d19ac829b22eb81e601", hex.EncodeToString(ser))
	hash, err := selp.Hash()
	require.NoError(err)
	require.Equal("bed58b64a6c4e959eca60a86f0b2149ce0e1dd527ac5fd26aef725ebf7c22a7d", hex.EncodeToString(hash[:]))
	// verify signature
	require.NoError(selp.Verify())
}

func TestUnstakeABIEncodeAndDecode(t *testing.T) {
	require := require.New(t)
	stake, err := NewUnstake(nonce, index, payload, gaslimit, gasprice)
	require.NoError(err)

	data, err := stake.EncodeABIBinary()
	require.NoError(err)
	stake, err = NewUnstakeFromABIBinary(data)
	require.NoError(err)
	require.Equal(index, stake.bucketIndex)
	require.Equal(payload, stake.payload)
}

func TestWithdraw(t *testing.T) {
	require := require.New(t)
	stake, err := NewWithdrawStake(nonce, index, payload, gaslimit, gasprice)
	require.NoError(err)

	ser := stake.Serialize()
	require.Equal("080a12077061796c6f6164", hex.EncodeToString(ser))

	require.NoError(err)
	require.Equal(gaslimit, stake.GasLimit())
	require.Equal(gasprice, stake.GasPrice())
	require.Equal(nonce, stake.Nonce())

	require.Equal(payload, stake.Payload())
	require.Equal(index, stake.BucketIndex())

	gas, err := stake.IntrinsicGas()
	require.NoError(err)
	require.Equal(uint64(10700), gas)
	cost, err := stake.Cost()
	require.NoError(err)
	require.Equal("107000", cost.Text(10))

	proto := stake.Proto()
	stake2 := &WithdrawStake{}
	require.NoError(stake2.LoadProto(proto))
	require.Equal(payload, stake2.Payload())
	require.Equal(index, stake2.BucketIndex())
}

func TestWithdrawSignVerify(t *testing.T) {
	require := require.New(t)

	require.Equal("cfa6ef757dee2e50351620dca002d32b9c090cfda55fb81f37f1d26b273743f1", senderKey.HexString())

	stake, err := NewWithdrawStake(nonce, index, payload, gaslimit, gasprice)
	require.NoError(err)

	bd := &EnvelopeBuilder{}
	elp := bd.SetGasLimit(gaslimit).
		SetGasPrice(gasprice).
		SetAction(stake).Build()
	// sign
	selp, err := Sign(elp, senderKey)
	require.NoError(err)
	require.NotNil(selp)
	ser, err := proto.Marshal(selp.Proto())
	require.NoError(err)
	require.Equal("0a18080118c0843d22023130d2020b080a12077061796c6f6164124104755ce6d8903f6b3793bddb4ea5d3589d637de2d209ae0ea930815c82db564ee8cc448886f639e8a0c7e94e99a5c1335b583c0bc76ef30dd6a1038ed9da8daf331a4152644d102186be6640d46b517331f3402e24424b0d85129595421d28503d75340b2922f5a0d4f667bbd6f576d9816770286b2ce032ba22eaec3952e24da4756b00", hex.EncodeToString(ser))
	hash, err := selp.Hash()
	require.NoError(err)
	require.Equal("28049348cf34f1aa927caa250e7a1b08778c44efaf73b565b6fa9abe843871b4", hex.EncodeToString(hash[:]))
	// verify signature
	require.NoError(selp.Verify())
}

func TestWithdrawABIEncodeAndDecode(t *testing.T) {
	require := require.New(t)
	stake, err := NewWithdrawStake(nonce, index, payload, gaslimit, gasprice)
	require.NoError(err)

	data, err := stake.EncodeABIBinary()
	require.NoError(err)
	stake, err = NewWithdrawStakeFromABIBinary(data)
	require.NoError(err)
	require.Equal(index, stake.bucketIndex)
	require.Equal(payload, stake.payload)
}
