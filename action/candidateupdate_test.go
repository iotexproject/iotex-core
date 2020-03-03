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

func TestCandidateUpdate(t *testing.T) {
	require := require.New(t)
	cr, err := NewCandidateUpdate(nonce, canAddress, canAddress, canAddress, gaslimit, gasprice)
	require.NoError(err)

	ser := cr.Serialize()
	require.Equal("0a29696f3178707136326177383575717a72636367397935686e727976386c64326e6b7079636333677a611229696f3178707136326177383575717a72636367397935686e727976386c64326e6b7079636333677a611a29696f3178707136326177383575717a72636367397935686e727976386c64326e6b7079636333677a61", hex.EncodeToString(ser))

	require.NoError(err)
	require.Equal(gaslimit, cr.GasLimit())
	require.Equal(gasprice, cr.GasPrice())
	require.Equal(nonce, cr.Nonce())

	require.Equal(canAddress, cr.Name())
	require.Equal(canAddress, cr.OperatorAddress().String())
	require.Equal(canAddress, cr.RewardAddress().String())

	gas, err := cr.IntrinsicGas()
	require.NoError(err)
	require.Equal(uint64(10000), gas)
	cost, err := cr.Cost()
	require.NoError(err)
	require.Equal("100000", cost.Text(10))

	proto := cr.Proto()
	cr2 := &CandidateUpdate{}
	require.NoError(cr2.LoadProto(proto))
	require.Equal(canAddress, cr2.Name())
	require.Equal(canAddress, cr2.OperatorAddress().String())
	require.Equal(canAddress, cr2.RewardAddress().String())
}

func TestCandidateUpdateSignVerify(t *testing.T) {
	require := require.New(t)
	require.Equal("cfa6ef757dee2e50351620dca002d32b9c090cfda55fb81f37f1d26b273743f1", senderKey.HexString())
	cr, err := NewCandidateUpdate(nonce, canAddress, canAddress, canAddress, gaslimit, gasprice)
	require.NoError(err)

	bd := &EnvelopeBuilder{}
	elp := bd.SetGasLimit(gaslimit).
		SetGasPrice(gasprice).
		SetAction(cr).Build()
	h := elp.Hash()
	require.Equal("f332644befa8893fbca97d0e23c72fdc52e8af596c17cd8af72f4a90eb664e20", hex.EncodeToString(h[:]))
	// sign
	selp, err := Sign(elp, senderKey)
	require.NoError(err)
	require.NotNil(selp)
	ser, err := proto.Marshal(selp.Proto())
	require.NoError(err)
	require.Equal("0a8f01080118c0843d22023130820381010a29696f3178707136326177383575717a72636367397935686e727976386c64326e6b7079636333677a611229696f3178707136326177383575717a72636367397935686e727976386c64326e6b7079636333677a611a29696f3178707136326177383575717a72636367397935686e727976386c64326e6b7079636333677a61124104755ce6d8903f6b3793bddb4ea5d3589d637de2d209ae0ea930815c82db564ee8cc448886f639e8a0c7e94e99a5c1335b583c0bc76ef30dd6a1038ed9da8daf331a415297298bd854eede98ec4f43905124cee967e050c7d20d30f37595f689068fd36f7315420c1e550a6e300c2a7ee734268202d4cff86a4b01fbf0a5f35980c98900", hex.EncodeToString(ser))
	hash := selp.Hash()
	require.Equal("69ab00a70f44036627f565ac174467518be505406977fd0836ca5b95266e0ade", hex.EncodeToString(hash[:]))
	// verify signature
	require.NoError(Verify(selp))
}
