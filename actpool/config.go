package actpool

import (
	"math/big"
	"time"

	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/pkg/unit"
)

var (
	// DefaultConfig is the default config for actpool
	DefaultConfig = Config{
		MaxNumActsPerPool:  32000,
		MaxGasLimitPerPool: 320000000,
		MaxNumActsPerAcct:  2000,
		WorkerBufferSize:   2000,
		ActionExpiry:       10 * time.Minute,
		MinGasPriceStr:     big.NewInt(unit.Qev).String(),
		BlackList:          []string{},
		MaxNumBlobsPerAcct: 16,
		Store: &StoreConfig{
			Datadir: "/var/data/actpool.cache",
		},
	}
)

// Config is the actpool config
type Config struct {
	// MaxNumActsPerPool indicates maximum number of actions the whole actpool can hold
	MaxNumActsPerPool uint64 `yaml:"maxNumActsPerPool"`
	// MaxGasLimitPerPool indicates maximum gas limit the whole actpool can hold
	MaxGasLimitPerPool uint64 `yaml:"maxGasLimitPerPool"`
	// MaxNumActsPerAcct indicates maximum number of actions an account queue can hold
	MaxNumActsPerAcct uint64 `yaml:"maxNumActsPerAcct"`
	// WorkerBufferSize indicates the buffer size for each worker's job queue
	WorkerBufferSize uint64 `yaml:"bufferPerAcct"`
	// ActionExpiry defines how long an action will be kept in action pool.
	ActionExpiry time.Duration `yaml:"actionExpiry"`
	// MinGasPriceStr defines the minimal gas price the delegate will accept for an action
	MinGasPriceStr string `yaml:"minGasPrice"`
	// BlackList lists the account address that are banned from initiating actions
	BlackList []string `yaml:"blackList"`
	// Store defines the config for persistent cache
	Store *StoreConfig `yaml:"store"`
	// MaxNumBlobsPerAcct defines the maximum number of blob txs an account can have
	MaxNumBlobsPerAcct uint64 `yaml:"maxNumBlobsPerAcct"`
}

// MinGasPrice returns the minimal gas price threshold
func (ap Config) MinGasPrice() *big.Int {
	mgp, ok := new(big.Int).SetString(ap.MinGasPriceStr, 10)
	if !ok {
		log.S().Panicf("Error when parsing minimal gas price string: %s", ap.MinGasPriceStr)
	}
	return mgp
}

// StoreConfig is the configuration for the blob store
type StoreConfig struct {
	Datadir string `yaml:"datadir"` // Data directory containing the currently executable blobs
}
