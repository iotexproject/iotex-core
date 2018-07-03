// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos2

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core-internal/config"
	"github.com/iotexproject/iotex-core-internal/test/mock/mock_blockchain"
	"github.com/iotexproject/iotex-core-internal/test/mock/mock_delegate"
	"github.com/iotexproject/iotex-core-internal/test/util"
)

func TestBackdoorEvt(t *testing.T) {
	ctx := makeTestRollDPoSCtx(
		nil,
		config.RollDPoS{
			EventChanSize: 1,
		},
		func(blockchain *mock_blockchain.MockBlockchain) {},
		func(pool *mock_delegate.MockPool) {},
	)
	cfsm, err := newConsensusFSM(ctx)
	require.Nil(t, err)
	require.NotNil(t, cfsm)
	require.Equal(t, sEpochStart, cfsm.currentState())

	cfsm.Start(context.Background())
	defer cfsm.Stop(context.Background())

	for _, state := range consensusStates {
		cfsm.enqueue(cfsm.newBackdoorEvt(state), 0)
		util.WaitUntil(10*time.Millisecond, 100*time.Millisecond, func() (bool, error) {
			return state == cfsm.currentState(), nil
		})
	}

}
