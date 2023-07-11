// Copyright (c) 2020 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"math"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/test/identityset"
)

func initTestProtocol(t *testing.T) (*Protocol, []*Candidate) {
	require := require.New(t)
	p, err := NewProtocol(nil, &BuilderConfig{
		Staking:                  genesis.Default.Staking,
		PersistStakingPatchBlock: math.MaxUint64,
	}, nil, nil, genesis.Default.GreenlandBlockHeight)
	require.NoError(err)

	var cans []*Candidate
	cans = append(cans, &Candidate{
		Owner:              identityset.Address(1),
		Operator:           identityset.Address(11),
		Reward:             identityset.Address(1),
		Name:               "test1",
		Votes:              big.NewInt(2),
		SelfStakeBucketIdx: 1,
		SelfStake:          big.NewInt(0),
	})
	cans = append(cans, &Candidate{
		Owner:              identityset.Address(28),
		Operator:           identityset.Address(28),
		Reward:             identityset.Address(29),
		Name:               "test2",
		Votes:              big.NewInt(2),
		SelfStakeBucketIdx: 2,
		SelfStake:          big.NewInt(10),
	})
	cans = append(cans, &Candidate{
		Owner:              identityset.Address(28),
		Operator:           identityset.Address(28),
		Reward:             identityset.Address(29),
		Name:               "test",
		Votes:              big.NewInt(2),
		SelfStakeBucketIdx: 2,
		SelfStake:          big.NewInt(10),
	})
	return p, cans
}
