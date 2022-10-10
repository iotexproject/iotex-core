package staking

import "github.com/iotexproject/iotex-core/blockchain/genesis"

type (

	// BuilderConfig returns the configuration of the builder
	BuilderConfig struct {
		Staking                  genesis.Staking
		PersistStakingPatchBlock uint64
		StakingPatchDir          string
	}
)
