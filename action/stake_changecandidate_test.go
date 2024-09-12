// Copyright (c) 2020 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestChangeCandidate(t *testing.T) {
	require := require.New(t)
	stake := NewChangeCandidate(_canName, _index, _payload)
	elp := (&EnvelopeBuilder{}).SetNonce(_nonce).SetGasLimit(_gasLimit).
		SetGasPrice(_gasPrice).SetAction(stake).Build()
	t.Run("proto", func(t *testing.T) {
		ser := stake.Serialize()
		require.Equal("080a120a63616e646964617465311a077061796c6f6164", hex.EncodeToString(ser))
		require.Equal(_gaslimit, elp.Gas())
		require.Equal(_gasprice, elp.GasPrice())
		require.Equal(_nonce, elp.Nonce())

		require.Equal(_payload, stake.Payload())
		require.Equal(_canName, stake.Candidate())
		require.Equal(_index, stake.BucketIndex())

		gas, err := stake.IntrinsicGas()
		require.NoError(err)
		require.Equal(uint64(10700), gas)
		cost, err := elp.Cost()
		require.NoError(err)
		require.Equal("107000", cost.Text(10))

		proto := stake.Proto()
		stake2 := &ChangeCandidate{}
		require.NoError(stake2.LoadProto(proto))
		require.Equal(_payload, stake2.Payload())
		require.Equal(_canName, stake2.Candidate())
		require.Equal(_index, stake2.BucketIndex())
	})
	t.Run("Invalid Gas Price", func(t *testing.T) {
		cc := NewChangeCandidate(_canName, _index, _payload)
		elp := (&EnvelopeBuilder{}).SetNonce(_nonce).SetGasLimit(_gasLimit).
			SetGasPrice(big.NewInt(-1)).SetAction(cc).Build()
		require.Equal(ErrNegativeValue, errors.Cause(elp.SanityCheck()))
	})
	t.Run("sign and verify", func(t *testing.T) {
		selp, err := Sign(elp, _senderKey)
		require.NoError(err)
		ser, err := proto.Marshal(selp.Proto())
		require.NoError(err)
		require.Equal("0a260801100118c0843d22023130ea0217080a120a63616e646964617465311a077061796c6f6164124104755ce6d8903f6b3793bddb4ea5d3589d637de2d209ae0ea930815c82db564ee8cc448886f639e8a0c7e94e99a5c1335b583c0bc76ef30dd6a1038ed9da8daf331a41c95c324dc009bfea03af4bd704882693cb129f6283e8cf1a716683cf08cb5b637407655191f248e3b858ff743894a2baed592f03dd7fbec536a80a13984e485d01", hex.EncodeToString(ser))
		hash, err := selp.Hash()
		require.NoError(err)
		require.Equal("c6023267a2acd3dc437764b461b46c0ec6c977aba3045f16ce7eef6a878e4406", hex.EncodeToString(hash[:]))
		// verify signature
		require.NoError(selp.VerifySignature())
	})
	t.Run("ABI encode", func(t *testing.T) {
		data, err := stake.EthData()
		require.NoError(err)
		stake, err = NewChangeCandidateFromABIBinary(data)
		require.NoError(err)
		require.Equal(_canName, stake.candidateName)
		require.Equal(_index, stake.bucketIndex)
		require.Equal(_payload, stake.payload)
	})
}
