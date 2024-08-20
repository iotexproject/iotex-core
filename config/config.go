// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package config

import (
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"
	uconfig "go.uber.org/config"

	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/actsync"
	"github.com/iotexproject/iotex-core/api"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/blockindex"
	"github.com/iotexproject/iotex-core/blocksync"
	"github.com/iotexproject/iotex-core/consensus"
	"github.com/iotexproject/iotex-core/consensus/consensusfsm"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/dispatcher"
	"github.com/iotexproject/iotex-core/nodeinfo"
	"github.com/iotexproject/iotex-core/p2p"
	"github.com/iotexproject/iotex-core/pkg/log"
)

// IMPORTANT: to define a config, add a field or a new config type to the existing config types. In addition, provide
// the default value in Default var.

const (
	// RollDPoSScheme means randomized delegated proof of stake
	RollDPoSScheme = "ROLLDPOS"
	// StandaloneScheme means that the node creates a block periodically regardless of others (if there is any)
	StandaloneScheme = "STANDALONE"
	// NOOPScheme means that the node does not create only block
	NOOPScheme = "NOOP"
)

const (
	// GatewayPlugin is the plugin of accepting user API requests and serving blockchain data to users
	GatewayPlugin = iota
)

type strs []string

func (ss *strs) String() string {
	return strings.Join(*ss, ",")
}

func (ss *strs) Set(str string) error {
	*ss = append(*ss, str)
	return nil
}

// Dardanelles consensus config
var (
	// Default is the default config
	Default = Config{
		Plugins:            make(map[int]interface{}),
		SubLogs:            make(map[string]log.GlobalConfig),
		Network:            p2p.DefaultConfig,
		Chain:              blockchain.DefaultConfig,
		ActPool:            actpool.DefaultConfig,
		Consensus:          consensus.DefaultConfig,
		DardanellesUpgrade: consensusfsm.DefaultDardanellesUpgradeConfig,
		BlockSync:          blocksync.DefaultConfig,
		Dispatcher:         dispatcher.DefaultConfig,
		API:                api.DefaultConfig,
		System: System{
			Active:                true,
			HeartbeatInterval:     10 * time.Second,
			HTTPStatsPort:         8080,
			HTTPAdminPort:         0,
			StartSubChainInterval: 10 * time.Second,
			SystemLogDBPath:       "/var/log",
		},
		DB:         db.DefaultConfig,
		Indexer:    blockindex.DefaultConfig,
		Genesis:    genesis.Default,
		NodeInfo:   nodeinfo.DefaultConfig,
		ActionSync: actsync.DefaultConfig,
	}

	// ErrInvalidCfg indicates the invalid config value
	ErrInvalidCfg = errors.New("invalid config value")

	// Validates is the collection config validation functions
	Validates = []Validate{
		ValidateRollDPoS,
		ValidateArchiveMode,
		ValidateDispatcher,
		ValidateAPI,
		ValidateActPool,
		ValidateForkHeights,
	}
)

// Network is the config struct for network package
type (

	// System is the system config
	System struct {
		// Active is the status of the node. True means active and false means stand-by
		Active            bool          `yaml:"active"`
		HeartbeatInterval time.Duration `yaml:"heartbeatInterval"`
		// HTTPProfilingPort is the port number to access golang performance profiling data of a blockchain node. It is
		// 0 by default, meaning performance profiling has been disabled
		HTTPAdminPort         int           `yaml:"httpAdminPort"`
		HTTPStatsPort         int           `yaml:"httpStatsPort"`
		StartSubChainInterval time.Duration `yaml:"startSubChainInterval"`
		SystemLogDBPath       string        `yaml:"systemLogDBPath"`
		MptrieLogPath         string        `yaml:"mptrieLogPath"`
	}

	// Config is the root config struct, each package's config should be put as its sub struct
	Config struct {
		Plugins            map[int]interface{}             `ymal:"plugins"`
		Network            p2p.Config                      `yaml:"network"`
		Chain              blockchain.Config               `yaml:"chain"`
		ActPool            actpool.Config                  `yaml:"actPool"`
		Consensus          consensus.Config                `yaml:"consensus"`
		DardanellesUpgrade consensusfsm.DardanellesUpgrade `yaml:"dardanellesUpgrade"`
		BlockSync          blocksync.Config                `yaml:"blockSync"`
		Dispatcher         dispatcher.Config               `yaml:"dispatcher"`
		API                api.Config                      `yaml:"api"`
		System             System                          `yaml:"system"`
		DB                 db.Config                       `yaml:"db"`
		Indexer            blockindex.Config               `yaml:"indexer"`
		Log                log.GlobalConfig                `yaml:"log"`
		SubLogs            map[string]log.GlobalConfig     `yaml:"subLogs"`
		Genesis            genesis.Genesis                 `yaml:"genesis"`
		NodeInfo           nodeinfo.Config                 `yaml:"nodeinfo"`
		ActionSync         actsync.Config                  `yaml:"actionSync"`
	}

	// Validate is the interface of validating the config
	Validate func(Config) error
)

// New creates a config instance. It first loads the default configs. If the config path is not empty, it will read from
// the file and override the default configs. By default, it will apply all validation functions. To bypass validation,
// use DoNotValidate instead.
func New(configPaths []string, _plugins []string, validates ...Validate) (Config, error) {
	opts := make([]uconfig.YAMLOption, 0)
	opts = append(opts, uconfig.Static(Default))
	opts = append(opts, uconfig.Expand(os.LookupEnv))
	for _, path := range configPaths {
		if path != "" {
			opts = append(opts, uconfig.File(path))
		}
	}
	yaml, err := uconfig.NewYAML(opts...)
	if err != nil {
		return Config{}, errors.Wrap(err, "failed to init config")
	}

	var cfg Config
	if err := yaml.Get(uconfig.Root).Populate(&cfg); err != nil {
		return Config{}, errors.Wrap(err, "failed to unmarshal YAML config to struct")
	}

	if err := cfg.Chain.SetProducerPrivKey(); err != nil {
		return Config{}, errors.Wrap(err, "failed to set producer private key")
	}

	// set network master key to private key
	if cfg.Network.MasterKey == "" {
		cfg.Network.MasterKey = cfg.Chain.ProducerPrivKey
	}

	// set plugins
	for _, plugin := range _plugins {
		switch strings.ToLower(plugin) {
		case "gateway":
			cfg.Plugins[GatewayPlugin] = nil
		default:
			return Config{}, errors.Errorf("Plugin %s is not supported", plugin)
		}
	}

	// By default, the config needs to pass all the validation
	if len(validates) == 0 {
		validates = Validates
	}
	for _, validate := range validates {
		if err := validate(cfg); err != nil {
			return Config{}, errors.Wrap(err, "failed to validate config")
		}
	}
	return cfg, nil
}

// NewSub create config for sub chain.
func NewSub(configPaths []string, validates ...Validate) (Config, error) {
	opts := make([]uconfig.YAMLOption, 0)
	opts = append(opts, uconfig.Static(Default))
	opts = append(opts, uconfig.Expand(os.LookupEnv))
	for _, path := range configPaths {
		if path != "" {
			opts = append(opts, uconfig.File(path))
		}
	}
	yaml, err := uconfig.NewYAML(opts...)
	if err != nil {
		return Config{}, errors.Wrap(err, "failed to init config")
	}

	var cfg Config
	if err := yaml.Get(uconfig.Root).Populate(&cfg); err != nil {
		return Config{}, errors.Wrap(err, "failed to unmarshal YAML config to struct")
	}

	// By default, the config needs to pass all the validation
	if len(validates) == 0 {
		validates = Validates
	}
	for _, validate := range validates {
		if err := validate(cfg); err != nil {
			return Config{}, errors.Wrap(err, "failed to validate config")
		}
	}
	return cfg, nil
}

// ValidateDispatcher validates the dispatcher configs
func ValidateDispatcher(cfg Config) error {
	if cfg.Dispatcher.ActionChanSize <= 0 || cfg.Dispatcher.BlockChanSize <= 0 || cfg.Dispatcher.BlockSyncChanSize <= 0 {
		return errors.Wrap(ErrInvalidCfg, "dispatcher chan size should be greater than 0")
	}

	if cfg.Dispatcher.ProcessSyncRequestInterval < 0 {
		return errors.Wrap(ErrInvalidCfg, "dispatcher processSyncRequestInterval should not be less than 0")
	}
	return nil
}

// ValidateRollDPoS validates the roll-DPoS configs
func ValidateRollDPoS(cfg Config) error {
	if cfg.Consensus.Scheme != RollDPoSScheme {
		return nil
	}
	rollDPoS := cfg.Consensus.RollDPoS
	fsm := rollDPoS.FSM
	if fsm.EventChanSize <= 0 {
		return errors.Wrap(ErrInvalidCfg, "roll-DPoS event chan size should be greater than 0")
	}
	return nil
}

// ValidateArchiveMode validates the state factory setting
func ValidateArchiveMode(cfg Config) error {
	if !cfg.Chain.EnableArchiveMode || !cfg.Chain.EnableTrielessStateDB {
		return nil
	}

	return errors.Wrap(ErrInvalidCfg, "Archive mode is incompatible with trieless state DB")
}

// ValidateAPI validates the api configs
func ValidateAPI(cfg Config) error {
	if cfg.API.TpsWindow <= 0 {
		return errors.Wrap(ErrInvalidCfg, "tps window is not a positive integer when the api is enabled")
	}
	return nil
}

// ValidateActPool validates the given config
func ValidateActPool(cfg Config) error {
	maxNumActPerPool := cfg.ActPool.MaxNumActsPerPool
	maxNumActPerAcct := cfg.ActPool.MaxNumActsPerAcct
	if maxNumActPerPool <= 0 || maxNumActPerAcct <= 0 {
		return errors.Wrap(
			ErrInvalidCfg,
			"maximum number of actions per pool or per account cannot be zero or negative",
		)
	}
	if maxNumActPerPool < maxNumActPerAcct {
		return errors.Wrap(
			ErrInvalidCfg,
			"maximum number of actions per pool cannot be less than maximum number of actions per account",
		)
	}
	return nil
}

// ValidateForkHeights validates the forked heights
func ValidateForkHeights(cfg Config) error {
	hu := cfg.Genesis
	switch {
	case hu.PacificBlockHeight > hu.AleutianBlockHeight:
		return errors.Wrap(ErrInvalidCfg, "Pacific is heigher than Aleutian")
	case hu.AleutianBlockHeight > hu.BeringBlockHeight:
		return errors.Wrap(ErrInvalidCfg, "Aleutian is heigher than Bering")
	case hu.BeringBlockHeight > hu.CookBlockHeight:
		return errors.Wrap(ErrInvalidCfg, "Bering is heigher than Cook")
	case hu.CookBlockHeight > hu.DardanellesBlockHeight:
		return errors.Wrap(ErrInvalidCfg, "Cook is heigher than Dardanelles")
	case hu.DardanellesBlockHeight > hu.DaytonaBlockHeight:
		return errors.Wrap(ErrInvalidCfg, "Dardanelles is heigher than Daytona")
	case hu.DaytonaBlockHeight > hu.EasterBlockHeight:
		return errors.Wrap(ErrInvalidCfg, "Daytona is heigher than Easter")
	case hu.EasterBlockHeight > hu.FbkMigrationBlockHeight:
		return errors.Wrap(ErrInvalidCfg, "Easter is heigher than FairbankMigration")
	case hu.FbkMigrationBlockHeight > hu.FairbankBlockHeight:
		return errors.Wrap(ErrInvalidCfg, "FairbankMigration is heigher than Fairbank")
	case hu.FairbankBlockHeight > hu.GreenlandBlockHeight:
		return errors.Wrap(ErrInvalidCfg, "Fairbank is heigher than Greenland")
	case hu.GreenlandBlockHeight > hu.IcelandBlockHeight:
		return errors.Wrap(ErrInvalidCfg, "Greenland is heigher than Iceland")
	case hu.IcelandBlockHeight > hu.JutlandBlockHeight:
		return errors.Wrap(ErrInvalidCfg, "Iceland is heigher than Jutland")
	case hu.JutlandBlockHeight > hu.KamchatkaBlockHeight:
		return errors.Wrap(ErrInvalidCfg, "Jutland is heigher than Kamchatka")
	case hu.KamchatkaBlockHeight > hu.LordHoweBlockHeight:
		return errors.Wrap(ErrInvalidCfg, "Kamchatka is heigher than LordHowe")
	case hu.LordHoweBlockHeight > hu.MidwayBlockHeight:
		return errors.Wrap(ErrInvalidCfg, "LordHowe is heigher than Midway")
	case hu.MidwayBlockHeight > hu.NewfoundlandBlockHeight:
		return errors.Wrap(ErrInvalidCfg, "Midway is heigher than Newfoundland")
	case hu.NewfoundlandBlockHeight > hu.OkhotskBlockHeight:
		return errors.Wrap(ErrInvalidCfg, "Newfoundland is heigher than Okhotsk")
	case hu.OkhotskBlockHeight > hu.PalauBlockHeight:
		return errors.Wrap(ErrInvalidCfg, "Okhotsk is heigher than Palau")
	case hu.PalauBlockHeight > hu.QuebecBlockHeight:
		return errors.Wrap(ErrInvalidCfg, "Palau is heigher than Quebec")
	case hu.QuebecBlockHeight > hu.RedseaBlockHeight:
		return errors.Wrap(ErrInvalidCfg, "Quebec is heigher than Redsea")
	case hu.RedseaBlockHeight > hu.SumatraBlockHeight:
		return errors.Wrap(ErrInvalidCfg, "Redsea is heigher than Sumatra")
	case hu.SumatraBlockHeight > hu.TsunamiBlockHeight:
		return errors.Wrap(ErrInvalidCfg, "Sumatra is heigher than Tsunami")
	case hu.TsunamiBlockHeight > hu.UpernavikBlockHeight:
		return errors.Wrap(ErrInvalidCfg, "Tsunami is heigher than Upernavik")
	case hu.UpernavikBlockHeight > hu.VanuatuBlockHeight:
		return errors.Wrap(ErrInvalidCfg, "Upernavik is heigher than Vanuatu")
	}
	return nil
}

// DoNotValidate validates the given config
func DoNotValidate(cfg Config) error { return nil }
