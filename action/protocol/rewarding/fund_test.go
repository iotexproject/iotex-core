// Copyright (c) 2019 IoTeX Foundation
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

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
)

func TestProtocol_Fund(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		actionCtx, ok := protocol.GetActionCtx(ctx)
		require.True(t, ok)

		// Deposit 5 token
		require.NoError(t, p.Deposit(ctx, sm, big.NewInt(5)))

		totalBalance, err := p.TotalBalance(ctx, sm)
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(5), totalBalance)
		availableBalance, err := p.AvailableBalance(ctx, sm)
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(5), availableBalance)
		acc, err := accountutil.LoadAccount(sm, hash.BytesToHash160(actionCtx.Caller.Bytes()))
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(995), acc.Balance)

		// Deposit another 6 token will fail because
		require.Error(t, p.Deposit(ctx, sm, big.NewInt(996)))
	}, false)

}

func TestDepositNegativeGasFee(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		r := protocol.NewRegistry()
		r.Register(ProtocolID, p)

		require.Error(t, DepositGas(ctx, sm, big.NewInt(-1), r))
	}, false)
}
