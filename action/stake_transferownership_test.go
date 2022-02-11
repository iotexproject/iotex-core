package action

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

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
	require.Equal(canAddress, stake.VoterAddress().String())
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
	require.Equal(canAddress, stake2.VoterAddress().String())
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
	// sign
	selp, err := Sign(elp, senderKey)
	require.NoError(err)
	require.NotNil(selp)
	ser, err := proto.Marshal(selp.Proto())
	require.NoError(err)
	require.Equal("0a43080118c0843d22023130f20236080a1229696f3178707136326177383575717a72636367397935686e727976386c64326e6b7079636333677a611a077061796c6f6164124104755ce6d8903f6b3793bddb4ea5d3589d637de2d209ae0ea930815c82db564ee8cc448886f639e8a0c7e94e99a5c1335b583c0bc76ef30dd6a1038ed9da8daf331a41fa26db427ab87a56a129196c1604f2e22c4dd2a1f99b2217bc916260757d00093d9e6dccdf53e3b0b64e41a69d71c238fbf9281625164694a74dfbeba075d0ce01", hex.EncodeToString(ser))
	hash, err := selp.Hash()
	require.NoError(err)
	require.Equal("74b2e1d6a09ba5d1298fa422d5850991ae516865077282196295a38f93c78b85", hex.EncodeToString(hash[:]))
	// verify signature
	require.NoError(Verify(selp))
}

func TestStakingTransferABIEncodeAndDecode(t *testing.T) {
	require := require.New(t)
	stake, err := NewTransferStake(nonce, canAddress, index, payload, gaslimit, gasprice)
	require.NoError(err)

	data, err := stake.EncodingABIBinary()
	require.NoError(err)
	err = stake.DecodingABIBinary(data)
	require.NoError(err)
	require.Equal(canAddress, stake.voterAddress.String())
	require.Equal(index, stake.bucketIndex)
	require.Equal(payload, stake.payload)
}
