// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"context"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/state/factory"
)

func TestProtocol_Fund(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, stateDB factory.Factory, p *Protocol) {
		raCtx, ok := protocol.GetRunActionsCtx(ctx)
		require.True(t, ok)

		// Deposit 5 token
		ws, err := stateDB.NewWorkingSet()
		require.NoError(t, err)
		require.NoError(t, p.Deposit(ctx, ws, big.NewInt(5)))
		require.NoError(t, stateDB.Commit(ws))

		ws, err = stateDB.NewWorkingSet()
		require.NoError(t, err)
		totalBalance, err := p.TotalBalance(ctx, ws)
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(5), totalBalance)
		availableBalance, err := p.AvailableBalance(ctx, ws)
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(5), availableBalance)
		acc, err := accountutil.LoadAccount(ws, hash.BytesToHash160(raCtx.Caller.Bytes()))
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(995), acc.Balance)

		// Deposit another 6 token will fail because
		ws, err = stateDB.NewWorkingSet()
		require.NoError(t, err)
		require.Error(t, p.Deposit(ctx, ws, big.NewInt(996)))
	}, false)

}
