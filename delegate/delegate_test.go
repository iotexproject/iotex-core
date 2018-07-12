// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
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

var (
	addrs = []string{
		"io1qyqsyqcy6nm58gjd2wr035wz5eyd5uq47zyqpng3gxe7nh",
		"io1qyqsyqcy6m6hkqkj3f4w4eflm2gzydmvc0mumm7kgax4l3",
		"io1qyqsyqcyyu9pfazcx0wglp35h2h4fm0hl8p8z2u35vkcwc",
		"io1qyqsyqcyg9pk8zg8xzkmv6g3630xggvacq9e77cwtd4rkc",
	}
)

func TestConfigBasedPool_AllDelegates(t *testing.T) {
	cfg := config.Config{}
	for i := 0; i < 4; i++ {
		cfg.Delegate.Addrs = append(cfg.Delegate.Addrs, addrs[i])
	}

	// duplicates should be ignored
	for i := 0; i < 4; i++ {
		cfg.Delegate.Addrs = append(cfg.Delegate.Addrs, addrs[i])
	}

	cbdp := NewConfigBasedPool(&cfg.Delegate)

	delegates, err := cbdp.AllDelegates()
	require.Nil(t, err)
	require.Equal(t, 4, len(delegates))
	for i := 0; i < 4; i++ {
		require.Equal(t, addrs[i], delegates[i])
	}

	other := cbdp.AnotherDelegate("io1qyqsyqcy6nm58gjd2wr035wz5eyd5uq47zyqpng3gxe7nh")
	require.Equal(t, fmt.Sprintf("io1qyqsyqcy6m6hkqkj3f4w4eflm2gzydmvc0mumm7kgax4l3"), other)
}

func TestConfigBasedPool_RollDelegates(t *testing.T) {
	cfg := config.Config{
		Delegate: config.Delegate{
			RollNum: 2,
		},
	}
	for i := 0; i < 4; i++ {
		cfg.Delegate.Addrs = append(cfg.Delegate.Addrs, addrs[i])
	}

	cbdp := NewConfigBasedPool(&cfg.Delegate)

	dlgts1, err := cbdp.RollDelegates(uint64(1))
	require.Nil(t, err)
	require.Equal(t, 2, len(dlgts1))

	dlgts2, err := cbdp.RollDelegates(uint64(1))
	require.Nil(t, err)
	require.Equal(t, 2, len(dlgts2))

	// delegates should be same for the same epoch ordinal number
	for i := 0; i < 2; i++ {
		require.Equal(t, dlgts1[i], dlgts2[i])
	}

	dlgts3, err := cbdp.RollDelegates(uint64(2))
	require.Nil(t, err)
	require.Equal(t, 2, len(dlgts3))

	// delegates should be different between epoch 1 and 2
	diffCnt := 0
	for i := 0; i < 2; i++ {
		if dlgts1[i] != dlgts3[i] {
			diffCnt++
		}
	}
	require.True(t, diffCnt > 0)

	dlgts4, err := cbdp.RollDelegates(uint64(3))
	require.Nil(t, err)
	require.Equal(t, 2, len(dlgts4))

	// delegates should be different between epoch 1 and 3
	diffCnt = 0
	for i := 0; i < 2; i++ {
		if dlgts1[i] != dlgts4[i] {
			diffCnt++
		}
	}
	require.True(t, diffCnt > 0)

	// delegates should be different between epoch 2 and 3
	diffCnt = 0
	for i := 0; i < 2; i++ {
		if dlgts3[i] != dlgts4[i] {
			diffCnt++
		}
	}
	require.True(t, diffCnt > 0)
}

func TestConfigBasedPool_NumDelegates(t *testing.T) {
	cfg := config.Config{}
	for i := 0; i < 4; i++ {
		cfg.Delegate.Addrs = append(cfg.Delegate.Addrs, addrs[i])
	}

	cbdp := NewConfigBasedPool(&cfg.Delegate)

	num, err := cbdp.NumDelegatesPerEpoch()
	require.Nil(t, err)
	require.Equal(t, uint(4), num)

	cfg.Delegate.RollNum = 4
	num, err = cbdp.NumDelegatesPerEpoch()
	require.Nil(t, err)
	require.Equal(t, uint(4), num)
}
