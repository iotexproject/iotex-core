// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package genesis

import (
	"flag"

	"github.com/pkg/errors"
	"go.uber.org/config"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/log"
)

// DefaultAdminPrivateKey is used to create the default genesis config. It could facilitate quick setup of the
// blockchain, but it MUST NOT be used in production.
const DefaultAdminPrivateKey = "bace9b2435db45b119e1570b4ea9c57993b2311e0c408d743d87cd22838ae892"

var (
	// Default contains the default genesis config
	Default Genesis

	genesisPath      string
	defaultAdminAddr address.Address
)

func init() {
	flag.StringVar(&genesisPath, "genesis-path", "", "Genesis path")
	sk, err := keypair.DecodePrivateKey(DefaultAdminPrivateKey)
	if err != nil {
		log.L().Panic("Error when decoding the default admin private key.", zap.Error(err))
	}
	pkHash := keypair.HashPubKey(&sk.PublicKey)
	defaultAdminAddr, err = address.FromBytes(pkHash[:])
	if err != nil {
		log.L().Panic("Error when constructing the default admin address.", zap.Error(err))
	}

	initDefaultConfig()
}

func initDefaultConfig() {
	Default = Genesis{
		Blockchain: Blockchain{
			BlockGasLimit:  20000000,
			ActionGasLimit: 5000000,
		},
		Rewarding: Rewarding{
			InitAdminAddrStr: defaultAdminAddr.String(),
		},
	}
}

type (
	// Genesis is the root level of genesis config. Genesis config is the network-wide blockchain config. All the nodes
	// participating into the same network should use the same genesis config.
	Genesis struct {
		Blockchain `yaml:"blockchain"`
		Rewarding  `yaml:"rewarding"`
	}
	// Blockchain contains blockchain level configs
	Blockchain struct {
		// BlockGasLimit is the total gas limit could be consumed in a block
		BlockGasLimit uint64 `yaml:"blockGasLimit"`
		// ActionGasLimit is the per action gas limit cap
		ActionGasLimit uint64 `yaml:"actionGasLimit"`
	}
	// Rewarding contains the configs for rewarding protocol
	Rewarding struct {
		// InitAdminAddrStr is the address of the initial rewarding protocol admin in encoded string format
		InitAdminAddrStr string `yaml:"initAdminAddr"`
	}
)

// New constructs a genesis config. It loads the default values, and could be overwritten by values defined in the yaml
// config files
func New() (Genesis, error) {
	opts := make([]config.YAMLOption, 0)
	opts = append(opts, config.Static(Default))
	if genesisPath != "" {
		opts = append(opts, config.File(genesisPath))
	}
	yaml, err := config.NewYAML(opts...)
	if err != nil {
		return Genesis{}, errors.Wrap(err, "error when constructing a genesis in yaml")
	}

	var genesis Genesis
	if err := yaml.Get(config.Root).Populate(&genesis); err != nil {
		return Genesis{}, errors.Wrap(err, "failed to unmarshal yaml genesis to struct")
	}
	return genesis, nil
}

// InitAdminAddr returns the address of the initial rewarding protocol admin
func (r *Rewarding) InitAdminAddr() address.Address {
	addr, err := address.FromString(r.InitAdminAddrStr)
	if err != nil {
		log.L().Panic("Error when decoding the rewarding protocol init admin address from string.", zap.Error(err))
	}
	return addr
}
