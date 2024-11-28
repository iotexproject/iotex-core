// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	_ "embed"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/iotexproject/iotex-address/address"
)

var (
	// StakingContractJSONABI is the abi json of staking contract
	//go:embed contract_staking_abi_v2.json
	StakingContractJSONABI string
	// StakingContractABI is the abi of staking contract
	StakingContractABI abi.ABI
)

type (

	// ContractStakingIndexer defines the interface of contract staking reader
	ContractStakingIndexer interface {
		Height() (uint64, error)
		// Buckets returns active buckets
		Buckets(height uint64) ([]*VoteBucket, error)
		// BucketsByIndices returns active buckets by indices
		BucketsByIndices([]uint64, uint64) ([]*VoteBucket, error)
		// BucketsByCandidate returns active buckets by candidate
		BucketsByCandidate(ownerAddr address.Address, height uint64) ([]*VoteBucket, error)
		// TotalBucketCount returns the total number of buckets including burned buckets
		TotalBucketCount(height uint64) (uint64, error)
		// ContractAddress returns the contract address
		ContractAddress() string
	}
	// ContractStakingIndexerWithBucketType defines the interface of contract staking reader with bucket type
	ContractStakingIndexerWithBucketType interface {
		ContractStakingIndexer
		// BucketTypes returns the active bucket types
		BucketTypes(height uint64) ([]*ContractStakingBucketType, error)
	}

	delayTolerantIndexer struct {
		ContractStakingIndexer
		duration    time.Duration
		startHeight uint64
	}

	delayTolerantIndexerWithBucketType struct {
		*delayTolerantIndexer
		indexer ContractStakingIndexerWithBucketType
	}
)

func init() {
	var err error
	StakingContractABI, err = abi.JSON(strings.NewReader(StakingContractJSONABI))
	if err != nil {
		panic(err)
	}
}

// NewDelayTolerantIndexer creates a delay tolerant indexer
func NewDelayTolerantIndexer(indexer ContractStakingIndexer, duration time.Duration) ContractStakingIndexer {
	d := &delayTolerantIndexer{ContractStakingIndexer: indexer, duration: duration}
	if indexWithStart, ok := indexer.(interface{ StartHeight() uint64 }); ok {
		d.startHeight = indexWithStart.StartHeight()
	}
	return d
}

// NewDelayTolerantIndexerWithBucketType creates a delay tolerant indexer with bucket type
func NewDelayTolerantIndexerWithBucketType(indexer ContractStakingIndexerWithBucketType, duration time.Duration) ContractStakingIndexerWithBucketType {
	return &delayTolerantIndexerWithBucketType{
		NewDelayTolerantIndexer(indexer, duration).(*delayTolerantIndexer),
		indexer,
	}
}

func (c *delayTolerantIndexer) wait(height uint64) (bool, error) {
	// first check if the height is already reached
	if c.startHeight >= height {
		return false, nil
	}
	h, err := c.Height()
	if err != nil {
		return false, err
	}
	if h >= height {
		return true, nil
	}
	// wait for the height to be reached
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	timer := time.NewTimer(c.duration)
	defer timer.Stop()
	for {
		select {
		case <-ticker.C:
			h, err := c.Height()
			if err != nil {
				return false, err
			}
			if h >= height {
				return true, nil
			}
		case <-timer.C:
			return false, nil
		}
	}
}

func (c *delayTolerantIndexer) Buckets(height uint64) ([]*VoteBucket, error) {
	_, err := c.wait(height)
	if err != nil {
		return nil, err
	}
	return c.ContractStakingIndexer.Buckets(height)
}

func (c *delayTolerantIndexer) BucketsByIndices(indices []uint64, height uint64) ([]*VoteBucket, error) {
	_, err := c.wait(height)
	if err != nil {
		return nil, err
	}
	return c.ContractStakingIndexer.BucketsByIndices(indices, height)
}

func (c *delayTolerantIndexer) BucketsByCandidate(ownerAddr address.Address, height uint64) ([]*VoteBucket, error) {
	_, err := c.wait(height)
	if err != nil {
		return nil, err
	}
	return c.ContractStakingIndexer.BucketsByCandidate(ownerAddr, height)
}

func (c *delayTolerantIndexer) TotalBucketCount(height uint64) (uint64, error) {
	_, err := c.wait(height)
	if err != nil {
		return 0, err
	}
	return c.ContractStakingIndexer.TotalBucketCount(height)
}

func (c *delayTolerantIndexerWithBucketType) BucketTypes(height uint64) ([]*ContractStakingBucketType, error) {
	_, err := c.wait(height)
	if err != nil {
		return nil, err
	}
	return c.indexer.BucketTypes(height)
}
