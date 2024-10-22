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
)

var (
	_cuNonce           = uint64(20)
	_cuName            = "test"
	_cuOperatorAddrStr = "io1cl6rl2ev5dfa988qmgzg2x4hfazmp9vn2g66ng"
	_cuRewardAddrStr   = "io1juvx5g063eu4ts832nukp4vgcwk2gnc5cu9ayd"
	_cuGasLimit        = uint64(200000)
	_cuGasPrice        = big.NewInt(2000)
)

func TestCandidateUpdate(t *testing.T) {
	require := require.New(t)
	cu, err := NewCandidateUpdate(_cuName, _cuOperatorAddrStr, _cuRewardAddrStr)
	require.NoError(err)
	elp := (&EnvelopeBuilder{}).SetNonce(_cuNonce).SetGasLimit(_cuGasLimit).
		SetGasPrice(_cuGasPrice).SetAction(cu).Build()
	t.Run("proto", func(t *testing.T) {
		ser := cu.Serialize()
		require.Equal("0a04746573741229696f31636c36726c32657635646661393838716d677a673278346866617a6d7039766e326736366e671a29696f316a757678356730363365753474733833326e756b7034766763776b32676e6335637539617964", hex.EncodeToString(ser))
		require.Equal(_cuGasLimit, elp.Gas())
		require.Equal(_cuGasPrice, elp.GasPrice())
		require.Equal(_cuNonce, elp.Nonce())
		require.Equal(_cuName, cu.Name())
		require.Equal(_cuOperatorAddrStr, cu.OperatorAddress().String())
		require.Equal(_cuRewardAddrStr, cu.RewardAddress().String())

		gas, err := cu.IntrinsicGas()
		require.NoError(err)
		require.Equal(uint64(10000), gas)
		cost, err := elp.Cost()
		require.NoError(err)
		require.Equal("20000000", cost.Text(10))

		proto := cu.Proto()
		cu2 := &CandidateUpdate{}
		require.NoError(cu2.LoadProto(proto))
		require.Equal(_cuName, cu2.Name())
		require.Equal(_cuOperatorAddrStr, cu2.OperatorAddress().String())
		require.Equal(_cuRewardAddrStr, cu2.RewardAddress().String())
	})
	t.Run("sign and verify", func(t *testing.T) {
		selp, err := Sign(elp, _senderKey)
		require.NoError(err)
		ser, err := proto.Marshal(selp.Proto())
		require.NoError(err)
		require.Equal("0a6d0801101418c09a0c22043230303082035c0a04746573741229696f31636c36726c32657635646661393838716d677a673278346866617a6d7039766e326736366e671a29696f316a757678356730363365753474733833326e756b7034766763776b32676e6335637539617964124104755ce6d8903f6b3793bddb4ea5d3589d637de2d209ae0ea930815c82db564ee8cc448886f639e8a0c7e94e99a5c1335b583c0bc76ef30dd6a1038ed9da8daf331a41b506eb384617d6cbcba05d74cd79503eb23a1f4d7eefbe09c296a241e567af81295508a3e1b87f98ba5eedfd21e0889764400b99d5c41b1e5c154a1126d72be200", hex.EncodeToString(ser))
		hash, err := selp.Hash()
		require.NoError(err)
		require.Equal("be1770571f979421e358f52cf4cbae7234801a2ef94ecc56e2bd0c7dd4cc9de4", hex.EncodeToString(hash[:]))
		// verify signature
		require.NoError(selp.VerifySignature())
	})
	t.Run("ABI encode", func(t *testing.T) {
		data, err := cu.EthData()
		require.NoError(err)
		cu, err = NewCandidateUpdateFromABIBinary(data)
		require.NoError(err)
		require.Equal(_cuName, cu.Name())
		require.Equal(_cuOperatorAddrStr, cu.OperatorAddress().String())
		require.Equal(_cuRewardAddrStr, cu.RewardAddress().String())

		cu.rewardAddress = nil
		_, err = cu.EthData()
		require.Equal(ErrAddress, err)
		cu.operatorAddress = nil
		_, err = cu.EthData()
		require.Equal(ErrAddress, err)
	})
}
