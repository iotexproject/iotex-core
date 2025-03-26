// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockindex

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_blockdao"
)

func TestSyncIndexers_CheckSync(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	master := mock_blockdao.NewMockBlockIndexer(ctrl)
	master.EXPECT().Start(gomock.Any()).Return(nil).Times(4)
	master.EXPECT().Height().Return(uint64(10), nil).AnyTimes()
	master.EXPECT().Stop(gomock.Any()).Return(nil).Times(4)

	ctx := context.Background()
	for _, c := range []struct {
		name   string
		height []uint64 // indexer's height
		active []bool
		err    error
	}{
		{"2 indexer in-sync", []uint64{10, 10}, []bool{true, true}, nil},
		{"2 indexer inactive", []uint64{15, 20}, []bool{false, false}, nil},
		{"1 indexer active", []uint64{20}, []bool{true}, ErrNotSychronized},
		{"2 indexer active", []uint64{10, 20}, []bool{true, true}, ErrNotSychronized},
	} {
		t.Run(c.name, func(t *testing.T) {
			var mockIndexer []blockdao.BlockIndexer
			for i := range c.height {
				iwa := mock_blockdao.NewMockBlockIndexerWithActive(ctrl)
				iwa.EXPECT().Start(gomock.Any()).Return(nil).Times(1)
				iwa.EXPECT().IsActive().Return(c.active[i]).Times(1)
				if c.active[i] {
					iwa.EXPECT().Height().Return(c.height[i], nil).Times(1)
				}
				iwa.EXPECT().Stop(gomock.Any()).Return(nil).Times(1)
				mockIndexer = append(mockIndexer, iwa)
			}
			syncx := NewSyncIndexers(master, mockIndexer...)
			r.ErrorIs(syncx.Start(ctx), c.err)
			r.NoError(syncx.Stop(ctx))
		})
	}
}

func TestSyncIndexers_PutBlock(t *testing.T) {
	// todo: modify this test
}
