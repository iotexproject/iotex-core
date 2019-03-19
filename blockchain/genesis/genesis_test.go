// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package genesis

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	// construct a config without overriding
	cfg, err := New()
	require.NoError(t, err)
	// Validate blockchain
	assert.Equal(t, Default.BlockGasLimit, cfg.BlockGasLimit)
	assert.Equal(t, Default.ActionGasLimit, cfg.ActionGasLimit)
	assert.Equal(t, Default.NumSubEpochs, cfg.NumSubEpochs)
	assert.Equal(t, Default.NumDelegates, cfg.NumDelegates)
	// Validate poll protocol
	assert.Equal(t, Default.Delegates, cfg.Delegates)
	assert.Equal(t, cfg.NumDelegates, uint64(len(cfg.Delegates)))
	// Validate account protocol
	eAddrs, eBalances := Default.InitBalances()
	aAddrs, aBalances := cfg.InitBalances()
	assert.Equal(t, eAddrs, aAddrs)
	assert.Equal(t, eBalances, aBalances)
	// Validate rewarding protocol)
	assert.Equal(t, Default.BlockReward(), cfg.BlockReward())
	assert.Equal(t, Default.EpochReward(), cfg.EpochReward())
	assert.Equal(t, Default.FoundationBonus(), cfg.FoundationBonus())
}
