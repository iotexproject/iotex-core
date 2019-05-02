package detc

import (
	"math/big"
	"math/rand"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

// Config contains the config information of Detc
type Config struct {
	contractAddr common.Address
	endpoints []string
}

// Option is the option function to set the config information
type Option func(cfg *Config) error

// WithContractAddress sets Detc contract address in hex string
func WithContractAddress(address string) Option {
	return func(cfg *Config) error {
		cfg.contractAddr = common.HexToAddress(address)
		return nil
	}
}

// WithEthEndpoints sets Ethereum json-rpc endpoints
func WithEthEndpoints(endpoints ...string) Option {
	return func(cfg *Config) error {
		cfg.endpoints = make([]string, len(endpoints))
		copy(cfg.endpoints, endpoints)
		return nil
	}
}

// Detc is the access wrapper to read the configs from Detc contract
type Detc struct {
	cfg   Config
}

// New constructs a Detc struct
func New(opts ...Option) (*Detc, error) {
	detc := Detc{
		cfg: Config{

		},
	}
	for _, opt := range opts {
		if err := opt(&detc.cfg); err != nil {
			return nil, err
		}
	}
	return &detc, nil
}

// GetConfig gets the config value for a given key. If ethBlkNum is nil, it will read the value based on the latest
// block number on Ethereum. Otherwise, it will read it from the given block number.
func (detc *Detc) GetConfig(key string, ethBlkNum *big.Int) ([]byte, *big.Int, error) {
	idx := rand.Intn(len(detc.cfg.endpoints))
	client, err := ethclient.Dial(detc.cfg.endpoints[idx])
	if err != nil {
		return nil, nil, err
	}
	defer client.Close()
	caller, err := NewDetcGenCaller(detc.cfg.contractAddr, client)
	if err != nil {
		return nil, nil, err
	}
	return caller.GetConfig(
		&bind.CallOpts{
			BlockNumber: ethBlkNum,
		},
		key,
	)
}
