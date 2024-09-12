package action

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestStakingTransfer(t *testing.T) {
	require := require.New(t)
	stake, err := NewTransferStake(_canAddress, _index, _payload)
	require.NoError(err)
	elp := (&EnvelopeBuilder{}).SetNonce(_nonce).SetGasLimit(_gasLimit).
		SetGasPrice(_gasPrice).SetAction(stake).Build()
	t.Run("proto", func(t *testing.T) {
		ser := stake.Serialize()
		require.Equal("080a1229696f3178707136326177383575717a72636367397935686e727976386c64326e6b7079636333677a611a077061796c6f6164", hex.EncodeToString(ser))
		require.Equal(_gaslimit, elp.Gas())
		require.Equal(_gasprice, elp.GasPrice())
		require.Equal(_nonce, elp.Nonce())

		require.Equal(_payload, stake.Payload())
		require.Equal(_canAddress, stake.VoterAddress().String())
		require.Equal(_index, stake.BucketIndex())

		gas, err := stake.IntrinsicGas()
		require.NoError(err)
		require.Equal(uint64(10700), gas)
		cost, err := elp.Cost()
		require.NoError(err)
		require.Equal("107000", cost.Text(10))

		proto := stake.Proto()
		stake2 := &TransferStake{}
		require.NoError(stake2.LoadProto(proto))
		require.Equal(_payload, stake2.Payload())
		require.Equal(_canAddress, stake2.VoterAddress().String())
		require.Equal(_index, stake2.BucketIndex())
	})
	t.Run("sign and verify", func(t *testing.T) {
		selp, err := Sign(elp, _senderKey)
		require.NoError(err)
		ser, err := proto.Marshal(selp.Proto())
		require.NoError(err)
		require.Equal("0a450801100118c0843d22023130f20236080a1229696f3178707136326177383575717a72636367397935686e727976386c64326e6b7079636333677a611a077061796c6f6164124104755ce6d8903f6b3793bddb4ea5d3589d637de2d209ae0ea930815c82db564ee8cc448886f639e8a0c7e94e99a5c1335b583c0bc76ef30dd6a1038ed9da8daf331a4131d3a608c86b134db575c5bf11d3c565c8ae91ef7c19b09d106e9bc9d346fbc4731092e8413394fd0eb8617e117ecae833d3366dd1e7b2adf6f0166aae8e5b6800", hex.EncodeToString(ser))
		hash, err := selp.Hash()
		require.NoError(err)
		require.Equal("4588cb05b9230e26f44fefd8b1d1e7e11c2aa09fa3bb1c45f3acbe012dda8dbe", hex.EncodeToString(hash[:]))
		// verify signature
		require.NoError(selp.VerifySignature())
	})
	t.Run("proto", func(t *testing.T) {
		data, err := stake.EthData()
		require.NoError(err)
		stake, err = NewTransferStakeFromABIBinary(data)
		require.NoError(err)
		require.Equal(_canAddress, stake.voterAddress.String())
		require.Equal(_index, stake.bucketIndex)
		require.Equal(_payload, stake.payload)
	})
}
