// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"net"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/consensus/fsm"
	"github.com/iotexproject/iotex-core/delegate"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockchain"
)

func TestRuleDKGGenerateCondition(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	addr := common.NewTCPNode("127.0.0.1:40001")
	bc := mock_blockchain.NewMockBlockchain(ctrl)
	bc.EXPECT().TipHeight().Return(uint64(16), nil).Times(1)

	elected := false
	h := ruleDKGGenerate{
		RollDPoS: &RollDPoS{
			self: addr,
			cfg: config.RollDPoS{
				ProposerInterval: 0,
			},
			bc: bc,
			rollDPoSCB: rollDPoSCB{
				prCb: func(_ delegate.Pool, _ []byte, _ uint64, _ uint64) (net.Addr, error) {
					elected = true
					return addr, nil
				},
			},
			eventChan: make(chan *fsm.Event, 1),
		},
	}
	h.RollDPoS.prnd = &proposerRotation{RollDPoS: h.RollDPoS}

	assert.True(
		t,
		h.Condition(&fsm.Event{
			State: stateRoundStart,
		}),
	)
	assert.True(t, elected)
}
