// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package delegate

import (
	"errors"
	"net"

	"github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/common/service"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/crypto"
)

var (
	// ErrZeroDelegate indicates seeing 0 delegates in the network
	ErrZeroDelegate = errors.New("zero delegates in the network")
)

// Pool is the interface
type Pool interface {
	// AllDelegates returns all delegates
	AllDelegates() ([]net.Addr, error)

	// RollDelegates returns rolling delegates
	RollDelegates(uint64) ([]net.Addr, error)

	// AnotherDelegate return another delegate that is not the passed-in address
	AnotherDelegate(self string) net.Addr

	// NumDelegatesPerEpoch returns number of delegates per epoch
	NumDelegatesPerEpoch() (uint, error)
}

// ConfigBasedPool is the simple delegate pool implementing Pool interface
type ConfigBasedPool struct {
	service.AbstractService
	cfg       *config.Delegate
	delegates []net.Addr
}

// NewConfigBasedPool creates an instance of config-based delegate pool
func NewConfigBasedPool(cfg *config.Delegate) *ConfigBasedPool {
	cbdp := &ConfigBasedPool{cfg: cfg}
	encountered := map[string]bool{} // dedup
	for _, addr := range cfg.Addrs {
		if !encountered[addr] {
			encountered[addr] = true
			cbdp.delegates = append(cbdp.delegates, common.NewTCPNode(addr))
		}
	}
	return cbdp
}

// AllDelegates implements getting the delegates from config
func (cbdp *ConfigBasedPool) AllDelegates() ([]net.Addr, error) {
	o := make([]net.Addr, len(cbdp.delegates))
	copy(o, cbdp.delegates)
	return o, nil
}

// RollDelegates implements getting the rolling delegates per epoch
func (cbdp *ConfigBasedPool) RollDelegates(epochNum uint64) ([]net.Addr, error) {
	if cbdp.cfg.RollNum == 0 {
		return cbdp.AllDelegates()
	}

	// Generate a random delegates slice based on the crypto sort
	hs := make([][]byte, len(cbdp.delegates))
	h2a := make(map[string]net.Addr)
	for i, d := range cbdp.delegates {
		aStr := d.String()
		hs[i] = []byte(aStr)
		h2a[aStr] = d
	}
	crypto.Sort(hs, epochNum)
	o := make([]net.Addr, len(cbdp.delegates))
	for i, h := range hs {
		o[i] = h2a[string(h)]
	}

	// Get the first RollNum as the rolling delegates
	return o[:cbdp.cfg.RollNum], nil
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

// NumDelegatesPerEpoch returns the configured rolling delegates number or all delegates number if 0
func (cbdp *ConfigBasedPool) NumDelegatesPerEpoch() (uint, error) {
	if cbdp.cfg.RollNum == 0 {
		return uint(len(cbdp.delegates)), nil
	}
	return cbdp.cfg.RollNum, nil
}
