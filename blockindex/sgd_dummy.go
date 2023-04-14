// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockindex

import (
	"context"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockindex/indexpb"
)

type dummySgdRegistry struct{}

// NewDummySGDRegistry creates a new dummy SGDIndexer
func NewDummySGDRegistry() SGDIndexer {
	return &dummySgdRegistry{}
}

// Start starts the dummySgdRegistry
func (sgd *dummySgdRegistry) Start(ctx context.Context) error {
	return nil
}

// Stop stops the dummySgdRegistry
func (sgd *dummySgdRegistry) Stop(ctx context.Context) error {
	return nil
}

// Height returns the height of the dummySgdRegistry
func (sgd *dummySgdRegistry) Height() (uint64, error) {
	return 0, nil
}

// PutBlock puts a block into the dummySgdRegistry
func (sgd *dummySgdRegistry) PutBlock(context.Context, *block.Block) error {
	return nil
}

// DeleteTipBlock deletes the tip block from the dummySgdRegistry
func (sgd *dummySgdRegistry) DeleteTipBlock(context.Context, *block.Block) error {
	return nil
}

// CheckContract checks if the contract is a SGD contract
func (sgd *dummySgdRegistry) CheckContract(contract string) (address.Address, uint64, bool) {
	return nil, 0, false
}

// GetSGDIndex gets the SGDIndex from the dummySgdRegistry
func (sgd *dummySgdRegistry) GetSGDIndex(contract string) (*indexpb.SGDIndex, error) {
	return nil, nil
}
