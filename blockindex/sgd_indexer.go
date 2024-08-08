// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockindex

import (
	"context"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
)

type (
	// SGDRegistry is the interface for Sharing of Gas-fee with DApps
	SGDRegistry interface {
		blockdao.BlockIndexer
		// CheckContract returns the contract's eligibility for SGD and percentage
		CheckContract(context.Context, string) (address.Address, uint64, bool, error)
	}

	sgdRegistry struct {
	}
)

// NewSGDRegistry creates a new SGDIndexer
func NewSGDRegistry() SGDRegistry {
	return &sgdRegistry{}
}

func (sgd *sgdRegistry) Start(ctx context.Context) error {
	return nil
}

func (sgd *sgdRegistry) Stop(ctx context.Context) error {
	return nil
}

func (sgd *sgdRegistry) Height() (uint64, error) {
	return 0, nil
}

func (sgd *sgdRegistry) PutBlock(ctx context.Context, blk *block.Block) error {
	return nil
}

func (sgd *sgdRegistry) DeleteTipBlock(context.Context, *block.Block) error {
	return nil
}

func (sgd *sgdRegistry) CheckContract(ctx context.Context, contract string) (address.Address, uint64, bool, error) {
	return nil, 0, false, nil
}
