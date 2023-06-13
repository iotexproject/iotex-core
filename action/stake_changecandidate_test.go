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
	stake, err := NewChangeCandidate(_nonce, _canName, _index, _payload, _gaslimit, _gasprice)
	require.NoError(err)

	ser := stake.Serialize()
	require.Equal("080a120a63616e646964617465311a077061796c6f6164", hex.EncodeToString(ser))

	require.NoError(err)
	require.Equal(_gaslimit, stake.GasLimit())
	require.Equal(_gasprice, stake.GasPrice())
	require.Equal(_nonce, stake.Nonce())

	require.Equal(_payload, stake.Payload())
	require.Equal(_canName, stake.Candidate())
	require.Equal(_index, stake.BucketIndex())

	gas, err := stake.IntrinsicGas()
	require.NoError(err)
	require.Equal(uint64(10700), gas)
	cost, err := stake.Cost()
	require.NoError(err)
	require.Equal("107000", cost.Text(10))

	proto := stake.Proto()
	stake2 := &ChangeCandidate{}
	require.NoError(stake2.LoadProto(proto))
	require.Equal(_payload, stake2.Payload())
	require.Equal(_canName, stake2.Candidate())
	require.Equal(_index, stake2.BucketIndex())

	t.Run("Invalid Gas Price", func(t *testing.T) {
		cc, err := NewChangeCandidate(_nonce, _canName, _index, _payload, _gaslimit, new(big.Int).Mul(_gasprice, big.NewInt(-1)))
		require.NoError(err)
		require.Equal(ErrNegativeValue, errors.Cause(cc.SanityCheck()))
	})
}

func TestChangeCandidateSignVerify(t *testing.T) {
	require := require.New(t)
	require.Equal("cfa6ef757dee2e50351620dca002d32b9c090cfda55fb81f37f1d26b273743f1", _senderKey.HexString())
	stake, err := NewChangeCandidate(_nonce, _canName, _index, _payload, _gaslimit, _gasprice)
	require.NoError(err)

	bd := &EnvelopeBuilder{}
	elp := bd.SetGasLimit(_gaslimit).
		SetGasPrice(_gasprice).
		SetAction(stake).Build()
	// sign
	selp, err := Sign(elp, _senderKey)
	require.NoError(err)
	require.NotNil(selp)
	ser, err := proto.Marshal(selp.Proto())
	require.NoError(err)
	require.Equal("0a24080118c0843d22023130ea0217080a120a63616e646964617465311a077061796c6f6164124104755ce6d8903f6b3793bddb4ea5d3589d637de2d209ae0ea930815c82db564ee8cc448886f639e8a0c7e94e99a5c1335b583c0bc76ef30dd6a1038ed9da8daf331a412da6cd3ba7e830f0b669661f88a5c03307bdb44cae6fb45a4432ab69719f051f6631276467977617be2d265fdb00a3acc3a493261e2363a60acd3512aa15a89301", hex.EncodeToString(ser))
	hash, err := selp.Hash()
	require.NoError(err)
	require.Equal("bc65d832237134c6a38d6ba10637af097b432d6c83d267aa6235e5b7c953d30f", hex.EncodeToString(hash[:]))
	// verify signature
	require.NoError(selp.VerifySignature())
}

func TestChangeCandidateABIEncodeAndDecode(t *testing.T) {
	require := require.New(t)
	stake, err := NewChangeCandidate(_nonce, _canName, _index, _payload, _gaslimit, _gasprice)
	require.NoError(err)

	data, err := stake.EncodeABIBinary()
	require.NoError(err)
	stake, err = NewChangeCandidateFromABIBinary(data)
	require.NoError(err)
	require.Equal(_canName, stake.candidateName)
	require.Equal(_index, stake.bucketIndex)
	require.Equal(_payload, stake.payload)
}
