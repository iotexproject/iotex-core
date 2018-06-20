// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package delegate

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/config"
)

func TestConfigBasedPool_AllDelegates(t *testing.T) {
	cfg := config.Config{}
	for i := 0; i < 4; i++ {
		cfg.Delegate.Addrs = append(cfg.Delegate.Addrs, fmt.Sprintf("127.0.0.1:1000%d", i))
	}

	// duplicates should be ignored
	for i := 0; i < 4; i++ {
		cfg.Delegate.Addrs = append(cfg.Delegate.Addrs, fmt.Sprintf("127.0.0.1:1000%d", i))
	}

	cbdp := NewConfigBasedPool(&cfg.Delegate)
	cbdp.Init()
	cbdp.Start()
	defer cbdp.Stop()

	delegates, err := cbdp.AllDelegates()
	require.Nil(t, err)
	require.Equal(t, 4, len(delegates))
	for i := 0; i < 4; i++ {
		require.Equal(t, "tcp", delegates[i].Network())
		require.Equal(t, fmt.Sprintf("127.0.0.1:1000%d", i), delegates[i].String())
	}

	other := cbdp.AnotherDelegate("127.0.0.1:10000")
	require.Equal(t, fmt.Sprintf("127.0.0.1:10001"), other.String())
}

func TestConfigBasedPool_RollDelegates(t *testing.T) {
	cfg := config.Config{
		Delegate: config.Delegate{
			RollNum: 7,
		},
	}
	for i := 0; i < 21; i++ {
		cfg.Delegate.Addrs = append(cfg.Delegate.Addrs, fmt.Sprintf("127.0.0.1:1000%d", i))
	}

	cbdp := NewConfigBasedPool(&cfg.Delegate)
	cbdp.Init()
	cbdp.Start()
	defer cbdp.Stop()

	dlgts1, err := cbdp.RollDelegates(uint64(1))
	require.Nil(t, err)
	require.Equal(t, 7, len(dlgts1))

	dlgts2, err := cbdp.RollDelegates(uint64(1))
	require.Nil(t, err)
	require.Equal(t, 7, len(dlgts2))

	// delegates should be same for the same epoch ordinal number
	for i := 0; i < 7; i++ {
		require.Equal(t, dlgts1[i].String(), dlgts2[i].String())
	}

	dlgts3, err := cbdp.RollDelegates(uint64(2))
	require.Nil(t, err)
	require.Equal(t, 7, len(dlgts3))

	// delegates should be different between epoch 1 and 2
	diffCnt := 0
	for i := 0; i < 7; i++ {
		if dlgts1[i].String() != dlgts3[i].String() {
			diffCnt++
		}
	}
	require.True(t, diffCnt > 0)

	dlgts4, err := cbdp.RollDelegates(uint64(3))
	require.Nil(t, err)
	require.Equal(t, 7, len(dlgts4))

	// delegates should be different between epoch 1 and 3
	diffCnt = 0
	for i := 0; i < 7; i++ {
		if dlgts1[i].String() != dlgts4[i].String() {
			diffCnt++
		}
	}
	require.True(t, diffCnt > 0)

	// delegates should be different between epoch 2 and 3
	diffCnt = 0
	for i := 0; i < 7; i++ {
		if dlgts3[i].String() != dlgts4[i].String() {
			diffCnt++
		}
	}
	require.True(t, diffCnt > 0)
}

func TestConfigBasedPool_NumDelegates(t *testing.T) {
	cfg := config.Config{}
	for i := 0; i < 21; i++ {
		cfg.Delegate.Addrs = append(cfg.Delegate.Addrs, fmt.Sprintf("127.0.0.1:1000%d", i))
	}

	cbdp := NewConfigBasedPool(&cfg.Delegate)
	cbdp.Init()
	cbdp.Start()
	defer cbdp.Stop()

	num, err := cbdp.NumDelegatesPerEpoch()
	require.Nil(t, err)
	require.Equal(t, uint(21), num)

	cfg.Delegate.RollNum = 4
	num, err = cbdp.NumDelegatesPerEpoch()
	require.Nil(t, err)
	require.Equal(t, uint(4), num)
}
