// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rdpos

import (
	"net"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	bc "github.com/iotexproject/iotex-core/blockchain"
	cm "github.com/iotexproject/iotex-core/common"
)

func TestProposerRotation(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// arrange 2 consensus nodes
	delegates := []net.Addr{
		cm.NewTCPNode("192.168.0.1:10000"),
		cm.NewTCPNode("192.168.0.1:10001"),
	}
	m := func(mcks mocks) {
		mcks.dp.EXPECT().AllDelegates().Return(delegates, nil).AnyTimes()
		mcks.dNet.EXPECT().Broadcast(gomock.Any()).AnyTimes()
		mcks.bc.EXPECT().TipHeight().AnyTimes()
		genesis := bc.NewGenesisBlock(bc.Gen)
		mcks.bc.EXPECT().MintNewBlock(gomock.Any(), gomock.Any(), gomock.Any()).Return(genesis).AnyTimes()
	}
	cs := createTestRDPoS(ctrl, delegates[0], delegates, m, true)
	cs.Start()
	defer cs.Stop()

	time.Sleep(2 * time.Second)

	assert.NotNil(t, cs.roundCtx)
}
