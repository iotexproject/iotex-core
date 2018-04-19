// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package delegate

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iotexproject/iotex-core/config"
)

func TestConfigBasedPool(t *testing.T) {
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
	assert.Nil(t, err)
	assert.Equal(t, 4, len(delegates))
	for i := 0; i < 4; i++ {
		assert.Equal(t, "tcp", delegates[i].Network())
		assert.Equal(t, fmt.Sprintf("127.0.0.1:1000%d", i), delegates[i].String())
	}

	other := cbdp.AnotherDelegate("127.0.0.1:10000")
	assert.Equal(t, fmt.Sprintf("127.0.0.1:10001"), other.String())
}
