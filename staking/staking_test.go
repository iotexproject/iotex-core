// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/stretchr/testify/require"
)

func TestStaking(t *testing.T) {
	client, err := ethclient.Dial("wss://mainnet.infura.io/ws")
	require.NoError(t, err)
	sc, err := NewIotxCaller(common.HexToAddress("6fb3e0a217407efff7ca062d46c26e5d60a14d69"), client)
	require.NoError(t, err)
	b, err := sc.BalanceOf(
		&bind.CallOpts{BlockNumber: big.NewInt(7088362)},
		common.HexToAddress("731eae7bEdec1F0A5A52BEb39a4e1dCdb4bb77Ac"),
	)
	require.NoError(t, err)
	require.Equal(t, 0, b.Cmp(big.NewInt(0)))
	b, err = sc.BalanceOf(
		&bind.CallOpts{BlockNumber: big.NewInt(7088363)},
		common.HexToAddress("731eae7bEdec1F0A5A52BEb39a4e1dCdb4bb77Ac"),
	)
	require.NoError(t, err)
	require.Equal(t, 0, b.Cmp(big.NewInt(0).Mul(big.NewInt(1170), big.NewInt(1000000000000000000))))
}
