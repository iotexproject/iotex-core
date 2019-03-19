// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package poll

import (
	"context"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-election/test/mock/mock_committee"
	"github.com/iotexproject/iotex-election/types"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action/protocol/vote/candidatesutil"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/state/factory"
)

func TestInitialize(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	cfg := config.Default
	ctx := context.Background()
	sf, err := factory.NewFactory(cfg, factory.InMemTrieOption())
	require.NoError(err)
	require.NoError(sf.Start(ctx))
	defer func() {
		require.NoError(sf.Stop(ctx))
	}()
	ws, err := sf.NewWorkingSet()
	require.NoError(err)
	committee := mock_committee.NewMockCommittee(ctrl)
	r := types.NewElectionResultForTest(time.Now())
	committee.EXPECT().ResultByHeight(uint64(123456)).Return(r, nil).Times(1)
	p, err := NewGovernanceChainCommitteeProtocol(
		nil,
		committee,
		uint64(123456),
		func(uint64) (time.Time, error) { return time.Now(), nil },
		func(uint64) uint64 { return 1 },
		func(uint64) uint64 { return 1 },
		cfg.Genesis.NumCandidateDelegates,
		cfg.Genesis.NumDelegates,
	)
	require.NoError(err)
	require.NoError(p.Initialize(ctx, ws))
	var sc state.CandidateList
	require.NoError(ws.State(candidatesutil.ConstructKey(1), &sc))
	candidates, err := state.CandidatesToMap(sc)
	require.NoError(err)
	require.Equal(2, len(candidates))
	for _, d := range r.Delegates() {
		operator := string(d.OperatorAddress())
		addr, err := address.FromString(operator)
		require.NoError(err)
		c, ok := candidates[hash.BytesToHash160(addr.Bytes())]
		require.True(ok)
		require.Equal(addr.String(), c.Address)
	}
}

func TestHandle(t *testing.T) {
}

func TestProtocol_Validate(t *testing.T) {
}
