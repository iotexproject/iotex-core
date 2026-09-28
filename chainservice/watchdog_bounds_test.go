// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package chainservice

import (
	"sync/atomic"
	"testing"

	"github.com/iotexproject/iotex-proto/golang/iotexrpc"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/test/mock/mock_blockdao"
)

// lastReceivedBlockHeight drives the restart watchdog, and the header is
// unvalidated here. Only heights this node would actually buffer may be
// recorded; otherwise a peer names any height it likes and decides when the
// process restarts.
func TestFilterOnlyRecordsAcceptableHeights(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)

	dao := mock_blockdao.NewMockBlockDAO(ctrl)
	dao.EXPECT().Height().Return(uint64(100), nil).AnyTimes()
	cs := &ChainService{blockdao: dao}

	blockAt := func(h uint64) *iotextypes.Block {
		return &iotextypes.Block{Header: &iotextypes.BlockHeader{
			Core: &iotextypes.BlockHeaderCore{Height: h},
		}}
	}
	const cap = 200

	// far-future: rejected, and must leave the watchdog input untouched
	r.False(cs.Filter(iotexrpc.MessageType_BLOCK, blockAt(1<<62), cap))
	r.Zero(atomic.LoadUint64(&cs.lastReceivedBlockHeight))

	// exactly at the window edge: rejected too
	r.False(cs.Filter(iotexrpc.MessageType_BLOCK, blockAt(100+cap), cap))
	r.Zero(atomic.LoadUint64(&cs.lastReceivedBlockHeight))

	// inside the window: accepted and recorded
	r.True(cs.Filter(iotexrpc.MessageType_BLOCK, blockAt(150), cap))
	r.EqualValues(150, atomic.LoadUint64(&cs.lastReceivedBlockHeight))

	// a lower height must not move it backwards
	r.True(cs.Filter(iotexrpc.MessageType_BLOCK, blockAt(120), cap))
	r.EqualValues(150, atomic.LoadUint64(&cs.lastReceivedBlockHeight))
}
