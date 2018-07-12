// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package delegate

import (
	"errors"

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
	AllDelegates() ([]string, error)

	// RollDelegates returns rolling delegates
	RollDelegates(uint64) ([]string, error)

	// AnotherDelegate return another delegate that is not the passed-in address
	AnotherDelegate(self string) string

	// NumDelegatesPerEpoch returns number of delegates per epoch
	NumDelegatesPerEpoch() (uint, error)
}

// ConfigBasedPool is the simple delegate pool implementing Pool interface
type ConfigBasedPool struct {
	cfg       *config.Delegate
	delegates []string
}

// NewConfigBasedPool creates an instance of config-based delegate pool
func NewConfigBasedPool(cfg *config.Delegate) *ConfigBasedPool {
	cbdp := &ConfigBasedPool{cfg: cfg}
	encountered := map[string]bool{} // dedup
	for _, addr := range cfg.Addrs {
		if !encountered[addr] {
			encountered[addr] = true
			cbdp.delegates = append(cbdp.delegates, addr)
		}
	}
	return cbdp
}

// AllDelegates implements getting the delegates from config
func (cbdp *ConfigBasedPool) AllDelegates() ([]string, error) {
	o := make([]string, len(cbdp.delegates))
	copy(o, cbdp.delegates)
	return o, nil
}

// RollDelegates implements getting the rolling delegates per epoch
func (cbdp *ConfigBasedPool) RollDelegates(epochNum uint64) ([]string, error) {
	if cbdp.cfg.RollNum == 0 {
		return cbdp.AllDelegates()
	}

	// Generate a random delegates slice based on the crypto sort
	hs := make([][]byte, len(cbdp.delegates))
	for i, d := range cbdp.delegates {
		hs[i] = []byte(d)
	}
	crypto.Sort(hs, epochNum)
	o := make([]string, len(cbdp.delegates))
	for i, h := range hs {
		o[i] = string(h)
	}

	// Get the first RollNum as the rolling delegates
	return o[:cbdp.cfg.RollNum], nil
}

// AnotherDelegate return the first delegate that is not the passed-in address
func (cbdp *ConfigBasedPool) AnotherDelegate(self string) string {
	for _, v := range cbdp.delegates {
		if self != v {
			return v
		}
	}
	return ""
}

// NumDelegatesPerEpoch returns the configured rolling delegates number or all delegates number if 0
func (cbdp *ConfigBasedPool) NumDelegatesPerEpoch() (uint, error) {
	if cbdp.cfg.RollNum == 0 {
		return uint(len(cbdp.delegates)), nil
	}
	return cbdp.cfg.RollNum, nil
}
