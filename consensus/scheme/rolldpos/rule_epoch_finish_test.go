// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"net"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/iotexproject/iotex-core/consensus/fsm"
	"github.com/iotexproject/iotex-core/network/node"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockchain"
)

func TestRuleEpochFinishCondition(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bc := mock_blockchain.NewMockBlockchain(ctrl)
	bc.EXPECT().TipHeight().Return(uint64(16), nil).Times(1)

	var dkg hash.DKGHash
	h := ruleEpochFinish{
		RollDPoS: &RollDPoS{
			bc: bc,
			epochCtx: &epochCtx{
				height:       10,
				numSubEpochs: 2,
				dkg:          dkg,
				delegates: []net.Addr{
					node.NewTCPNode("127.0.0.1:10001"),
					node.NewTCPNode("127.0.0.1:10002"),
					node.NewTCPNode("127.0.0.1:10003"),
					node.NewTCPNode("127.0.0.1:10004"),
				},
			},
			rollDPoSCB: rollDPoSCB{
				epochStartCb: NeverStartNewEpoch,
			},
		},
	}

	assert.False(t, h.Condition(&fsm.Event{State: stateRoundStart}))
	assert.False(t, h.Condition(&fsm.Event{State: stateEpochStart}))
	bc.EXPECT().TipHeight().Return(uint64(17), nil).Times(1)
	assert.True(t, h.Condition(&fsm.Event{State: stateEpochStart}))
}
