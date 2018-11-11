// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"context"
	"math/big"
	"testing"

	"github.com/CoderZhi/go-ethereum/common"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestAddBalance(t *testing.T) {
	require := require.New(t)
	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)
	defer testutil.CleanupPath(t, testDBPath)
	ctx := context.Background()
	cfg := config.Default
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Explorer.Enabled = true
	bc := NewBlockchain(cfg, DefaultStateFactoryOption(), BoltDBDaoOption())
	require.NoError(bc.Start(ctx))
	require.NotNil(bc)
	defer func() {
		err := bc.Stop(ctx)
		require.NoError(err)
	}()
	sf := bc.GetFactory()
	require.NotNil(sf)
	ws, err := sf.NewWorkingSet()
	require.NoError(err)
	addr := common.HexToAddress("02ae2a956d21e8d481c3a69e146633470cf625ec")
	stateDB := NewEVMStateDBAdapter(bc, ws, 1, hash.ZeroHash32B, uint(0), hash.ZeroHash32B)
	addAmount := big.NewInt(40000)
	stateDB.AddBalance(addr, addAmount)
	amount := stateDB.GetBalance(addr)
	require.Equal(0, amount.Cmp(addAmount))
	stateDB.AddBalance(addr, addAmount)
	amount = stateDB.GetBalance(addr)
	require.Equal(0, amount.Cmp(big.NewInt(80000)))
}
