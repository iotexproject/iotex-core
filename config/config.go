// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package config

import (
	"encoding/hex"
	"flag"
	"os"
	"time"

	"github.com/iotexproject/go-ethereum/crypto"
	"github.com/pkg/errors"
	uconfig "go.uber.org/config"

	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/consensus/consensusfsm"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/log"
)

// IMPORTANT: to define a config, add a field or a new config type to the existing config types. In addition, provide
// the default value in Default var.

func init() {
	flag.StringVar(&_overwritePath, "config-path", "", "Config path")
	flag.StringVar(&_secretPath, "secret-path", "", "Secret path")
	flag.StringVar(&_subChainPath, "sub-config-path", "", "Sub chain Config path")
}

var (
	// overwritePath is the path to the config file which overwrite default values
	_overwritePath string
	// secretPath is the path to the  config file store secret values
	_secretPath   string
	_subChainPath string
)

const (
	// DelegateType represents the delegate node type
	DelegateType = "delegate"
	// FullNodeType represents the full node type
	FullNodeType = "full_node"
	// LightweightType represents the lightweight type
	LightweightType = "lightweight"

	// RollDPoSScheme means randomized delegated proof of stake
	RollDPoSScheme = "ROLLDPOS"
	// StandaloneScheme means that the node creates a block periodically regardless of others (if there is any)
	StandaloneScheme = "STANDALONE"
	// NOOPScheme means that the node does not create only block
	NOOPScheme = "NOOP"
	// IndexTransfer is table identifier for transfer index in indexer
	IndexTransfer = "transfer"
	// IndexVote is table identifier for vote index in indexer
	IndexVote = "vote"
	// IndexExecution is table identifier for execution index in indexer
	IndexExecution = "execution"
	// IndexAction is table identifier for action index in indexer
	IndexAction = "action"
	// IndexReceipt is table identifier for receipt index in indexer
	IndexReceipt = "receipt"
)

var (
	// Default is the default config
	Default = Config{
		NodeType: FullNodeType,
		Network: Network{
			Host:           "0.0.0.0",
			Port:           4689,
			ExternalHost:   "",
			ExternalPort:   4689,
			BootstrapNodes: make([]string, 0),
			MasterKey:      "",
		},
		Chain: Chain{
			ChainDBPath:                  "/tmp/chain.db",
			TrieDBPath:                   "/tmp/trie.db",
			ID:                           1,
			Address:                      "",
			ProducerPubKey:               keypair.EncodePublicKey(&PrivateKey.PublicKey),
			ProducerPrivKey:              keypair.EncodePrivateKey(PrivateKey),
			GenesisActionsPath:           "",
			EmptyGenesis:                 false,
			NumCandidates:                101,
			EnableFallBackToFreshDB:      false,
			EnableSubChainStartInGenesis: false,
			EnableGasCharge:              false,
			EnableTrielessStateDB:        true,
			EnableIndex:                  false,
			EnableAsyncIndexWrite:        false,
		},
		ActPool: ActPool{
			MaxNumActsPerPool: 32000,
			MaxNumActsPerAcct: 2000,
			MaxNumActsToPick:  0,
			ActionExpiry:      10 * time.Minute,
		},
		Consensus: Consensus{
			Scheme: NOOPScheme,
			RollDPoS: RollDPoS{
				FSM: consensusfsm.Config{
					UnmatchedEventTTL:            3 * time.Second,
					UnmatchedEventInterval:       100 * time.Millisecond,
					AcceptBlockTTL:               4 * time.Second,
					AcceptProposalEndorsementTTL: 2 * time.Second,
					AcceptLockEndorsementTTL:     2 * time.Second,
					EventChanSize:                10000,
				},
				ToleratedOvertime: 2 * time.Second,
				DelegateInterval:  10 * time.Second,
				Delay:             5 * time.Second,
				NumSubEpochs:      1,
				NumDelegates:      21,
				TimeBasedRotation: false,
			},
			BlockCreationInterval: 10 * time.Second,
		},
		BlockSync: BlockSync{
			Interval:   10 * time.Second,
			BufferSize: 16,
		},
		Dispatcher: Dispatcher{
			EventChanSize: 10000,
		},
		Explorer: Explorer{
			Enabled:    false,
			UseIndexer: false,
			Port:       14004,
			TpsWindow:  10,
			GasStation: GasStation{
				SuggestBlockWindow: 20,
				DefaultGas:         1,
				Percentile:         60,
			},
			MaxTransferPayloadBytes: 1024,
		},
		API: API{
			Enabled:   false,
			UseRDS:    false,
			Port:      14004,
			TpsWindow: 10,
			GasStation: GasStation{
				SuggestBlockWindow: 20,
				DefaultGas:         1,
				Percentile:         60,
			},
			MaxTransferPayloadBytes: 1024,
		},
		Indexer: Indexer{
			Enabled:           false,
			NodeAddr:          "",
			WhetherLocalStore: true,
			BlockByIndexList:  []string{IndexTransfer, IndexVote, IndexExecution, IndexAction, IndexReceipt},
			IndexHistoryList:  []string{IndexTransfer, IndexVote, IndexExecution, IndexAction},
		},
		System: System{
			HeartbeatInterval:     10 * time.Second,
			HTTPProfilingPort:     0,
			HTTPMetricsPort:       8080,
			HTTPProbePort:         7788,
			StartSubChainInterval: 10 * time.Second,
		},
		DB: DB{
			UseBadgerDB: false,
			NumRetries:  3,
			SQLITE3: SQLITE3{
				SQLite3File: "./explorer.db",
			},
		},
	}

	// ErrInvalidCfg indicates the invalid config value
	ErrInvalidCfg = errors.New("invalid config value")

	// Validates is the collection config validation functions
	Validates = []Validate{
		ValidateKeyPair,
		ValidateConsensusScheme,
		ValidateRollDPoS,
		ValidateDispatcher,
		ValidateExplorer,
		ValidateAPI,
		ValidateActPool,
		ValidateChain,
	}

	// PrivateKey is a randomly generated producer's key for testing purpose
	PrivateKey, _ = crypto.GenerateKey()
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
	}

	// Chain is the config struct for blockchain package
	Chain struct {
		ChainDBPath                  string `yaml:"chainDBPath"`
		TrieDBPath                   string `yaml:"trieDBPath"`
		ID                           uint32 `yaml:"id"`
		Address                      string `yaml:"address"`
		ProducerPubKey               string `yaml:"producerPubKey"`
		ProducerPrivKey              string `yaml:"producerPrivKey"`
		GenesisActionsPath           string `yaml:"genesisActionsPath"`
		EmptyGenesis                 bool   `yaml:"emptyGenesis"`
		NumCandidates                uint   `yaml:"numCandidates"`
		EnableFallBackToFreshDB      bool   `yaml:"enableFallbackToFreshDb"`
		EnableSubChainStartInGenesis bool   `yaml:"enableSubChainStartInGenesis"`
		EnableTrielessStateDB        bool   `yaml:"enableTrielessStateDB"`

		// enable gas charge for block producer
		EnableGasCharge bool `yaml:"enableGasCharge"`
		// enable index the block actions and receipts
		EnableIndex bool `yaml:"enableIndex"`
		// enable writing the block actions' and receipts' index asynchronously
		EnableAsyncIndexWrite bool `yaml:"enableAsyncIndexWrite"`
	}

	// Consensus is the config struct for consensus package
	Consensus struct {
		// There are three schemes that are supported
		Scheme                string        `yaml:"scheme"`
		RollDPoS              RollDPoS      `yaml:"rollDPoS"`
		BlockCreationInterval time.Duration `yaml:"blockCreationInterval"`
	}

	// BlockSync is the config struct for the BlockSync
	BlockSync struct {
		Interval   time.Duration `yaml:"interval"` // update duration
		BufferSize uint64        `yaml:"bufferSize"`
	}

	// RollDPoS is the config struct for RollDPoS consensus package
	RollDPoS struct {
		FSM               consensusfsm.Config `yaml:"fsm"`
		ToleratedOvertime time.Duration       `yaml:"toleratedOvertime"`
		DelegateInterval  time.Duration       `yaml:"delegateInterval"`
		Delay             time.Duration       `yaml:"delay"`
		NumSubEpochs      uint                `yaml:"numSubEpochs"`
		NumDelegates      uint                `yaml:"numDelegates"`
		TimeBasedRotation bool                `yaml:"timeBasedRotation"`
	}

	// Dispatcher is the dispatcher config
	Dispatcher struct {
		EventChanSize uint `yaml:"eventChanSize"`
	}

	// Explorer is the explorer service config
	Explorer struct {
		Enabled    bool       `yaml:"enabled"`
		IsTest     bool       `yaml:"isTest"`
		UseIndexer bool       `yaml:"useIndexer"`
		Port       int        `yaml:"port"`
		TpsWindow  int        `yaml:"tpsWindow"`
		GasStation GasStation `yaml:"gasStation"`
		// MaxTransferPayloadBytes limits how many bytes a playload can contain at most
		MaxTransferPayloadBytes uint64 `yaml:"maxTransferPayloadBytes"`
	}

	// API is the api service config
	API struct {
		Enabled    bool       `yaml:"enabled"`
		IsTest     bool       `yaml:"isTest"`
		UseRDS     bool       `yaml:"useRDS"`
		Port       int        `yaml:"port"`
		TpsWindow  int        `yaml:"tpsWindow"`
		GasStation GasStation `yaml:"gasStation"`
		// MaxTransferPayloadBytes limits how many bytes a playload can contain at most
		MaxTransferPayloadBytes uint64 `yaml:"maxTransferPayloadBytes"`
	}

	// GasStation is the gas station config
	GasStation struct {
		SuggestBlockWindow int `yaml:"suggestBlockWindow"`
		DefaultGas         int `yaml:"defaultGas"`
		Percentile         int `yaml:"Percentile"`
	}

	// Indexer is the index service config
	Indexer struct {
		Enabled           bool   `yaml:"enabled"`
		NodeAddr          string `yaml:"nodeAddr"`
		WhetherLocalStore bool   `yaml:"whetherLocalStore"`
		// BlockByIndexList store list of BlockByIndex tables
		BlockByIndexList []string `yaml:"blockByIndexList"`
		// IndexHistoryList store list of IndexHistory tables
		IndexHistoryList []string `yaml:"indexHistoryList"`
	}

	// System is the system config
	System struct {
		HeartbeatInterval time.Duration `yaml:"heartbeatInterval"`
		// HTTPProfilingPort is the port number to access golang performance profiling data of a blockchain node. It is
		// 0 by default, meaning performance profiling has been disabled
		HTTPProfilingPort     int           `yaml:"httpProfilingPort"`
		HTTPMetricsPort       int           `yaml:"httpMetricsPort"`
		HTTPProbePort         int           `yaml:"httpProbePort"`
		StartSubChainInterval time.Duration `yaml:"startSubChainInterval"`
	}

	// ActPool is the actpool config
	ActPool struct {
		// MaxNumActsPerPool indicates maximum number of actions the whole actpool can hold
		MaxNumActsPerPool uint64 `yaml:"maxNumActsPerPool"`
		// MaxNumActsPerAcct indicates maximum number of actions an account queue can hold
		MaxNumActsPerAcct uint64 `yaml:"maxNumActsPerAcct"`
		// MaxNumActsToPick indicates maximum number of actions to pick to mint a block. Default is 0, which means no
		// limit on the number of actions to pick.
		MaxNumActsToPick uint64 `yaml:"maxNumActsToPick"`
		// ActionExpiry defines how long an action will be kept in action pool.
		ActionExpiry time.Duration `yaml:"actionExpiry"`
	}

	// DB is the config for database
	DB struct {
		DbPath string `yaml:"dbPath"`
		// Use BadgerDB, otherwise use BoltDB
		UseBadgerDB bool `yaml:"useBadgerDB"`
		// NumRetries is the number of retries
		NumRetries uint8 `yaml:"numRetries"`

		// RDS is the config for rds
		RDS RDS `yaml:"RDS"`

		// SQLite3 is the config for SQLITE3
		SQLITE3 SQLITE3 `yaml:"SQLITE3"`
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
		NodeType   string           `yaml:"nodeType"`
		Network    Network          `yaml:"network"`
		Chain      Chain            `yaml:"chain"`
		ActPool    ActPool          `yaml:"actPool"`
		Consensus  Consensus        `yaml:"consensus"`
		BlockSync  BlockSync        `yaml:"blockSync"`
		Dispatcher Dispatcher       `yaml:"dispatcher"`
		Explorer   Explorer         `yaml:"explorer"`
		API        API              `yaml:"api"`
		Indexer    Indexer          `yaml:"indexer"`
		System     System           `yaml:"system"`
		DB         DB               `yaml:"db"`
		Log        log.GlobalConfig `yaml:"log"`
	}

	// Validate is the interface of validating the config
	Validate func(Config) error
)

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

// IsDelegate returns true if the node type is Delegate
func (cfg Config) IsDelegate() bool {
	return cfg.NodeType == DelegateType
}

// IsFullnode returns true if the node type is Fullnode
func (cfg Config) IsFullnode() bool {
	return cfg.NodeType == FullNodeType
}

// IsLightweight returns true if the node type is Lightweight
func (cfg Config) IsLightweight() bool {
	return cfg.NodeType == LightweightType
}

// BlockchainAddress returns the address derived from the configured chain ID and public key
func (cfg Config) BlockchainAddress() (address.Address, error) {
	pk, err := keypair.DecodePublicKey(cfg.Chain.ProducerPubKey)
	if err != nil {
		return nil, errors.Wrapf(err, "error when decoding public key %s", cfg.Chain.ProducerPubKey)
	}
	pkHash := keypair.HashPubKey(pk)
	return address.FromBytes(pkHash[:])
}

// KeyPair returns the decoded public and private key pair
func (cfg Config) KeyPair() (keypair.PublicKey, keypair.PrivateKey, error) {
	pk, err := keypair.DecodePublicKey(cfg.Chain.ProducerPubKey)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "error when decoding public key %s", cfg.Chain.ProducerPubKey)
	}
	sk, err := keypair.DecodePrivateKey(cfg.Chain.ProducerPrivKey)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "error when decoding private key %s", cfg.Chain.ProducerPrivKey)
	}
	return pk, sk, nil
}

// ValidateKeyPair validates the block producer address
func ValidateKeyPair(cfg Config) error {
	pkBytes, err := hex.DecodeString(cfg.Chain.ProducerPubKey)
	if err != nil {
		return err
	}
	priKey, err := keypair.DecodePrivateKey(cfg.Chain.ProducerPrivKey)
	if err != nil {
		return err
	}
	// Validate producer pubkey and prikey by signing a dummy message and verify it
	validationMsg := "connecting the physical world block by block"
	msgHash := hash.Hash256b([]byte(validationMsg))
	sig, err := crypto.Sign(msgHash[:], priKey)
	if err != nil {
		return err
	}
	if !crypto.VerifySignature(pkBytes, msgHash[:], sig[:64]) {
		return errors.Wrap(ErrInvalidCfg, "block producer has unmatched pubkey and prikey")
	}
	return nil
}

// ValidateChain validates the chain configure
func ValidateChain(cfg Config) error {
	if cfg.Chain.NumCandidates <= 0 {
		return errors.Wrapf(ErrInvalidCfg, "candidate number should be greater than 0")
	}
	if cfg.Consensus.Scheme == RollDPoSScheme && cfg.Chain.NumCandidates < cfg.Consensus.RollDPoS.NumDelegates {
		return errors.Wrapf(ErrInvalidCfg, "candidate number should be greater than or equal to delegate number")
	}
	return nil
}

// ValidateConsensusScheme validates the if scheme and node type match
func ValidateConsensusScheme(cfg Config) error {
	switch cfg.NodeType {
	case DelegateType:
	case FullNodeType:
		if cfg.Consensus.Scheme != NOOPScheme {
			return errors.Wrap(ErrInvalidCfg, "consensus scheme of fullnode should be NOOP")
		}
	case LightweightType:
		if cfg.Consensus.Scheme != NOOPScheme {
			return errors.Wrap(ErrInvalidCfg, "consensus scheme of lightweight node should be NOOP")
		}
	default:
		return errors.Wrapf(ErrInvalidCfg, "unknown node type %s", cfg.NodeType)
	}
	return nil
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
	if rollDPoS.NumDelegates <= 0 {
		return errors.Wrap(ErrInvalidCfg, "roll-DPoS event delegate number should be greater than 0")
	}
	fsm := rollDPoS.FSM
	if fsm.EventChanSize <= 0 {
		return errors.Wrap(ErrInvalidCfg, "roll-DPoS event chan size should be greater than 0")
	}
	ttl := fsm.AcceptLockEndorsementTTL + fsm.AcceptBlockTTL + fsm.AcceptProposalEndorsementTTL
	if ttl >= rollDPoS.DelegateInterval {
		return errors.Wrap(ErrInvalidCfg, "roll-DPoS ttl sum is larger than proposer interval")
	}

	return nil
}

// ValidateExplorer validates the explorer configs
func ValidateExplorer(cfg Config) error {
	if cfg.Explorer.Enabled && cfg.Explorer.TpsWindow <= 0 {
		return errors.Wrap(ErrInvalidCfg, "tps window is not a positive integer when the explorer is enabled")
	}
	return nil
}

// ValidateAPI validates the api configs
func ValidateAPI(cfg Config) error {
	if cfg.API.Enabled && cfg.API.TpsWindow <= 0 {
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

// DoNotValidate validates the given config
func DoNotValidate(cfg Config) error { return nil }
