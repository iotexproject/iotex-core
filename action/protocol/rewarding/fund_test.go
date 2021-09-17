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

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
)

func TestProtocol_Fund(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		actionCtx, ok := protocol.GetActionCtx(ctx)
		require.True(t, ok)

		// Deposit 5 token
		rlog, err := p.Deposit(ctx, sm, big.NewInt(5), iotextypes.TransactionLogType_DEPOSIT_TO_REWARDING_FUND)
		require.NoError(t, err)
		require.NotNil(t, rlog)
		require.Equal(t, big.NewInt(5).String(), rlog.Amount.String())
		require.Equal(t, actionCtx.Caller.String(), rlog.Sender)
		require.Equal(t, address.RewardingPoolAddr, rlog.Recipient)

		totalBalance, _, err := p.TotalBalance(ctx, sm)
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(5), totalBalance)
		availableBalance, _, err := p.AvailableBalance(ctx, sm)
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(5), availableBalance)
		acc, err := accountutil.LoadAccount(sm, actionCtx.Caller)
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(995), acc.Balance)

		// Deposit another 6 token will fail because
		_, err = p.Deposit(ctx, sm, big.NewInt(996), iotextypes.TransactionLogType_DEPOSIT_TO_REWARDING_FUND)
		require.Error(t, err)
	}, false)

}

func TestDepositNegativeGasFee(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		_, err := DepositGas(ctx, sm, big.NewInt(-1))
		require.Error(t, err)
	}, false)
}
