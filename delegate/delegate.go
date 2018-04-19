// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package delegate

import (
	"net"

	cm "github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/common/service"
	"github.com/iotexproject/iotex-core/config"
)

// Pool is the interface
type Pool interface {
	// AllDelegates returns all delegates
	AllDelegates() ([]net.Addr, error)

	// AnotherDelegate return another delegate that is not the passed-in address
	AnotherDelegate(self string) net.Addr
}

// ConfigBasedPool is the simple delegate pool implementing Pool interface
type ConfigBasedPool struct {
	service.AbstractService
	delegates []net.Addr
}

// NewConfigBasedPool creates an instance of config-based delegate pool
func NewConfigBasedPool(delegate *config.Delegate) *ConfigBasedPool {
	cbdp := &ConfigBasedPool{}
	encountered := map[string]bool{} // dedup
	for _, addr := range delegate.Addrs {
		if encountered[addr] == false {
			encountered[addr] = true
			cbdp.delegates = append(cbdp.delegates, cm.NewTCPNode(addr))
		}
	}
	return cbdp
}

// AllDelegates implements getting the delegates from config
func (cbdp *ConfigBasedPool) AllDelegates() ([]net.Addr, error) {
	return cbdp.delegates, nil
}

// AnotherDelegate return the first delegate that is not the passed-in address
func (cbdp *ConfigBasedPool) AnotherDelegate(self string) net.Addr {
	for _, v := range cbdp.delegates {
		if self != v.String() {
			return v
		}
	}
	return nil
}
