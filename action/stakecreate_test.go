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

	"github.com/iotexproject/iotex-core/v2/pkg/unit"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

var stakeCreateTestParams = []struct {
	SenderKey    crypto.PrivateKey
	Nonce        uint64
	CanAddress   string
	AmountStr    string
	Duration     uint32
	AutoStake    bool
	Payload      []byte
	GasLimit     uint64
	GasPrice     *big.Int
	Serialize    string
	IntrinsicGas uint64
	Cost         string
	ElpHash      string
	Sign         string
	SelpHash     string
	Expected     error
	SanityCheck  error
}{
	// valid test
	{
		identityset.PrivateKey(27), uint64(10), "test", "100", uint32(10000), true, []byte("payload"), uint64(1000000), big.NewInt(10), "0a0474657374120331303018904e20012a077061796c6f6164", uint64(10700), "107100", "18d76ff9f3cfed0fe84f3fd4831f11379edc5b3d689d646187520b3fe74ab44c", "0a26080118c0843d22023130c202190a0474657374120331303018904e20012a077061796c6f6164124104755ce6d8903f6b3793bddb4ea5d3589d637de2d209ae0ea930815c82db564ee8cc448886f639e8a0c7e94e99a5c1335b583c0bc76ef30dd6a1038ed9da8daf331a41563785be9d7e2d796a8aaca41dbe1a53a0bce3614ede09718e72c75cb40cdb48355964b69156008f2319e20db4a4023730c3a1664ac35dfc10a7ceff26be8ebe00", "ebb26b08e824e18cb6d38918411749351c065198603e4626bbdc10b900dde270", nil, nil,
	},
	// invalid test
	{
		identityset.PrivateKey(27), uint64(10), "test", "ae-10", uint32(10000), false, []byte("payload"), uint64(1000000), big.NewInt(1000), "", uint64(10700), "", "", "", "", ErrInvalidAmount, nil,
	},
	{
		identityset.PrivateKey(27), uint64(10), "test", "-10", uint32(10000), false, []byte("payload"), uint64(1000000), big.NewInt(1000), "", uint64(10700), "", "", "", "", nil, ErrInvalidAmount,
	},
	{
		identityset.PrivateKey(27), uint64(10), "test", "0", uint32(10000), false, []byte("payload"), uint64(1000000), big.NewInt(1000), "", uint64(10700), "", "", "", "", nil, ErrInvalidAmount,
	},
	{
		identityset.PrivateKey(27), uint64(10), "test", "100", uint32(10000), true, []byte("payload"), uint64(1000000), big.NewInt(-unit.Qev), "0a0474657374120331303018904e20012a077061796c6f6164", uint64(10700), "107100", "18d76ff9f3cfed0fe84f3fd4831f11379edc5b3d689d646187520b3fe74ab44c", "0a26080118c0843d22023130c202190a0474657374120331303018904e20012a077061796c6f6164124104755ce6d8903f6b3793bddb4ea5d3589d637de2d209ae0ea930815c82db564ee8cc448886f639e8a0c7e94e99a5c1335b583c0bc76ef30dd6a1038ed9da8daf331a41563785be9d7e2d796a8aaca41dbe1a53a0bce3614ede09718e72c75cb40cdb48355964b69156008f2319e20db4a4023730c3a1664ac35dfc10a7ceff26be8ebe00", "ebb26b08e824e18cb6d38918411749351c065198603e4626bbdc10b900dde270", nil, ErrNegativeValue,
	},
}

func TestCreateStake(t *testing.T) {
	require := require.New(t)
	for _, test := range stakeCreateTestParams {
		stake, err := NewCreateStake(test.Nonce, test.CanAddress, test.AmountStr, test.Duration, test.AutoStake, test.Payload, test.GasLimit, test.GasPrice)
		require.Equal(test.Expected, errors.Cause(err))
		if err != nil {
			continue
		}
		err = stake.SanityCheck()
		require.Equal(test.SanityCheck, errors.Cause(err))
		if err != nil {
			continue
		}

		ser := stake.Serialize()
		require.Equal(test.Serialize, hex.EncodeToString(ser))

		require.NoError(err)
		require.Equal(test.GasLimit, stake.GasLimit())
		require.Equal(test.GasPrice, stake.GasPrice())
		require.Equal(test.Nonce, stake.Nonce())

		require.Equal(test.AmountStr, stake.Amount().String())
		require.Equal(test.Payload, stake.Payload())
		require.Equal(test.CanAddress, stake.Candidate())
		require.Equal(test.Duration, stake.Duration())
		require.True(stake.AutoStake())

		gas, err := stake.IntrinsicGas()
		require.NoError(err)
		require.Equal(test.IntrinsicGas, gas)
		cost, err := stake.Cost()
		require.NoError(err)
		require.Equal(test.Cost, cost.Text(10))

		cs2 := &CreateStake{}
		require.NoError(cs2.LoadProto(stake.Proto()))
		require.Equal(test.AmountStr, cs2.Amount().String())
		require.Equal(test.Payload, cs2.Payload())
		require.Equal(test.CanAddress, cs2.Candidate())
		require.Equal(test.Duration, cs2.Duration())
		require.True(cs2.AutoStake())

		// verify sign
		bd := &EnvelopeBuilder{}
		elp := bd.SetGasLimit(test.GasLimit).
			SetGasPrice(test.GasPrice).
			SetAction(stake).Build()
		// sign
		selp, err := Sign(elp, test.SenderKey)
		require.NoError(err)
		require.NotNil(selp)
		ser, err = proto.Marshal(selp.Proto())
		require.NoError(err)
		require.Equal(test.Sign, hex.EncodeToString(ser))
		hash, err := selp.Hash()
		require.NoError(err)
		require.Equal(test.SelpHash, hex.EncodeToString(hash[:]))
		// verify signature
		require.NoError(selp.VerifySignature())
	}

}

func TestCreateStakeABIEncodeAndDecode(t *testing.T) {
	require := require.New(t)
	test := stakeCreateTestParams[0]
	stake, err := NewCreateStake(test.Nonce, test.CanAddress, test.AmountStr, test.Duration, test.AutoStake, test.Payload, test.GasLimit, test.GasPrice)
	require.NoError(err)

	data, err := stake.EncodeABIBinary()
	require.NoError(err)
	stake, err = NewCreateStakeFromABIBinary(data)
	require.NoError(err)
	require.Equal(test.CanAddress, stake.candName)
	require.Equal(test.AmountStr, stake.amount.String())
	require.Equal(test.Duration, stake.duration)
	require.Equal(test.AutoStake, stake.autoStake)
	require.Equal(test.Payload, stake.payload)
}
