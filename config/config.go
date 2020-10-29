// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package config

import (
	"crypto/ecdsa"
	"flag"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/iotexproject/go-p2p"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-election/committee"
	"github.com/pkg/errors"
	uconfig "go.uber.org/config"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/unit"
)

// IMPORTANT: to define a config, add a field or a new config type to the existing config types. In addition, provide
// the default value in Default var.

func init() {
	flag.StringVar(&_overwritePath, "config-path", "", "Config path")
	flag.StringVar(&_secretPath, "secret-path", "", "Secret path")
	flag.StringVar(&_subChainPath, "sub-config-path", "", "Sub chain Config path")
	flag.Var(&_plugins, "plugin", "Plugin of the node")
}

var (
	// overwritePath is the path to the config file which overwrite default values
	_overwritePath string
	// secretPath is the path to the  config file store secret values
	_secretPath   string
	_subChainPath string
	_plugins      strs
)

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
const (
	DardanellesUnmatchedEventTTL            = 2 * time.Second
	DardanellesUnmatchedEventInterval       = 100 * time.Millisecond
	DardanellesAcceptBlockTTL               = 2 * time.Second
	DardanellesAcceptProposalEndorsementTTL = time.Second
	DardanellesAcceptLockEndorsementTTL     = time.Second
	DardanellesCommitTTL                    = time.Second
	DardanellesBlockInterval                = 5 * time.Second
	DardanellesDelay                        = 2 * time.Second

	SigP256k1  = "secp256k1"
	SigP256sm2 = "p256sm2"
)

var (
	// Default is the default config
	Default = Config{
		Plugins: make(map[int]interface{}),
		SubLogs: make(map[string]log.GlobalConfig),
		Network: Network{
			Host:              "0.0.0.0",
			Port:              4689,
			ExternalHost:      "",
			ExternalPort:      4689,
			BootstrapNodes:    []string{},
			MasterKey:         "",
			RateLimit:         p2p.DefaultRatelimitConfig,
			EnableRateLimit:   true,
			PrivateNetworkPSK: "",
		},
		Chain: Chain{
			ChainDBPath:            "/var/data/chain.db",
			TrieDBPath:             "/var/data/trie.db",
			IndexDBPath:            "/var/data/index.db",
			BloomfilterIndexDBPath: "/var/data/bloomfilter.index.db",
			CandidateIndexDBPath:   "/var/data/candidate.index.db",
			StakingIndexDBPath:     "/var/data/staking.index.db",
			ID:                     1,
			Address:                "",
			ProducerPrivKey:        generateRandomKey(SigP256k1),
			SignatureScheme:        []string{SigP256k1},
			EmptyGenesis:           false,
			GravityChainDB:         DB{DbPath: "/var/data/poll.db", NumRetries: 10},
			Committee: committee.Config{
				GravityChainAPIs: []string{},
			},
			EnableTrielessStateDB:         true,
			EnableStateDBCaching:          false,
			EnableAsyncIndexWrite:         true,
			EnableSystemLogIndexer:        false,
			EnableStakingProtocol:         true,
			EnableStakingIndexer:          false,
			CompressBlock:                 false,
			AllowedBlockGasResidue:        10000,
			MaxCacheSize:                  0,
			PollInitialCandidatesInterval: 10 * time.Second,
			StateDBCacheSize:              1000,
			WorkingSetCacheSize:           20,
			EnableArchiveMode:             false,
			RangeBloomFilterSize:          4096,
		},
		ActPool: ActPool{
			MaxNumActsPerPool:  32000,
			MaxGasLimitPerPool: 320000000,
			MaxNumActsPerAcct:  2000,
			ActionExpiry:       10 * time.Minute,
			MinGasPriceStr:     big.NewInt(unit.Qev).String(),
			BlackList:          []string{},
		},
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
		BlockSync: BlockSync{
			Interval:              10 * time.Second,
			ProcessSyncRequestTTL: 10 * time.Second,
			BufferSize:            200,
			IntervalSize:          20,
			MaxRepeat:             3,
			RepeatDecayStep:       1,
		},
		Dispatcher: Dispatcher{
			EventChanSize: 10000,
		},
		API: API{
			UseRDS:    false,
			Port:      14014,
			TpsWindow: 10,
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
			SystemLogDBPath:       "/var/data/systemlog.db",
		},
		DB: DB{
			NumRetries:          3,
			MaxCacheSize:        64,
			BlockStoreBatchSize: 16,
			Compressor:          "Snappy",
			CompressLegacy:      false,
			SQLITE3: SQLITE3{
				SQLite3File: "./explorer.db",
			},
			SplitDBSizeMB:         0,
			SplitDBHeight:         900000,
			HistoryStateRetention: 2000,
		},
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
	Network struct {
		Host           string   `yaml:"host"`
		Port           int      `yaml:"port"`
		ExternalHost   string   `yaml:"externalHost"`
		ExternalPort   int      `yaml:"externalPort"`
		BootstrapNodes []string `yaml:"bootstrapNodes"`
		MasterKey      string   `yaml:"masterKey"` // master key will be PrivateKey if not set.
		// RelayType is the type of P2P network relay. By default, the value is empty, meaning disabled. Two relay types
		// are supported: active, nat.
		RelayType         string              `yaml:"relayType"`
		RateLimit         p2p.RateLimitConfig `yaml:"rateLimit"`
		EnableRateLimit   bool                `yaml:"enableRateLimit"`
		PrivateNetworkPSK string              `yaml:"privateNetworkPSK"`
	}

	// Chain is the config struct for blockchain package
	Chain struct {
		ChainDBPath            string           `yaml:"chainDBPath"`
		TrieDBPath             string           `yaml:"trieDBPath"`
		IndexDBPath            string           `yaml:"indexDBPath"`
		BloomfilterIndexDBPath string           `yaml:"bloomfilterIndexDBPath"`
		CandidateIndexDBPath   string           `yaml:"candidateIndexDBPath"`
		StakingIndexDBPath     string           `yaml:"stakingIndexDBPath"`
		ID                     uint32           `yaml:"id"`
		Address                string           `yaml:"address"`
		ProducerPrivKey        string           `yaml:"producerPrivKey"`
		SignatureScheme        []string         `yaml:"signatureScheme"`
		EmptyGenesis           bool             `yaml:"emptyGenesis"`
		GravityChainDB         DB               `yaml:"gravityChainDB"`
		Committee              committee.Config `yaml:"committee"`

		EnableTrielessStateDB bool `yaml:"enableTrielessStateDB"`
		// EnableStateDBCaching enables cachedStateDBOption
		EnableStateDBCaching bool `yaml:"enableStateDBCaching"`
		// EnableArchiveMode is only meaningful when EnableTrielessStateDB is false
		EnableArchiveMode bool `yaml:"enableArchiveMode"`
		// EnableAsyncIndexWrite enables writing the block actions' and receipts' index asynchronously
		EnableAsyncIndexWrite bool `yaml:"enableAsyncIndexWrite"`
		// deprecated
		EnableSystemLogIndexer bool `yaml:"enableSystemLog"`
		// EnableStakingProtocol enables staking protocol
		EnableStakingProtocol bool `yaml:"enableStakingProtocol"`
		// EnableStakingIndexer enables staking indexer
		EnableStakingIndexer bool `yaml:"enableStakingIndexer"`
		// deprecated by DB.CompressBlock
		CompressBlock bool `yaml:"compressBlock"`
		// AllowedBlockGasResidue is the amount of gas remained when block producer could stop processing more actions
		AllowedBlockGasResidue uint64 `yaml:"allowedBlockGasResidue"`
		// MaxCacheSize is the max number of blocks that will be put into an LRU cache. 0 means disabled
		MaxCacheSize int `yaml:"maxCacheSize"`
		// PollInitialCandidatesInterval is the config for committee init db
		PollInitialCandidatesInterval time.Duration `yaml:"pollInitialCandidatesInterval"`
		// StateDBCacheSize is the max size of statedb LRU cache
		StateDBCacheSize int `yaml:"stateDBCacheSize"`
		// WorkingSetCacheSize is the max size of workingset cache in state factory
		WorkingSetCacheSize uint64 `yaml:"workingSetCacheSize"`
		// RangeBloomFilterSize is the number of blocks that rangeBloomfilter will store in bloomfilterIndexer
		RangeBloomFilterSize uint64 `yaml:"rangeBloomFilterSize"`
	}

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

	// Dispatcher is the dispatcher config
	Dispatcher struct {
		EventChanSize uint `yaml:"eventChanSize"`
		// TODO: explorer dependency deleted at #1085, need to revive by migrating to api
	}

	// API is the api service config
	API struct {
		UseRDS          bool       `yaml:"useRDS"`
		Port            int        `yaml:"port"`
		TpsWindow       int        `yaml:"tpsWindow"`
		GasStation      GasStation `yaml:"gasStation"`
		RangeQueryLimit uint64     `yaml:"rangeQueryLimit"`
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

	// ActPool is the actpool config
	ActPool struct {
		// MaxNumActsPerPool indicates maximum number of actions the whole actpool can hold
		MaxNumActsPerPool uint64 `yaml:"maxNumActsPerPool"`
		// MaxGasLimitPerPool indicates maximum gas limit the whole actpool can hold
		MaxGasLimitPerPool uint64 `yaml:"maxGasLimitPerPool"`
		// MaxNumActsPerAcct indicates maximum number of actions an account queue can hold
		MaxNumActsPerAcct uint64 `yaml:"maxNumActsPerAcct"`
		// ActionExpiry defines how long an action will be kept in action pool.
		ActionExpiry time.Duration `yaml:"actionExpiry"`
		// MinGasPriceStr defines the minimal gas price the delegate will accept for an action
		MinGasPriceStr string `yaml:"minGasPrice"`
		// BlackList lists the account address that are banned from initiating actions
		BlackList []string `yaml:"blackList"`
	}

	// DB is the config for database
	DB struct {
		DbPath string `yaml:"dbPath"`
		// NumRetries is the number of retries
		NumRetries uint8 `yaml:"numRetries"`
		// MaxCacheSize is the max number of blocks that will be put into an LRU cache. 0 means disabled
		MaxCacheSize int `yaml:"maxCacheSize"`
		// BlockStoreBatchSize is the number of blocks to be stored together as a unit (to get better compression)
		BlockStoreBatchSize int `yaml:"blockStoreBatchSize"`
		// V2BlocksToSplitDB is the accumulated number of blocks to split a new file after v1.1.2
		V2BlocksToSplitDB uint64 `yaml:"v2BlocksToSplitDB"`
		// Compressor is the compression used on block data, used by new DB file after v1.1.2
		Compressor string `yaml:"compressor"`
		// CompressLegacy enables gzip compression on block data, used by legacy DB file before v1.1.2
		CompressLegacy bool `yaml:"compressLegacy"`
		// RDS is the config for rds
		RDS RDS `yaml:"RDS"`
		// SQLite3 is the config for SQLITE3
		SQLITE3 SQLITE3 `yaml:"SQLITE3"`
		// SplitDBSize is the config for DB's split file size
		SplitDBSizeMB uint64 `yaml:"splitDBSizeMB"`
		// SplitDBHeight is the config for DB's split start height
		SplitDBHeight uint64 `yaml:"splitDBHeight"`
		// HistoryStateRetention is the number of blocks account/contract state will be retained
		HistoryStateRetention uint64 `yaml:"historyStateRetention"`
	}

	// RDS is the cloud rds config
	RDS struct {
		// AwsRDSEndpoint is the endpoint of aws rds
		AwsRDSEndpoint string `yaml:"awsRDSEndpoint"`
		// AwsRDSPort is the port of aws rds
		AwsRDSPort uint64 `yaml:"awsRDSPort"`
		// AwsRDSUser is the user to access aws rds
		AwsRDSUser string `yaml:"awsRDSUser"`
		// AwsPass is the pass to access aws rds
		AwsPass string `yaml:"awsPass"`
		// AwsDBName is the db name of aws rds
		AwsDBName string `yaml:"awsDBName"`
	}

	// SQLITE3 is the local sqlite3 config
	SQLITE3 struct {
		// SQLite3File is the sqlite3 db file
		SQLite3File string `yaml:"sqlite3File"`
	}

	// Config is the root config struct, each package's config should be put as its sub struct
	Config struct {
		Plugins    map[int]interface{}         `ymal:"plugins"`
		Network    Network                     `yaml:"network"`
		Chain      Chain                       `yaml:"chain"`
		ActPool    ActPool                     `yaml:"actPool"`
		Consensus  Consensus                   `yaml:"consensus"`
		BlockSync  BlockSync                   `yaml:"blockSync"`
		Dispatcher Dispatcher                  `yaml:"dispatcher"`
		API        API                         `yaml:"api"`
		System     System                      `yaml:"system"`
		DB         DB                          `yaml:"db"`
		Log        log.GlobalConfig            `yaml:"log"`
		SubLogs    map[string]log.GlobalConfig `yaml:"subLogs"`
		Genesis    genesis.Genesis             `yaml:"genesis"`
	}

	// Validate is the interface of validating the config
	Validate func(Config) error
)

// SplitDBSize returns the configured SplitDBSizeMB
func (db DB) SplitDBSize() uint64 {
	return db.SplitDBSizeMB * 1024 * 1024
}

// New creates a config instance. It first loads the default configs. If the config path is not empty, it will read from
// the file and override the default configs. By default, it will apply all validation functions. To bypass validation,
// use DoNotValidate instead.
func New(validates ...Validate) (Config, error) {
	opts := make([]uconfig.YAMLOption, 0)
	opts = append(opts, uconfig.Static(Default))
	opts = append(opts, uconfig.Expand(os.LookupEnv))
	if _overwritePath != "" {
		opts = append(opts, uconfig.File(_overwritePath))
	}
	if _secretPath != "" {
		opts = append(opts, uconfig.File(_secretPath))
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
func NewSub(validates ...Validate) (Config, error) {
	if _subChainPath == "" {
		return Config{}, nil
	}
	opts := make([]uconfig.YAMLOption, 0)
	opts = append(opts, uconfig.Static(Default))
	opts = append(opts, uconfig.Expand(os.LookupEnv))
	opts = append(opts, uconfig.File(_subChainPath))
	if _secretPath != "" {
		opts = append(opts, uconfig.File(_secretPath))
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

// ProducerAddress returns the configured producer address derived from key
func (cfg Config) ProducerAddress() address.Address {
	sk := cfg.ProducerPrivateKey()
	addr, err := address.FromBytes(sk.PublicKey().Hash())
	if err != nil {
		log.L().Panic(
			"Error when constructing producer address",
			zap.Error(err),
		)
	}
	return addr
}

// ProducerPrivateKey returns the configured private key
func (cfg Config) ProducerPrivateKey() crypto.PrivateKey {
	sk, err := crypto.HexStringToPrivateKey(cfg.Chain.ProducerPrivKey)
	if err != nil {
		log.L().Panic(
			"Error when decoding private key",
			zap.Error(err),
		)
	}

	if !cfg.whitelistSignatureScheme(sk) {
		log.L().Panic("The private key's signature scheme is not whitelisted")
	}
	return sk
}

func (cfg Config) whitelistSignatureScheme(sk crypto.PrivateKey) bool {
	var sigScheme string

	switch sk.EcdsaPrivateKey().(type) {
	case *ecdsa.PrivateKey:
		sigScheme = SigP256k1
	case *crypto.P256sm2PrvKey:
		sigScheme = SigP256sm2
	}

	if sigScheme == "" {
		return false
	}
	for _, e := range cfg.Chain.SignatureScheme {
		if sigScheme == e {
			// signature scheme is whitelisted
			return true
		}
	}
	return false
}

func generateRandomKey(scheme string) string {
	// generate a random key
	switch scheme {
	case SigP256k1:
		sk, _ := crypto.GenerateKey()
		return sk.HexString()
	case SigP256sm2:
		sk, _ := crypto.GenerateKeySm2()
		return sk.HexString()
	}
	return ""
}

// MinGasPrice returns the minimal gas price threshold
func (ap ActPool) MinGasPrice() *big.Int {
	mgp, ok := big.NewInt(0).SetString(ap.MinGasPriceStr, 10)
	if !ok {
		log.S().Panicf("Error when parsing minimal gas price string: %s", ap.MinGasPriceStr)
	}
	return mgp
}

// ValidateDispatcher validates the dispatcher configs
func ValidateDispatcher(cfg Config) error {
	if cfg.Dispatcher.EventChanSize <= 0 {
		return errors.Wrap(ErrInvalidCfg, "dispatcher event chan size should be greater than 0")
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
	hu := NewHeightUpgrade(&cfg.Genesis)
	switch {
	case hu.PacificBlockHeight() > hu.AleutianBlockHeight():
		return errors.Wrap(ErrInvalidCfg, "Pacific is heigher than Aleutian")
	case hu.AleutianBlockHeight() > hu.BeringBlockHeight():
		return errors.Wrap(ErrInvalidCfg, "Aleutian is heigher than Bering")
	case hu.BeringBlockHeight() > hu.CookBlockHeight():
		return errors.Wrap(ErrInvalidCfg, "Bering is heigher than Cook")
	case hu.CookBlockHeight() > hu.DardanellesBlockHeight():
		return errors.Wrap(ErrInvalidCfg, "Cook is heigher than Dardanelles")
	case hu.DardanellesBlockHeight() > hu.DaytonaBlockHeight():
		return errors.Wrap(ErrInvalidCfg, "Dardanelles is heigher than Daytona")
	case hu.DaytonaBlockHeight() > hu.EasterBlockHeight():
		return errors.Wrap(ErrInvalidCfg, "Daytona is heigher than Easter")
	case hu.EasterBlockHeight() > hu.FbkMigrationBlockHeight():
		return errors.Wrap(ErrInvalidCfg, "Easter is heigher than FairbankMigration")
	case hu.FbkMigrationBlockHeight() > hu.FairbankBlockHeight():
		return errors.Wrap(ErrInvalidCfg, "FairbankMigration is heigher than Fairbank")
	case hu.FairbankBlockHeight() > hu.GreenlandBlockHeight():
		return errors.Wrap(ErrInvalidCfg, "Fairbank is heigher than Greenland")
	}
	return nil
}

// DoNotValidate validates the given config
func DoNotValidate(cfg Config) error { return nil }
