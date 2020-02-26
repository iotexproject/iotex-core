// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/hex"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/require"
)

func TestChangeCandidate(t *testing.T) {
	require := require.New(t)
	stake, err := NewChangeCandidate(nonce, canAddress, index, payload, gaslimit, gasprice)
	require.NoError(err)

	ser := stake.Serialize()
	require.Equal("080a1229696f3178707136326177383575717a72636367397935686e727976386c64326e6b7079636333677a611a077061796c6f6164", hex.EncodeToString(ser))

	require.NoError(err)
	require.Equal(gaslimit, stake.GasLimit())
	require.Equal(gasprice, stake.GasPrice())
	require.Equal(nonce, stake.Nonce())

	require.Equal(payload, stake.Payload())
	require.Equal(canAddress, stake.Candidate())
	require.Equal(index, stake.BucketIndex())

	gas, err := stake.IntrinsicGas()
	require.NoError(err)
	require.Equal(uint64(10700), gas)
	cost, err := stake.Cost()
	require.NoError(err)
	require.Equal("107000", cost.Text(10))

	proto := stake.Proto()
	stake2 := &ChangeCandidate{}
	require.NoError(stake2.LoadProto(proto))
	require.Equal(payload, stake2.Payload())
	require.Equal(canAddress, stake2.Candidate())
	require.Equal(index, stake2.BucketIndex())
}

func TestChangeCandidateSignVerify(t *testing.T) {
	require := require.New(t)
	require.Equal("cfa6ef757dee2e50351620dca002d32b9c090cfda55fb81f37f1d26b273743f1", senderKey.HexString())
	stake, err := NewChangeCandidate(nonce, canAddress, index, payload, gaslimit, gasprice)
	require.NoError(err)

	bd := &EnvelopeBuilder{}
	elp := bd.SetGasLimit(gaslimit).
		SetGasPrice(gasprice).
		SetAction(stake).Build()
	h := elp.Hash()
	require.Equal("4fe20be23e3de2fd81cacdebb04d4b69b0f80404fd0193a92d6478ae38602d9f", hex.EncodeToString(h[:]))
	// sign
	selp, err := Sign(elp, senderKey)
	require.NoError(err)
	require.NotNil(selp)
	ser, err := proto.Marshal(selp.Proto())
	require.NoError(err)
	require.Equal("0a43080118c0843d22023130ea0236080a1229696f3178707136326177383575717a72636367397935686e727976386c64326e6b7079636333677a611a077061796c6f6164124104755ce6d8903f6b3793bddb4ea5d3589d637de2d209ae0ea930815c82db564ee8cc448886f639e8a0c7e94e99a5c1335b583c0bc76ef30dd6a1038ed9da8daf331a41d519eb3747163b945b862989b7e82a7f8468001e9683757cb88d5ddd95f81895047429e858bd48f7d59a88bfec92de231d216293aeba1e4fbe11461d9c9fc99801", hex.EncodeToString(ser))
	hash := selp.Hash()
	require.Equal("186526b5b9fe74e25beb52c83c41780a69108160bef2ddaf3bffb9f1f1e5e73a", hex.EncodeToString(hash[:]))
	// verify signature
	require.NoError(Verify(selp))
}

func TestStakingTransfer(t *testing.T) {
	require := require.New(t)
	stake, err := NewTransferStake(nonce, canAddress, index, payload, gaslimit, gasprice)
	require.NoError(err)

	ser := stake.Serialize()
	require.Equal("080a1229696f3178707136326177383575717a72636367397935686e727976386c64326e6b7079636333677a611a077061796c6f6164", hex.EncodeToString(ser))

	require.NoError(err)
	require.Equal(gaslimit, stake.GasLimit())
	require.Equal(gasprice, stake.GasPrice())
	require.Equal(nonce, stake.Nonce())

	require.Equal(payload, stake.Payload())
	require.Equal(canAddress, stake.Candidate())
	require.Equal(index, stake.BucketIndex())

	gas, err := stake.IntrinsicGas()
	require.NoError(err)
	require.Equal(uint64(10700), gas)
	cost, err := stake.Cost()
	require.NoError(err)
	require.Equal("107000", cost.Text(10))

	proto := stake.Proto()
	stake2 := &TransferStake{}
	require.NoError(stake2.LoadProto(proto))
	require.Equal(payload, stake2.Payload())
	require.Equal(canAddress, stake2.Candidate())
	require.Equal(index, stake2.BucketIndex())
}

func TestStakingTransferSignVerify(t *testing.T) {
	require := require.New(t)
	require.Equal("cfa6ef757dee2e50351620dca002d32b9c090cfda55fb81f37f1d26b273743f1", senderKey.HexString())
	stake, err := NewTransferStake(nonce, canAddress, index, payload, gaslimit, gasprice)
	require.NoError(err)

	bd := &EnvelopeBuilder{}
	elp := bd.SetGasLimit(gaslimit).
		SetGasPrice(gasprice).
		SetAction(stake).Build()
	h := elp.Hash()
	require.Equal("d22b4b3e630e1d494951e9041a983608232cf64629262296b6ef1f57fa748fd2", hex.EncodeToString(h[:]))
	// sign
	selp, err := Sign(elp, senderKey)
	require.NoError(err)
	require.NotNil(selp)
	ser, err := proto.Marshal(selp.Proto())
	require.NoError(err)
	require.Equal("0a43080118c0843d22023130f20236080a1229696f3178707136326177383575717a72636367397935686e727976386c64326e6b7079636333677a611a077061796c6f6164124104755ce6d8903f6b3793bddb4ea5d3589d637de2d209ae0ea930815c82db564ee8cc448886f639e8a0c7e94e99a5c1335b583c0bc76ef30dd6a1038ed9da8daf331a41fa26db427ab87a56a129196c1604f2e22c4dd2a1f99b2217bc916260757d00093d9e6dccdf53e3b0b64e41a69d71c238fbf9281625164694a74dfbeba075d0ce01", hex.EncodeToString(ser))
	hash := selp.Hash()
	require.Equal("74b2e1d6a09ba5d1298fa422d5850991ae516865077282196295a38f93c78b85", hex.EncodeToString(hash[:]))
	// verify signature
	require.NoError(Verify(selp))
}
