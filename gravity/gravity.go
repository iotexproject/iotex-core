// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package gravity

import (
	"context"
	"math/big"
	"time"

	"github.com/iotexproject/detc"
	"github.com/iotexproject/iotex-election/committee"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
)

// Gravity is the abstraction of gravity chain service
type Gravity struct {
	committee committee.Committee
	detc      *detc.Detc
}

// New constructs a gravity chain service
func New(cfg config.Config) (*Gravity, error) {
	var err error
	var ec committee.Committee
	if cfg.Genesis.EnableGravityChainVoting {
		committeeConfig := cfg.Chain.Committee
		committeeConfig.GravityChainStartHeight = cfg.Genesis.GravityChainStartHeight
		committeeConfig.GravityChainHeightInterval = cfg.Genesis.GravityChainHeightInterval
		committeeConfig.RegisterContractAddress = cfg.Genesis.RegisterContractAddress
		committeeConfig.StakingContractAddress = cfg.Genesis.StakingContractAddress
		committeeConfig.VoteThreshold = cfg.Genesis.VoteThreshold
		committeeConfig.ScoreThreshold = cfg.Genesis.ScoreThreshold
		committeeConfig.StakingContractAddress = cfg.Genesis.StakingContractAddress
		committeeConfig.SelfStakingThreshold = cfg.Genesis.SelfStakingThreshold

		kvstore := db.NewOnDiskDB(cfg.Chain.GravityChainDB)
		if committeeConfig.GravityChainStartHeight != 0 {
			if ec, err = committee.NewCommitteeWithKVStoreWithNamespace(
				kvstore,
				committeeConfig,
			); err != nil {
				return nil, err
			}
		}
	}

	var de *detc.Detc
	if cfg.Genesis.DetcContractAddress != "" {
		de, err = detc.New(
			detc.WithContractAddress(cfg.Genesis.DetcContractAddress),
			detc.WithEthEndpoints(cfg.Chain.Committee.GravityChainAPIs...),
		)
		if err != nil {
			return nil, err
		}
	}

	return &Gravity{
		committee: ec,
		detc:      de,
	}, nil
}

// Start starts the gravity service
func (g *Gravity) Start(ctx context.Context) error {
	if g.committee != nil {
		return g.committee.Start(ctx)
	}
	return nil
}

// Stop stops the gravity services
func (g *Gravity) Stop(ctx context.Context) error {
	if g.committee != nil {
		return g.committee.Stop(ctx)
	}
	return nil
}

// Committee returns the committee
func (g *Gravity) Committee() committee.Committee { return g.committee }

// GetConfig gets the config defined on gravity chain
func (g *Gravity) GetConfig(ts time.Time, key string) ([]byte, error) {
	if g.detc == nil {
		return nil, nil
	}
	var ethBlkNum *big.Int
	if g.committee != nil {
		height, err := g.committee.HeightByTime(ts)
		if err != nil {
			return nil, err
		}
		ethBlkNum = big.NewInt(int64(height))
	}
	value, _, err := g.detc.GetConfig(key, ethBlkNum)
	return value, err
}
