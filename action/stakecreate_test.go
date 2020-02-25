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

	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/test/identityset"
)

var (
	gaslimit   = uint64(1000000)
	gasprice   = big.NewInt(10)
	canAddress = "io1xpq62aw85uqzrccg9y5hnryv8ld2nkpycc3gza"
	payload    = []byte("payload")
	amount     = big.NewInt(10)
	nonce      = uint64(0)
	duration   = uint32(1000)
	autoStake  = true
	index      = uint64(10)
	senderKey  = identityset.PrivateKey(27)
)

func TestCreateStake(t *testing.T) {
	require := require.New(t)
	stake, err := NewCreateStake(nonce, canAddress, amount.Text(10), duration, autoStake, payload, gaslimit, gasprice)
	require.NoError(err)

	ser := stake.Serialize()
	require.Equal("0a29696f3178707136326177383575717a72636367397935686e727976386c64326e6b7079636333677a611202313018e80720012a077061796c6f6164", hex.EncodeToString(ser))

	require.NoError(err)
	require.Equal(gaslimit, stake.GasLimit())
	require.Equal(gasprice, stake.GasPrice())
	require.Equal(nonce, stake.Nonce())

	require.Equal(amount, stake.Amount())
	require.Equal(payload, stake.Payload())
	require.Equal(canAddress, stake.Candidate())
	require.Equal(duration, stake.Duration())
	require.True(stake.AutoStake())

	gas, err := stake.IntrinsicGas()
	require.NoError(err)
	require.Equal(uint64(10700), gas)
	cost, err := stake.Cost()
	require.NoError(err)
	require.Equal("107010", cost.Text(10))

	proto := stake.Proto()
	cs2 := &CreateStake{}
	require.NoError(cs2.LoadProto(proto))
	require.Equal(amount, cs2.Amount())
	require.Equal(payload, cs2.Payload())
	require.Equal(canAddress, cs2.Candidate())
	require.Equal(duration, cs2.Duration())
	require.True(cs2.AutoStake())
}

func TestCreateStakeSignVerify(t *testing.T) {
	require := require.New(t)
	require.Equal("cfa6ef757dee2e50351620dca002d32b9c090cfda55fb81f37f1d26b273743f1", senderKey.HexString())
	stake, err := NewCreateStake(nonce, canAddress, amount.Text(10), duration, autoStake, payload, gaslimit, gasprice)
	require.NoError(err)

	bd := &EnvelopeBuilder{}
	elp := bd.SetGasLimit(gaslimit).
		SetGasPrice(gasprice).
		SetAction(stake).Build()
	h := elp.Hash()
	require.Equal("219483a7309db9f1c41ac3fa0aadecfbdbeb0448b0dfaee54daec4ec178aa9f1", hex.EncodeToString(h[:]))
	// sign
	selp, err := Sign(elp, senderKey)
	require.NoError(err)
	require.NotNil(selp)
	ser, err := proto.Marshal(selp.Proto())
	require.NoError(err)
	require.Equal("0a4a080118c0843d22023130c2023d0a29696f3178707136326177383575717a72636367397935686e727976386c64326e6b7079636333677a611202313018e80720012a077061796c6f6164124104755ce6d8903f6b3793bddb4ea5d3589d637de2d209ae0ea930815c82db564ee8cc448886f639e8a0c7e94e99a5c1335b583c0bc76ef30dd6a1038ed9da8daf331a415db41c974bc1d8edd59fad54c4eac41250981640c44183c1c3ed9e45873bf15c02f3575de59233aefd7ec6eecfa7254bf4b67501e96bea8a4d54a18b4e0e4fec01", hex.EncodeToString(ser))
	hash := selp.Hash()
	require.Equal("a324d56f5b50e86aab27c0c6d33f9699f36d3ed8e27967a56e644f582bbd5e2d", hex.EncodeToString(hash[:]))
	// verify signature
	require.NoError(Verify(selp))
}
