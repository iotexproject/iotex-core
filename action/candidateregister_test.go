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

	"github.com/iotexproject/go-pkgs/crypto"

	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

var candidateRegisterTestParams = []struct {
	SenderKey       crypto.PrivateKey
	Nonce           uint64
	Name            string
	OperatorAddrStr string
	RewardAddrStr   string
	OwnerAddrStr    string
	AmountStr       string
	Duration        uint32
	AutoStake       bool
	Payload         []byte
	GasLimit        uint64
	GasPrice        *big.Int
	Serialize       string
	IntrinsicGas    uint64
	Cost            string
	ElpHash         string
	Sign            string
	SelpHash        string
	Expected        error
	SanityCheck     error
}{
	// valid test
	{
		identityset.PrivateKey(27), uint64(10), "test", "io10a298zmzvrt4guq79a9f4x7qedj59y7ery84he", "io13sj9mzpewn25ymheukte4v39hvjdtrfp00mlyv", "io19d0p3ah4g8ww9d7kcxfq87yxe7fnr8rpth5shj", "100", uint32(10000), false, []byte("payload"), uint64(1000000), big.NewInt(1000), "0a5c0a04746573741229696f3130613239387a6d7a7672743467757137396139663478377165646a35397937657279383468651a29696f3133736a396d7a7065776e3235796d6865756b74653476333968766a647472667030306d6c7976120331303018904e2a29696f313964307033616834673877773964376b63786671383779786537666e7238727074683573686a32077061796c6f6164", uint64(10700), "10700100", "769725930ed38023058bb4f01c220feef2e3e40febb36856dfb780c4f7b1ea9b", "0aac010801100a18c0843d220431303030fa029a010a5c0a04746573741229696f3130613239387a6d7a7672743467757137396139663478377165646a35397937657279383468651a29696f3133736a396d7a7065776e3235796d6865756b74653476333968766a647472667030306d6c7976120331303018904e2a29696f313964307033616834673877773964376b63786671383779786537666e7238727074683573686a32077061796c6f6164124104755ce6d8903f6b3793bddb4ea5d3589d637de2d209ae0ea930815c82db564ee8cc448886f639e8a0c7e94e99a5c1335b583c0bc76ef30dd6a1038ed9da8daf331a41a1f4e2eebfb6228b5f2523da29e0946a823b54250c0883f025d527be275e98277e5403a9f13a2f46d3323420ff47461fc71014b3eb4f8f4927ff85513167da4d00", "6e1fa7697e84c220e8cd2ae1c3540a1754db0aa03d8b762dbc6e759d7d1620c1", nil, nil,
	},
	// invalid test
	{
		identityset.PrivateKey(27), uint64(10), "test", "io10a298zmzvrt4guq79a9f4x7qedj59y7ery84he", "io13sj9mzpewn25ymheukte4v39hvjdtrfp00mlyv", "io19d0p3ah4g8ww9d7kcxfq87yxe7fnr8rpth5shj", "ab-10", uint32(10000), false, []byte("payload"), uint64(1000000), big.NewInt(1000), "", uint64(10700), "", "", "", "", ErrInvalidAmount, nil,
	},
	// invalid candidate name
	{
		identityset.PrivateKey(27), uint64(10), "F@￥", "io10a298zmzvrt4guq79a9f4x7qedj59y7ery84he", "io13sj9mzpewn25ymheukte4v39hvjdtrfp00mlyv", "io19d0p3ah4g8ww9d7kcxfq87yxe7fnr8rpth5shj", "10", uint32(10000), false, []byte("payload"), uint64(1000000), big.NewInt(1000), "", uint64(10700), "", "", "", "", nil, ErrInvalidCanName,
	},
	// invalid candidate name
	{
		identityset.PrivateKey(27), uint64(10), "", "io10a298zmzvrt4guq79a9f4x7qedj59y7ery84he", "io13sj9mzpewn25ymheukte4v39hvjdtrfp00mlyv", "io19d0p3ah4g8ww9d7kcxfq87yxe7fnr8rpth5shj", "10", uint32(10000), false, []byte("payload"), uint64(1000000), big.NewInt(1000), "", uint64(10700), "", "", "", "", nil, ErrInvalidCanName,
	},
	// invalid candidate name
	{
		identityset.PrivateKey(27), uint64(10), "aaaaaaaaaaaaa", "io10a298zmzvrt4guq79a9f4x7qedj59y7ery84he", "io13sj9mzpewn25ymheukte4v39hvjdtrfp00mlyv", "io19d0p3ah4g8ww9d7kcxfq87yxe7fnr8rpth5shj", "10", uint32(10000), false, []byte("payload"), uint64(1000000), big.NewInt(1000), "", uint64(10700), "", "", "", "", nil, ErrInvalidCanName,
	},
	{
		identityset.PrivateKey(27), uint64(10), "test", "io10a298zmzvrt4guq79a9f4x7qedj59y7ery84he", "io13sj9mzpewn25ymheukte4v39hvjdtrfp00mlyv", "io19d0p3ah4g8ww9d7kcxfq87yxe7fnr8rpth5shj", "-10", uint32(10000), false, []byte("payload"), uint64(1000000), big.NewInt(1000), "", uint64(10700), "", "", "", "", nil, ErrInvalidAmount,
	},
	{
		identityset.PrivateKey(27), uint64(10), "test", "io10a298zmzvrt4guq79a9f4x7qedj59y7ery84he", "io13sj9mzpewn25ymheukte4v39hvjdtrfp00mlyv", "io19d0p3ah4g8ww9d7kcxfq87yxe7fnr8rpth5shj", "100", uint32(10000), false, []byte("payload"), uint64(1000000), big.NewInt(-1000), "", uint64(10700), "", "", "", "", nil, ErrNegativeValue,
	},
}

func TestCandidateRegister(t *testing.T) {
	require := require.New(t)
	for _, test := range candidateRegisterTestParams {
		cr, err := NewCandidateRegister(test.Name, test.OperatorAddrStr, test.RewardAddrStr, test.OwnerAddrStr, test.AmountStr, test.Duration, test.AutoStake, test.Payload)
		require.Equal(test.Expected, errors.Cause(err))
		if err != nil {
			continue
		}
		elp := (&EnvelopeBuilder{}).SetNonce(test.Nonce).SetGasLimit(test.GasLimit).
			SetGasPrice(test.GasPrice).SetAction(cr).Build()
		err = elp.SanityCheck()
		require.Equal(test.SanityCheck, errors.Cause(err))
		if err != nil {
			continue
		}

		require.Equal(test.Serialize, hex.EncodeToString(cr.Serialize()))
		require.Equal(test.GasLimit, elp.Gas())
		require.Equal(test.GasPrice, elp.GasPrice())
		require.Equal(test.Nonce, elp.Nonce())

		require.Equal(test.Name, cr.Name())
		require.Equal(test.OperatorAddrStr, cr.OperatorAddress().String())
		require.Equal(test.RewardAddrStr, cr.RewardAddress().String())
		require.Equal(test.OwnerAddrStr, cr.OwnerAddress().String())
		require.Equal(test.AmountStr, cr.Amount().String())
		require.Equal(test.Duration, cr.Duration())
		require.Equal(test.AutoStake, cr.AutoStake())
		require.Equal(test.Payload, cr.Payload())

		gas, err := cr.IntrinsicGas()
		require.NoError(err)
		require.Equal(test.IntrinsicGas, gas)
		cost, err := elp.Cost()
		require.NoError(err)
		require.Equal(test.Cost, cost.Text(10))

		cr2 := &CandidateRegister{}
		require.NoError(cr2.LoadProto(cr.Proto()))
		require.Equal(test.Name, cr2.Name())
		require.Equal(test.OperatorAddrStr, cr2.OperatorAddress().String())
		require.Equal(test.RewardAddrStr, cr2.RewardAddress().String())
		require.Equal(test.OwnerAddrStr, cr2.OwnerAddress().String())
		require.Equal(test.AmountStr, cr2.Amount().String())
		require.Equal(test.Duration, cr2.Duration())
		require.Equal(test.AutoStake, cr2.AutoStake())
		require.Equal(test.Payload, cr2.Payload())

		// verify sign
		selp, err := Sign(elp, test.SenderKey)
		require.NoError(err)
		require.NotNil(selp)
		ser, err := proto.Marshal(selp.Proto())
		require.NoError(err)
		require.Equal(test.Sign, hex.EncodeToString(ser))
		hash, err := selp.Hash()
		require.NoError(err)
		require.Equal(test.SelpHash, hex.EncodeToString(hash[:]))
		// verify signature
		require.NoError(selp.VerifySignature())
	}

}

func TestCandidateRegisterABIEncodeAndDecode(t *testing.T) {
	require := require.New(t)
	test := candidateRegisterTestParams[0]
	stake, err := NewCandidateRegister(test.Name, test.OperatorAddrStr, test.RewardAddrStr, test.OwnerAddrStr, test.AmountStr, test.Duration, test.AutoStake, test.Payload)
	require.NoError(err)

	data, err := stake.EthData()
	require.NoError(err)
	stake, err = NewCandidateRegisterFromABIBinary(data)
	require.NoError(err)
	require.Equal(test.Name, stake.Name())
	require.Equal(test.OperatorAddrStr, stake.OperatorAddress().String())
	require.Equal(test.RewardAddrStr, stake.RewardAddress().String())
	require.Equal(test.OwnerAddrStr, stake.OwnerAddress().String())
	require.Equal(test.AmountStr, stake.Amount().String())
	require.Equal(test.Duration, stake.Duration())
	require.Equal(test.AutoStake, stake.AutoStake())
	require.Equal(test.Payload, stake.Payload())

	stake.ownerAddress = nil
	_, err = stake.EthData()
	require.Equal(ErrAddress, err)
	stake.rewardAddress = nil
	_, err = stake.EthData()
	require.Equal(ErrAddress, err)
	stake.operatorAddress = nil
	_, err = stake.EthData()
	require.Equal(ErrAddress, err)
}

func TestIsValidCandidateName(t *testing.T) {
	require := require.New(t)
	tests := []struct {
		input  string
		output bool
	}{
		{
			input:  "abc",
			output: true,
		},
		{
			input:  "123",
			output: true,
		},
		{
			input:  "abc123abc123",
			output: true,
		},
		{
			input:  "Abc123",
			output: false,
		},
		{
			input:  "Abc 123",
			output: false,
		},
		{
			input:  "Abc-123",
			output: false,
		},
		{
			input:  "abc123abc123abc123",
			output: false,
		},
		{
			input:  "",
			output: false,
		},
	}

	for _, tt := range tests {
		output := IsValidCandidateName(tt.input)
		require.Equal(tt.output, output)
	}
}
