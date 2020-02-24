// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/hex"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/require"
)

func TestDeposit(t *testing.T) {
	require := require.New(t)
	stake, err := NewDepositToStake(nonce, index, amount, payload, gaslimit, gasprice)
	require.NoError(err)

	ser := stake.Serialize()
	require.Equal("080a120231301a077061796c6f6164", hex.EncodeToString(ser))

	require.NoError(err)
	require.Equal(gaslimit, stake.GasLimit())
	require.Equal(gasprice, stake.GasPrice())
	require.Equal(nonce, stake.Nonce())

	require.Equal(amount, stake.Amount())
	require.Equal(payload, stake.Payload())
	require.Equal(index, stake.BucketIndex())

	gas, err := stake.IntrinsicGas()
	require.NoError(err)
	require.Equal(uint64(10700), gas)
	cost, err := stake.Cost()
	require.NoError(err)
	require.Equal("107010", cost.Text(10))

	proto := stake.Proto()
	ds2 := &DepositToStake{}
	require.NoError(ds2.LoadProto(proto))
	require.Equal(amount, ds2.Amount())
	require.Equal(payload, ds2.Payload())
	require.Equal(index, ds2.BucketIndex())
}

func TestDepositSignVerify(t *testing.T) {
	require := require.New(t)
	require.Equal("cfa6ef757dee2e50351620dca002d32b9c090cfda55fb81f37f1d26b273743f1", senderKey.HexString())
	stake, err := NewDepositToStake(nonce, index, amount, payload, gaslimit, gasprice)
	require.NoError(err)

	bd := &EnvelopeBuilder{}
	elp := bd.SetGasLimit(gaslimit).
		SetGasPrice(gasprice).
		SetAction(stake).Build()
	h := elp.Hash()
	require.Equal("9089e7eb1afed64fcdbd3c7ee29a6cedab9aa59cf3f7881dfaa3d19f99f09338", hex.EncodeToString(h[:]))
	// sign
	selp, err := Sign(elp, senderKey)
	require.NoError(err)
	require.NotNil(selp)
	ser, err := proto.Marshal(selp.Proto())
	require.NoError(err)
	require.Equal("0a1c080118c0843d22023130da020f080a120231301a077061796c6f6164124104755ce6d8903f6b3793bddb4ea5d3589d637de2d209ae0ea930815c82db564ee8cc448886f639e8a0c7e94e99a5c1335b583c0bc76ef30dd6a1038ed9da8daf331a41a48ab1feba8181d760de946aefed7d815a89fd9b1ab503d2392bb55e1bb75eec42dddc8bd642f89accc3a37b3cf15a103a95d66695fdf0647b202869fdd66bcb01", hex.EncodeToString(ser))
	hash := selp.Hash()
	require.Equal("ca8937d6f224a4e4bf93cb5605581de2d26fb0481e1dfc1eef384ee7ccf94b73", hex.EncodeToString(hash[:]))
	// verify signature
	require.NoError(Verify(selp))
}
