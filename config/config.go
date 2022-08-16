// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package config

import (
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"
	uconfig "go.uber.org/config"

	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/blockindex"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/dispatcher"
	"github.com/iotexproject/iotex-core/p2p"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/tracer"
	"github.com/iotexproject/iotex-core/pkg/unit"
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
		Plugins: make(map[int]interface{}),
		SubLogs: make(map[string]log.GlobalConfig),
		Network: p2p.DefaultConfig,
		Chain:   blockchain.DefaultConfig,
		ActPool: actpool.DefaultConfig,
		Consensus: Consensus{
			Scheme: StandaloneScheme,
			RollDPoS: RollDPoS{
				FSM: ConsensusTiming{
					UnmatchedEventTTL:            3 * time.Second,
					UnmatchedEventInterval:       100 * time.Millisecond,
					AcceptBlockTTL:               4 * time.Second,
					AcceptProposalEndorsementTTL: 2 * time.Second,
					AcceptLockEndorsementTTL:     2 * time.Second,
					CommitTTL:                    2 * time.Second,
					EventChanSize:                10000,
				},
				ToleratedOvertime: 2 * time.Second,
				Delay:             5 * time.Second,
				ConsensusDBPath:   "/var/data/consensus.db",
			},
		},
		DardanellesUpgrade: DardanellesUpgrade{
			UnmatchedEventTTL:            2 * time.Second,
			UnmatchedEventInterval:       100 * time.Millisecond,
			AcceptBlockTTL:               2 * time.Second,
			AcceptProposalEndorsementTTL: time.Second,
			AcceptLockEndorsementTTL:     time.Second,
			CommitTTL:                    time.Second,
			BlockInterval:                5 * time.Second,
			Delay:                        2 * time.Second,
		},
		BlockSync: BlockSync{
			Interval:              30 * time.Second,
			ProcessSyncRequestTTL: 10 * time.Second,
			BufferSize:            200,
			IntervalSize:          20,
			MaxRepeat:             3,
			RepeatDecayStep:       1,
		},
		Dispatcher: dispatcher.DefaultConfig,
		API: API{
			UseRDS:        false,
			GRPCPort:      14014,
			HTTPPort:      15014,
			WebSocketPort: 16014,
			TpsWindow:     10,
			GasStation: GasStation{
				SuggestBlockWindow: 20,
				DefaultGas:         uint64(unit.Qev),
				Percentile:         60,
			},
			RangeQueryLimit: 1000,
		},
		System: System{
			Active:                true,
			HeartbeatInterval:     10 * time.Second,
			HTTPStatsPort:         8080,
			HTTPAdminPort:         9009,
			StartSubChainInterval: 10 * time.Second,
			SystemLogDBPath:       "/var/log",
		},
		DB:      db.DefaultConfig,
		Indexer: blockindex.DefaultConfig,
		Genesis: genesis.Default,
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
	// Consensus is the config struct for consensus package
	Consensus struct {
		// There are three schemes that are supported
		Scheme   string   `yaml:"scheme"`
		RollDPoS RollDPoS `yaml:"rollDPoS"`
	}

	// BlockSync is the config struct for the BlockSync
	BlockSync struct {
		Interval              time.Duration `yaml:"interval"` // update duration
		ProcessSyncRequestTTL time.Duration `yaml:"processSyncRequestTTL"`
		BufferSize            uint64        `yaml:"bufferSize"`
		IntervalSize          uint64        `yaml:"intervalSize"`
		// MaxRepeat is the maximal number of repeat of a block sync request
		MaxRepeat int `yaml:"maxRepeat"`
		// RepeatDecayStep is the step for repeat number decreasing by 1
		RepeatDecayStep int `yaml:"repeatDecayStep"`
	}

	// DardanellesUpgrade is the config for dardanelles upgrade
	DardanellesUpgrade struct {
		UnmatchedEventTTL            time.Duration `yaml:"unmatchedEventTTL"`
		UnmatchedEventInterval       time.Duration `yaml:"unmatchedEventInterval"`
		AcceptBlockTTL               time.Duration `yaml:"acceptBlockTTL"`
		AcceptProposalEndorsementTTL time.Duration `yaml:"acceptProposalEndorsementTTL"`
		AcceptLockEndorsementTTL     time.Duration `yaml:"acceptLockEndorsementTTL"`
		CommitTTL                    time.Duration `yaml:"commitTTL"`
		BlockInterval                time.Duration `yaml:"blockInterval"`
		Delay                        time.Duration `yaml:"delay"`
	}

	// RollDPoS is the config struct for RollDPoS consensus package
	RollDPoS struct {
		FSM               ConsensusTiming `yaml:"fsm"`
		ToleratedOvertime time.Duration   `yaml:"toleratedOvertime"`
		Delay             time.Duration   `yaml:"delay"`
		ConsensusDBPath   string          `yaml:"consensusDBPath"`
	}

	// ConsensusTiming defines a set of time durations used in fsm and event queue size
	ConsensusTiming struct {
		EventChanSize                uint          `yaml:"eventChanSize"`
		UnmatchedEventTTL            time.Duration `yaml:"unmatchedEventTTL"`
		UnmatchedEventInterval       time.Duration `yaml:"unmatchedEventInterval"`
		AcceptBlockTTL               time.Duration `yaml:"acceptBlockTTL"`
		AcceptProposalEndorsementTTL time.Duration `yaml:"acceptProposalEndorsementTTL"`
		AcceptLockEndorsementTTL     time.Duration `yaml:"acceptLockEndorsementTTL"`
		CommitTTL                    time.Duration `yaml:"commitTTL"`
	}

	// API is the api service config
	API struct {
		UseRDS          bool          `yaml:"useRDS"`
		GRPCPort        int           `yaml:"port"`
		HTTPPort        int           `yaml:"web3port"`
		WebSocketPort   int           `yaml:"webSocketPort"`
		RedisCacheURL   string        `yaml:"redisCacheURL"`
		TpsWindow       int           `yaml:"tpsWindow"`
		GasStation      GasStation    `yaml:"gasStation"`
		RangeQueryLimit uint64        `yaml:"rangeQueryLimit"`
		Tracer          tracer.Config `yaml:"tracer"`
	}

	// GasStation is the gas station config
	GasStation struct {
		SuggestBlockWindow int    `yaml:"suggestBlockWindow"`
		DefaultGas         uint64 `yaml:"defaultGas"`
		Percentile         int    `yaml:"Percentile"`
	}

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
	}

	// Config is the root config struct, each package's config should be put as its sub struct
	Config struct {
		Plugins            map[int]interface{}         `ymal:"plugins"`
		Network            p2p.Config                  `yaml:"network"`
		Chain              blockchain.Config           `yaml:"chain"`
		ActPool            actpool.Config              `yaml:"actPool"`
		Consensus          Consensus                   `yaml:"consensus"`
		DardanellesUpgrade DardanellesUpgrade          `yaml:"dardanellesUpgrade"`
		BlockSync          BlockSync                   `yaml:"blockSync"`
		Dispatcher         dispatcher.Config           `yaml:"dispatcher"`
		API                API                         `yaml:"api"`
		System             System                      `yaml:"system"`
		DB                 db.Config                   `yaml:"db"`
		Indexer            blockindex.Config           `yaml:"indexer"`
		Log                log.GlobalConfig            `yaml:"log"`
		SubLogs            map[string]log.GlobalConfig `yaml:"subLogs"`
		Genesis            genesis.Genesis             `yaml:"genesis"`
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
	}
	return nil
}

// DoNotValidate validates the given config
func DoNotValidate(cfg Config) error { return nil }
