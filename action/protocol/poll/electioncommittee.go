// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package poll

import (
	"context"
	"time"

	"github.com/iotexproject/iotex-election/committee"
	"github.com/iotexproject/iotex-election/types"
)

type (
	// ElectionCommittee defines an interface of election committee for poll protocol
	ElectionCommittee interface {
		ResultByHeight(uint64) (*types.ElectionResult, error)
		Start(context.Context) error
		Stop(context.Context) error
	}

	// GetEpochStartBlockTime defines a function to retrieve the time of the first block in an epoch
	GetEpochStartBlockTime func(uint64) (time.Time, error)

	electionCommittee struct {
		initGravityChainHeight uint64
		getEpochStartBlockTime GetEpochStartBlockTime
		gravityCommittee       committee.Committee
	}
)

// NewElectionCommittee returns a wrapped committee of gravity committee, in which it will translate
//  native chain height to gravity chain height
func NewElectionCommittee(
	gravityCommittee committee.Committee,
	getEpochStartBlockTime GetEpochStartBlockTime,
	initGravityChainHeight uint64,
) ElectionCommittee {
	return &electionCommittee{
		getEpochStartBlockTime: getEpochStartBlockTime,
		gravityCommittee:       gravityCommittee,
		initGravityChainHeight: initGravityChainHeight,
	}
}

func (ec *electionCommittee) Start(ctx context.Context) error {
	return ec.Start(ctx)
}

func (ec *electionCommittee) Stop(ctx context.Context) error {
	return ec.Stop(ctx)
}

func (ec *electionCommittee) ResultByHeight(height uint64) (*types.ElectionResult, error) {
	gravityHeight := ec.initGravityChainHeight
	if height > 0 {
		ts, err := ec.getEpochStartBlockTime(height)
		if err != nil {
			return nil, err
		}
		gravityHeight, err = ec.gravityCommittee.HeightByTime(ts)
		if err != nil {
			return nil, err
		}
	}
	return ec.gravityCommittee.ResultByHeight(gravityHeight)
}
