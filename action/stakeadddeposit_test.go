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

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/go-pkgs/crypto"

	"github.com/iotexproject/iotex-core/test/identityset"
)

var stakeDepositTestParams = []struct {
	SenderKey    crypto.PrivateKey
	Nonce        uint64
	Index        uint64
	Amount       string
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
		identityset.PrivateKey(27), uint64(0), uint64(10), "10", []byte("payload"), uint64(1000000), big.NewInt(10), "080a120231301a077061796c6f6164", uint64(10700), "107010", "9089e7eb1afed64fcdbd3c7ee29a6cedab9aa59cf3f7881dfaa3d19f99f09338", "0a1c080118c0843d22023130da020f080a120231301a077061796c6f6164124104755ce6d8903f6b3793bddb4ea5d3589d637de2d209ae0ea930815c82db564ee8cc448886f639e8a0c7e94e99a5c1335b583c0bc76ef30dd6a1038ed9da8daf331a41a48ab1feba8181d760de946aefed7d815a89fd9b1ab503d2392bb55e1bb75eec42dddc8bd642f89accc3a37b3cf15a103a95d66695fdf0647b202869fdd66bcb01", "ca8937d6f224a4e4bf93cb5605581de2d26fb0481e1dfc1eef384ee7ccf94b73", nil, nil,
	},
	// invalid test
	{
		identityset.PrivateKey(27), uint64(0), uint64(10), "abci", []byte("payload"), uint64(1000000), big.NewInt(1000), "", uint64(10700), "", "", "", "", ErrInvalidAmount, nil,
	},
	{
		identityset.PrivateKey(27), uint64(0), uint64(10), "0", []byte("payload"), uint64(1000000), big.NewInt(1000), "", uint64(10700), "", "", "", "", nil, ErrInvalidAmount,
	},
	{
		identityset.PrivateKey(27), uint64(0), uint64(10), "-10", []byte("payload"), uint64(1000000), big.NewInt(1000), "", uint64(10700), "", "", "", "", nil, ErrInvalidAmount,
	},
	{
		identityset.PrivateKey(27), uint64(0), uint64(10), "10", []byte("payload"), uint64(1000000), big.NewInt(-10), "080a120231301a077061796c6f6164", uint64(10700), "107010", "9089e7eb1afed64fcdbd3c7ee29a6cedab9aa59cf3f7881dfaa3d19f99f09338", "0a1c080118c0843d22023130da020f080a120231301a077061796c6f6164124104755ce6d8903f6b3793bddb4ea5d3589d637de2d209ae0ea930815c82db564ee8cc448886f639e8a0c7e94e99a5c1335b583c0bc76ef30dd6a1038ed9da8daf331a41a48ab1feba8181d760de946aefed7d815a89fd9b1ab503d2392bb55e1bb75eec42dddc8bd642f89accc3a37b3cf15a103a95d66695fdf0647b202869fdd66bcb01", "ca8937d6f224a4e4bf93cb5605581de2d26fb0481e1dfc1eef384ee7ccf94b73", nil, ErrNegativeValue,
	},
}

func TestDeposit(t *testing.T) {
	require := require.New(t)
	for _, test := range stakeDepositTestParams {
		stake, err := NewDepositToStake(test.Nonce, test.Index, test.Amount, test.Payload, test.GasLimit, test.GasPrice)
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

		require.Equal(test.Amount, stake.Amount().String())
		require.Equal(test.Payload, stake.Payload())
		require.Equal(test.Index, stake.BucketIndex())

		gas, err := stake.IntrinsicGas()
		require.NoError(err)
		require.Equal(test.IntrinsicGas, gas)
		cost, err := stake.Cost()
		require.NoError(err)
		require.Equal(test.Cost, cost.Text(10))

		ds2 := &DepositToStake{}
		require.NoError(ds2.LoadProto(stake.Proto()))
		require.Equal(test.Amount, ds2.Amount().String())
		require.Equal(test.Payload, ds2.Payload())
		require.Equal(test.Index, ds2.BucketIndex())

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

func TestDepositToStakeABIEncodeAndDecode(t *testing.T) {
	require := require.New(t)
	test := stakeDepositTestParams[0]
	stake, err := NewDepositToStake(test.Nonce, test.Index, test.Amount, test.Payload, test.GasLimit, test.GasPrice)
	require.NoError(err)

	data, err := stake.EncodeABIBinary()
	require.NoError(err)
	stake, err = NewDepositToStakeFromABIBinary(data)
	require.NoError(err)
	require.Equal(test.Index, stake.bucketIndex)
	require.Equal(test.Amount, stake.amount.String())
	require.Equal(test.Payload, stake.payload)
}
