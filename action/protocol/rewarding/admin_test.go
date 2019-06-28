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

	"github.com/iotexproject/iotex-core/state/factory"
)

func TestProtocol_SetEpochReward(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, stateDB factory.Factory, p *Protocol) {
		ws, err := stateDB.NewWorkingSet()
		require.NoError(t, err)
		amount, err := p.EpochReward(ctx, ws)
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(100), amount)

		ws, err = stateDB.NewWorkingSet()
		require.NoError(t, err)
		require.NoError(t, p.SetReward(ctx, ws, big.NewInt(200), false))
		require.NoError(t, stateDB.Commit(ws))

		ws, err = stateDB.NewWorkingSet()
		require.NoError(t, err)
		amount, err = p.EpochReward(ctx, ws)
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(200), amount)

	}, false)
}
