// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package genesis

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	// construct a config without overriding
	cfg, err := New("")
	require.NoError(t, err)
	// Validate blockchain
	assert.Equal(t, Default.BlockGasLimit, cfg.BlockGasLimit)
	assert.Equal(t, Default.ActionGasLimit, cfg.ActionGasLimit)
	assert.Equal(t, Default.NumSubEpochs, cfg.NumSubEpochs)
	assert.Equal(t, Default.NumDelegates, cfg.NumDelegates)
	// Validate rewarding protocol)
	assert.Equal(t, Default.BlockReward(), cfg.BlockReward())
	assert.Equal(t, Default.EpochReward(), cfg.EpochReward())
	assert.Equal(t, Default.FoundationBonus(), cfg.FoundationBonus())
}
func TestHash(t *testing.T) {
	require := require.New(t)
	cfg, err := New("")
	require.NoError(err)
	hash := cfg.Hash()
	require.Equal("3dfcdee76186b59a9f9abd0ded8e6c093c35bddea23834044550fb68626adb62", hex.EncodeToString(hash[:]))
}
func TestAccount_InitBalances(t *testing.T) {
	require := require.New(t)
	InitBalanceMap := make(map[string]string, 0)
	InitBalanceMap["io1emxf8zzqckhgjde6dqd97ts0y3q496gm3fdrl6"] = "1"
	InitBalanceMap["io1mflp9m6hcgm2qcghchsdqj3z3eccrnekx9p0ms"] = "2"
	acc := Account{InitBalanceMap}
	adds, balances := acc.InitBalances()
	require.Equal("io1emxf8zzqckhgjde6dqd97ts0y3q496gm3fdrl6", adds[0].String())
	require.Equal("io1mflp9m6hcgm2qcghchsdqj3z3eccrnekx9p0ms", adds[1].String())
	require.Equal(InitBalanceMap["io1emxf8zzqckhgjde6dqd97ts0y3q496gm3fdrl6"], balances[0].Text(10))
	require.Equal(InitBalanceMap["io1mflp9m6hcgm2qcghchsdqj3z3eccrnekx9p0ms"], balances[1].Text(10))
}
