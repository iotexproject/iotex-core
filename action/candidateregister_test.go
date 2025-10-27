// Copyright (c) 2020 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

type candidateRegisterTest struct {
	SenderKey       crypto.PrivateKey
	Nonce           uint64
	Name            string
	OperatorAddrStr string
	RewardAddrStr   string
	OwnerAddrStr    string
	AmountStr       string
	Duration        uint32
	AutoStake       bool
	blsPubKey       []byte
	Payload         []byte
	GasLimit        uint64
	GasPrice        *big.Int
	IntrinsicGas    uint64
	Cost            string
	ElpHash         string
	Expected        error
	SanityCheck     error
}

var candidateRegisterTestParams = []candidateRegisterTest{
	// valid test
	{
		identityset.PrivateKey(27), uint64(10), "test", "io10a298zmzvrt4guq79a9f4x7qedj59y7ery84he", "io13sj9mzpewn25ymheukte4v39hvjdtrfp00mlyv", "io19d0p3ah4g8ww9d7kcxfq87yxe7fnr8rpth5shj", "100", uint32(10000), false, []byte{}, []byte("payload"), uint64(1000000), big.NewInt(1000), uint64(10700), "10700100", "769725930ed38023058bb4f01c220feef2e3e40febb36856dfb780c4f7b1ea9b", nil, nil,
	},
	// invalid test
	{
		identityset.PrivateKey(27), uint64(10), "test", "io10a298zmzvrt4guq79a9f4x7qedj59y7ery84he", "io13sj9mzpewn25ymheukte4v39hvjdtrfp00mlyv", "io19d0p3ah4g8ww9d7kcxfq87yxe7fnr8rpth5shj", "ab-10", uint32(10000), false, []byte{}, []byte("payload"), uint64(1000000), big.NewInt(1000), uint64(10700), "", "", ErrInvalidAmount, nil,
	},
	// invalid candidate name
	{
		identityset.PrivateKey(27), uint64(10), "F@￥", "io10a298zmzvrt4guq79a9f4x7qedj59y7ery84he", "io13sj9mzpewn25ymheukte4v39hvjdtrfp00mlyv", "io19d0p3ah4g8ww9d7kcxfq87yxe7fnr8rpth5shj", "10", uint32(10000), false, []byte{}, []byte("payload"), uint64(1000000), big.NewInt(1000), uint64(10700), "", "", nil, ErrInvalidCanName,
	},
	// invalid candidate name
	{
		identityset.PrivateKey(27), uint64(10), "", "io10a298zmzvrt4guq79a9f4x7qedj59y7ery84he", "io13sj9mzpewn25ymheukte4v39hvjdtrfp00mlyv", "io19d0p3ah4g8ww9d7kcxfq87yxe7fnr8rpth5shj", "10", uint32(10000), false, []byte{}, []byte("payload"), uint64(1000000), big.NewInt(1000), uint64(10700), "", "", nil, ErrInvalidCanName,
	},
	// invalid candidate name
	{
		identityset.PrivateKey(27), uint64(10), "aaaaaaaaaaaaa", "io10a298zmzvrt4guq79a9f4x7qedj59y7ery84he", "io13sj9mzpewn25ymheukte4v39hvjdtrfp00mlyv", "io19d0p3ah4g8ww9d7kcxfq87yxe7fnr8rpth5shj", "10", uint32(10000), false, []byte{}, []byte("payload"), uint64(1000000), big.NewInt(1000), uint64(10700), "", "", nil, ErrInvalidCanName,
	},
	{
		identityset.PrivateKey(27), uint64(10), "test", "io10a298zmzvrt4guq79a9f4x7qedj59y7ery84he", "io13sj9mzpewn25ymheukte4v39hvjdtrfp00mlyv", "io19d0p3ah4g8ww9d7kcxfq87yxe7fnr8rpth5shj", "-10", uint32(10000), false, []byte{}, []byte("payload"), uint64(1000000), big.NewInt(1000), uint64(10700), "", "", nil, ErrInvalidAmount,
	},
	{
		identityset.PrivateKey(27), uint64(10), "test", "io10a298zmzvrt4guq79a9f4x7qedj59y7ery84he", "io13sj9mzpewn25ymheukte4v39hvjdtrfp00mlyv", "io19d0p3ah4g8ww9d7kcxfq87yxe7fnr8rpth5shj", "100", uint32(10000), false, []byte{}, []byte("payload"), uint64(1000000), big.NewInt(-1000), uint64(10700), "", "", nil, ErrNegativeValue,
	},
}

func TestCandidateRegister(t *testing.T) {
	require := require.New(t)
	validate := func(test candidateRegisterTest, cr *CandidateRegister) {
		elp := (&EnvelopeBuilder{}).SetNonce(test.Nonce).SetGasLimit(test.GasLimit).
			SetGasPrice(test.GasPrice).SetAction(cr).Build()
		err := elp.SanityCheck()
		require.Equal(test.SanityCheck, errors.Cause(err))
		if err != nil {
			return
		}

		require.Equal(test.GasLimit, elp.Gas())
		require.Equal(test.GasPrice, elp.GasPrice())
		require.Equal(test.Nonce, elp.Nonce())

		require.Equal(test.Name, cr.Name())
		require.Equal(test.OperatorAddrStr, cr.OperatorAddress().String())
		require.Equal(test.RewardAddrStr, cr.RewardAddress().String())
		require.Equal(test.OwnerAddrStr, cr.OwnerAddress().String())
		if len(test.blsPubKey) > 0 {
			require.Equal(test.blsPubKey, cr.BLSPubKey())
		}
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
		_, err = proto.Marshal(selp.Proto())
		require.NoError(err)
		_, err = selp.Hash()
		require.NoError(err)
		// verify signature
		require.NoError(selp.VerifySignature())
	}
	for _, test := range candidateRegisterTestParams {
		cr, err := NewCandidateRegister(test.Name, test.OperatorAddrStr, test.RewardAddrStr, test.OwnerAddrStr, test.AmountStr, test.Duration, test.AutoStake, test.Payload)
		require.Equal(test.Expected, errors.Cause(err))
		if err != nil {
			continue
		}
		validate(test, cr)
	}
	blsPrivKey, err := crypto.GenerateBLS12381PrivateKey(identityset.PrivateKey(0).Bytes())
	require.NoError(err)
	blsPubKey := blsPrivKey.PublicKey().Bytes()
	for _, test := range candidateRegisterTestParams {
		test.blsPubKey = blsPubKey
		cr, err := NewCandidateRegisterWithBLS(test.Name, test.OperatorAddrStr, test.RewardAddrStr, test.OwnerAddrStr, test.AmountStr, test.Duration, test.AutoStake, test.blsPubKey, test.Payload)
		require.Equal(test.Expected, errors.Cause(err))
		if err != nil {
			continue
		}
		cr.value, _ = new(big.Int).SetString(test.AmountStr, 10)
		validate(test, cr)
	}

}

func TestCandidateRegisterABIEncodeAndDecode(t *testing.T) {
	require := require.New(t)
	test := candidateRegisterTestParams[0]
	encode := func(input *CandidateRegister) {
		data, err := input.EthData()
		require.NoError(err)
		stake, err := NewCandidateRegisterFromABIBinary(data, input.Amount())
		require.NoError(err)
		require.Equal(test.Name, stake.Name())
		require.Equal(test.OperatorAddrStr, stake.OperatorAddress().String())
		require.Equal(test.RewardAddrStr, stake.RewardAddress().String())
		require.Equal(test.OwnerAddrStr, stake.OwnerAddress().String())
		require.Equal(test.Duration, stake.Duration())
		require.Equal(test.AutoStake, stake.AutoStake())
		if stake.WithBLS() {
			require.Equal(input.BLSPubKey(), stake.BLSPubKey())
		} else {
			require.Equal(test.AmountStr, stake.Amount().String())
		}
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
	t.Run("without public key", func(t *testing.T) {
		stake, err := NewCandidateRegister(test.Name, test.OperatorAddrStr, test.RewardAddrStr, test.OwnerAddrStr, test.AmountStr, test.Duration, test.AutoStake, test.Payload)
		require.NoError(err)
		encode(stake)
	})
	t.Run("with public key", func(t *testing.T) {
		pk, err := crypto.GenerateBLS12381PrivateKey(identityset.PrivateKey(0).Bytes())
		require.NoError(err)
		stake, err := NewCandidateRegisterWithBLS(test.Name, test.OperatorAddrStr, test.RewardAddrStr, test.OwnerAddrStr, test.AmountStr, test.Duration, test.AutoStake, pk.PublicKey().Bytes(), test.Payload)
		require.NoError(err)
		encode(stake)
	})

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

func TestStakingEvent(t *testing.T) {
	require := require.New(t)
	cand := identityset.Address(0)
	owner := identityset.Address(1)
	operator := identityset.Address(2)
	reward := identityset.Address(3)
	voter := identityset.Address(4)
	blsPrivKey, err := crypto.GenerateBLS12381PrivateKey(identityset.PrivateKey(0).Bytes())
	require.NoError(err)
	blsPubKey := blsPrivKey.PublicKey().Bytes()
	name := "test"

	t.Run("register", func(t *testing.T) {
		topics, data, err := PackCandidateRegisteredEvent(cand, operator, owner, name, reward, blsPubKey)
		require.NoError(err)
		checkCandidateRegisterEvent(require, topics, data, owner, cand, operator, reward, name, blsPubKey)
	})
	t.Run("staked", func(t *testing.T) {
		bktIdx := uint64(1)
		amount := big.NewInt(100)
		duration := uint32(3600)
		autoStake := true
		topics, data, err := PackStakedEvent(voter, cand, bktIdx, amount, duration, autoStake)
		require.NoError(err)
		checkStakedEvent(require, topics, data, voter, cand, bktIdx, amount, duration, autoStake)
	})
	t.Run("activate", func(t *testing.T) {
		bktIdx := uint64(1)
		topics, data, err := PackCandidateActivatedEvent(cand, bktIdx)
		require.NoError(err)
		checkActivatedEvent(require, topics, data, cand, bktIdx)
	})
	t.Run("update", func(t *testing.T) {
		topics, data, err := PackCandidateUpdatedEvent(cand, operator, owner, name, reward, blsPubKey)
		require.NoError(err)
		checkCandidateUpdateEvent(require, topics, data, owner, cand, operator, reward, name, blsPubKey)
	})
}

func checkCandidateRegisterEvent(require *require.Assertions, topics Topics, data []byte,
	owner, cand, operator, reward address.Address, name string, blsPubKey []byte,
) {
	paramsNonIndexed, err := _candidateRegisteredEvent.Inputs.Unpack(data)
	require.NoError(err)
	require.Equal(4, len(paramsNonIndexed))
	require.Equal(operator.Bytes(), paramsNonIndexed[0].(common.Address).Bytes())
	require.Equal(name, paramsNonIndexed[1].(string))
	require.Equal(reward.Bytes(), paramsNonIndexed[2].(common.Address).Bytes())
	require.Equal(blsPubKey, paramsNonIndexed[3].([]byte))
	require.Equal(3, len(topics))
	require.Equal(hash.Hash256(_candidateRegisteredEvent.ID), topics[0])
	require.Equal(hash.BytesToHash256(cand.Bytes()), topics[1])
	require.Equal(hash.BytesToHash256(owner.Bytes()), topics[2])
}

func checkStakedEvent(require *require.Assertions, topics Topics, data []byte,
	voter, cand address.Address, bktIdx uint64, amount *big.Int, duration uint32, autoStake bool,
) {
	paramsNonIndexed, err := _stakedEvent.Inputs.Unpack(data)
	require.NoError(err)
	require.Equal(4, len(paramsNonIndexed))
	require.Equal(bktIdx, paramsNonIndexed[0].(uint64))
	require.Equal(amount, paramsNonIndexed[1].(*big.Int))
	require.Equal(duration, paramsNonIndexed[2].(uint32))
	require.Equal(autoStake, paramsNonIndexed[3].(bool))
	require.Equal(3, len(topics))
	require.Equal(hash.Hash256(_stakedEvent.ID), topics[0])
	require.Equal(hash.BytesToHash256(voter.Bytes()), topics[1])
	require.Equal(hash.BytesToHash256(cand.Bytes()), topics[2])
}

func checkActivatedEvent(require *require.Assertions, topics Topics, data []byte,
	cand address.Address, bktIdx uint64,
) {
	paramsNonIndexed, err := _candidateActivatedEvent.Inputs.Unpack(data)
	require.NoError(err)
	require.Equal(1, len(paramsNonIndexed))
	require.Equal(bktIdx, paramsNonIndexed[0].(uint64))
	require.Equal(2, len(topics))
	require.Equal(hash.Hash256(_candidateActivatedEvent.ID), topics[0])
	require.Equal(hash.BytesToHash256(cand.Bytes()), topics[1])
}

func checkCandidateUpdateEvent(require *require.Assertions, topics Topics, data []byte,
	owner, cand, operator, reward address.Address, name string, blsPubKey []byte,
) {
	paramsNonIndexed, err := _candidateUpdateWithBLSEvent.Inputs.Unpack(data)
	require.NoError(err)
	require.Equal(4, len(paramsNonIndexed))
	require.Equal(reward.Bytes(), paramsNonIndexed[0].(common.Address).Bytes())
	require.Equal(name, paramsNonIndexed[1].(string))
	require.Equal(operator.Bytes(), paramsNonIndexed[2].(common.Address).Bytes())
	require.Equal(blsPubKey, paramsNonIndexed[3].([]byte))
	require.Equal(3, len(topics))
	require.Equal(hash.Hash256(_candidateUpdateWithBLSEvent.ID), topics[0])
	require.Equal(hash.BytesToHash256(cand.Bytes()), topics[1])
	require.Equal(hash.BytesToHash256(owner.Bytes()), topics[2])
}
