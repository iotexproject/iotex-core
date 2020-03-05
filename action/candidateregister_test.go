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
)

var (
	crNonce           = uint64(10)
	crName            = "test"
	crOperatorAddrStr = "io10a298zmzvrt4guq79a9f4x7qedj59y7ery84he"
	crRewardAddrStr   = "io13sj9mzpewn25ymheukte4v39hvjdtrfp00mlyv"
	crOwnerAddrStr    = "io19d0p3ah4g8ww9d7kcxfq87yxe7fnr8rpth5shj"
	crAmountStr       = "100"
	crDuration        = uint32(10000)
	crAutoStake       = false
	crPayload         = []byte("payload")
	crGasLimit        = uint64(1000000)
	crGasPrice        = big.NewInt(1000)
)

func TestCandidateRegister(t *testing.T) {
	require := require.New(t)
	cr, err := NewCandidateRegister(crNonce, crName, crOperatorAddrStr, crRewardAddrStr, crOwnerAddrStr, crAmountStr, crDuration, crAutoStake, crPayload, crGasLimit, crGasPrice)
	require.NoError(err)

	ser := cr.Serialize()
	require.Equal("0a5c0a04746573741229696f3130613239387a6d7a7672743467757137396139663478377165646a35397937657279383468651a29696f3133736a396d7a7065776e3235796d6865756b74653476333968766a647472667030306d6c7976120331303018904e2a29696f313964307033616834673877773964376b63786671383779786537666e7238727074683573686a32077061796c6f6164", hex.EncodeToString(ser))

	require.NoError(err)
	require.Equal(crGasLimit, cr.GasLimit())
	require.Equal(crGasPrice, cr.GasPrice())
	require.Equal(crNonce, cr.Nonce())

	require.Equal(crName, cr.Name())
	require.Equal(crOperatorAddrStr, cr.OperatorAddress().String())
	require.Equal(crRewardAddrStr, cr.RewardAddress().String())
	require.Equal(crOwnerAddrStr, cr.OwnerAddress().String())
	require.Equal(crAmountStr, cr.Amount().String())
	require.Equal(crDuration, cr.Duration())
	require.Equal(crAutoStake, cr.AutoStake())
	require.Equal(crPayload, cr.Payload())

	gas, err := cr.IntrinsicGas()
	require.NoError(err)
	require.Equal(uint64(10700), gas)
	cost, err := cr.Cost()
	require.NoError(err)
	require.Equal("10700100", cost.Text(10))

	proto := cr.Proto()
	cr2 := &CandidateRegister{}
	require.NoError(cr2.LoadProto(proto))
	require.Equal(crName, cr2.Name())
	require.Equal(crOperatorAddrStr, cr2.OperatorAddress().String())
	require.Equal(crRewardAddrStr, cr2.RewardAddress().String())
	require.Equal(crOwnerAddrStr, cr2.OwnerAddress().String())
	require.Equal(crAmountStr, cr2.Amount().String())
	require.Equal(crDuration, cr2.Duration())
	require.Equal(crAutoStake, cr2.AutoStake())
	require.Equal(crPayload, cr2.Payload())
}

func TestCandidateRegisterSignVerify(t *testing.T) {
	require := require.New(t)
	cr, err := NewCandidateRegister(crNonce, crName, crOperatorAddrStr, crRewardAddrStr, crOwnerAddrStr, crAmountStr, crDuration, crAutoStake, crPayload, crGasLimit, crGasPrice)
	require.NoError(err)

	bd := &EnvelopeBuilder{}
	elp := bd.SetGasLimit(gaslimit).
		SetGasPrice(gasprice).
		SetAction(cr).Build()
	h := elp.Hash()
	require.Equal("9aa19b4e481d798ee82822596a6e069e9b2ba44b5997f6d5d6cbafe6792e5e92", hex.EncodeToString(h[:]))
	// sign
	selp, err := Sign(elp, senderKey)
	require.NoError(err)
	require.NotNil(selp)
	ser, err := proto.Marshal(selp.Proto())
	require.NoError(err)
	require.Equal("0aa801080118c0843d22023130fa029a010a5c0a04746573741229696f3130613239387a6d7a7672743467757137396139663478377165646a35397937657279383468651a29696f3133736a396d7a7065776e3235796d6865756b74653476333968766a647472667030306d6c7976120331303018904e2a29696f313964307033616834673877773964376b63786671383779786537666e7238727074683573686a32077061796c6f6164124104755ce6d8903f6b3793bddb4ea5d3589d637de2d209ae0ea930815c82db564ee8cc448886f639e8a0c7e94e99a5c1335b583c0bc76ef30dd6a1038ed9da8daf331a41a24fbde43c0bee6f3ce703f449fcb618b84b52a66eb294c89bd13ec1eaece39d39010ec28297f7c299185852ff85af77baa134dd963622d53c6eeee1bd56b22700", hex.EncodeToString(ser))
	hash := selp.Hash()
	require.Equal("5d02b48f36d1b862e527fbd931a3c736c9434edd98e438aa3da8199c029a546f", hex.EncodeToString(hash[:]))
	// verify signature
	require.NoError(Verify(selp))
}
