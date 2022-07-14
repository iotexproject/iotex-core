package actpool

import (
	"math/big"
	"time"

	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/unit"
)

var (
	// DefaultConfig is the default config for actpool
	DefaultConfig = Config{
		MaxNumActsPerPool:  32000,
		MaxGasLimitPerPool: 320000000,
		MaxNumActsPerAcct:  10000,
		ActionExpiry:       10 * time.Minute,
		MinGasPriceStr:     big.NewInt(unit.Qev).String(),
		BlackList:          []string{},
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
	// ActionExpiry defines how long an action will be kept in action pool.
	ActionExpiry time.Duration `yaml:"actionExpiry"`
	// MinGasPriceStr defines the minimal gas price the delegate will accept for an action
	MinGasPriceStr string `yaml:"minGasPrice"`
	// BlackList lists the account address that are banned from initiating actions
	BlackList []string `yaml:"blackList"`
}

// MinGasPrice returns the minimal gas price threshold
func (ap Config) MinGasPrice() *big.Int {
	mgp, ok := new(big.Int).SetString(ap.MinGasPriceStr, 10)
	if !ok {
		log.S().Panicf("Error when parsing minimal gas price string: %s", ap.MinGasPriceStr)
	}
	return mgp
}
