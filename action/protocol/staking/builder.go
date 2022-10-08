package staking

import "github.com/iotexproject/iotex-core/blockchain/genesis"

type (

	// BuilderConfig returns the configuration of the builder
	BuilderConfig struct {
		Staking              genesis.Staking
		PersistCandsMapBlock uint64
	}
)
