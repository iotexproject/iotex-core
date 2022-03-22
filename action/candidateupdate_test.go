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
)

var (
	cuNonce           = uint64(20)
	cuName            = "test"
	cuOperatorAddrStr = "io1cl6rl2ev5dfa988qmgzg2x4hfazmp9vn2g66ng"
	cuRewardAddrStr   = "io1juvx5g063eu4ts832nukp4vgcwk2gnc5cu9ayd"
	cuGasLimit        = uint64(200000)
	cuGasPrice        = big.NewInt(2000)
)

func TestCandidateUpdate(t *testing.T) {
	require := require.New(t)
	cu, err := NewCandidateUpdate(cuNonce, cuName, cuOperatorAddrStr, cuRewardAddrStr, cuGasLimit, cuGasPrice)
	require.NoError(err)

	ser := cu.Serialize()
	require.Equal("0a04746573741229696f31636c36726c32657635646661393838716d677a673278346866617a6d7039766e326736366e671a29696f316a757678356730363365753474733833326e756b7034766763776b32676e6335637539617964", hex.EncodeToString(ser))

	require.NoError(err)
	require.Equal(cuGasLimit, cu.GasLimit())
	require.Equal(cuGasPrice, cu.GasPrice())
	require.Equal(cuNonce, cu.Nonce())

	require.Equal(cuName, cu.Name())
	require.Equal(cuOperatorAddrStr, cu.OperatorAddress().String())
	require.Equal(cuRewardAddrStr, cu.RewardAddress().String())

	gas, err := cu.IntrinsicGas()
	require.NoError(err)
	require.Equal(uint64(10000), gas)
	cost, err := cu.Cost()
	require.NoError(err)
	require.Equal("20000000", cost.Text(10))

	proto := cu.Proto()
	cu2 := &CandidateUpdate{}
	require.NoError(cu2.LoadProto(proto))
	require.Equal(cuName, cu2.Name())
	require.Equal(cuOperatorAddrStr, cu2.OperatorAddress().String())
	require.Equal(cuRewardAddrStr, cu2.RewardAddress().String())
}

func TestCandidateUpdateSignVerify(t *testing.T) {
	require := require.New(t)
	cu, err := NewCandidateUpdate(cuNonce, cuName, cuOperatorAddrStr, cuRewardAddrStr, cuGasLimit, cuGasPrice)
	require.NoError(err)

	bd := &EnvelopeBuilder{}
	elp := bd.SetGasLimit(gaslimit).
		SetGasPrice(gasprice).
		SetAction(cu).Build()
	// sign
	selp, err := Sign(elp, senderKey)
	require.NoError(err)
	require.NotNil(selp)
	ser, err := proto.Marshal(selp.Proto())
	require.NoError(err)
	require.Equal("0a69080118c0843d2202313082035c0a04746573741229696f31636c36726c32657635646661393838716d677a673278346866617a6d7039766e326736366e671a29696f316a757678356730363365753474733833326e756b7034766763776b32676e6335637539617964124104755ce6d8903f6b3793bddb4ea5d3589d637de2d209ae0ea930815c82db564ee8cc448886f639e8a0c7e94e99a5c1335b583c0bc76ef30dd6a1038ed9da8daf331a4101885c9c6684a4a8f2f5bf11f8326f27be48658f292e8f55ec8a11a604bb0c563a11ebf12d995ca1c152e00f8e0f0edf288db711aa10dbdfd5b7d73b4a28e1f701", hex.EncodeToString(ser))
	hash, err := selp.Hash()
	require.NoError(err)
	require.Equal("ca1a28f0e9a58ffc67037cc75066dbdd8e024aa2b2e416e4d6ce16c3d86282e5", hex.EncodeToString(hash[:]))
	// verify signature
	require.NoError(selp.VerifySignature())
}

func TestCandidateUpdateABIEncodeAndDecode(t *testing.T) {
	require := require.New(t)
	stake, err := NewCandidateUpdate(cuNonce, cuName, cuOperatorAddrStr, cuRewardAddrStr, cuGasLimit, cuGasPrice)
	require.NoError(err)

	data, err := stake.EncodeABIBinary()
	require.NoError(err)
	stake, err = NewCandidateUpdateFromABIBinary(data)
	require.NoError(err)
	require.Equal(cuName, stake.Name())
	require.Equal(cuOperatorAddrStr, stake.OperatorAddress().String())
	require.Equal(cuRewardAddrStr, stake.RewardAddress().String())
}
