package staking

import "github.com/iotexproject/iotex-core/v2/blockchain/genesis"

type (

	// BuilderConfig returns the configuration of the builder
	BuilderConfig struct {
		Staking                  genesis.Staking
		PersistStakingPatchBlock uint64
		FixAliasForNonStopHeight uint64
		StakingPatchDir          string
		Revise                   ReviseConfig
	}
)
